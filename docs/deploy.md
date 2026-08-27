# Deployment Runbook

Infrastructure is managed with **Terraform** in [`infra/terraform/`](../infra/terraform):
APIs, Firestore (+ rules), the Storage bucket (+ CORS + rules), the JWT secret,
the service account and IAM, Artifact Registry, the Hosting site, and the Cloud Run service.

Three things Terraform doesn't own:

1. **The container image** — GitHub Actions builds and deploys it (step 2).
2. **Frontend build + Hosting upload** — GitHub Actions builds and deploys it
   too (step 2).
3. **Google sign-in provider** — a one-time console step (step 3).

## Prerequisites

- `terraform`, `gcloud`, `firebase`, and `node` installed; `gcloud auth login`
  and `gcloud auth application-default login` done.
  Alternatively, use [Cloud Shell](https://console.cloud.google.com) — `terraform`,
  `gcloud`, and `node` come preinstalled and already authenticated as you.
- A GCP/Firebase project with billing enabled.

```sh
export PROJECT_ID=<your-project-id>
gcloud config set project "$PROJECT_ID"
```

## 1. Provision infrastructure (Terraform)

State is stored remotely in GCS. Create the state bucket once (Terraform can't
create the bucket holding its own state), then init with it:

```sh
export REGION=northamerica-northeast2
export STATE_BUCKET=$PROJECT_ID-tfstate
gcloud storage buckets create "gs://$STATE_BUCKET" \
  --project="$PROJECT_ID" --location="$REGION" --uniform-bucket-level-access
gcloud storage buckets update "gs://$STATE_BUCKET" --versioning

cd infra/terraform
cp terraform.tfvars.example terraform.tfvars   # then edit values
cp backend.hcl.example backend.hcl             # set bucket = "$STATE_BUCKET"
terraform init -backend-config=backend.hcl

terraform apply
```

This creates Cloud Run running a public placeholder image, since Artifact Registry
is still empty. Step 2 wires up CI, which pushes the real image over it on the
next push to `main`.

The full `terraform apply` creates everything and prints outputs used below:

- `backend_url` — the Cloud Run URL (Hosting rewrites route here).
- `storage_bucket` — wired into the backend as `STORAGE_BUCKET`.
- `published_base_url` — for the frontend's `VITE_PUBLISHED_BASE_URL`.

Notes:
- Persistence/ADC credentials, WebSocket-friendly Cloud Run settings, and
  state/secret handling are documented in
  [`infra/terraform/README.md`](../infra/terraform/README.md#notes).
- To redeploy by hand instead of via CI (e.g. to roll back):
  `gcloud run deploy phil-canvas-backend --region <region> --image <region>-docker.pkg.dev/<project>/phil-canvas/backend:<sha>`.
  Reference a real SHA, not `latest`.

## 2. Wire up CI (GitHub Actions)

Three path-scoped workflows run on push/PR to their own paths:
- [`backend.yml`](../.github/workflows/backend.yml) — `go test -race` + `vet` +
  `gofmt`. On pushes to `main` (after tests pass) it also builds
  `backend/Dockerfile`, pushes `…/phil-canvas/backend:{sha,latest}` to Artifact
  Registry, then deploys that image straight to Cloud Run via
  `google-github-actions/deploy-cloudrun` (job `deploy`).
- [`frontend.yml`](../.github/workflows/frontend.yml) — `lint` + `test` +
  `build`. On pushes to `main` it also builds with production config
  (`VITE_`-prefixed values all ship to the browser) and runs `firebase deploy --only hosting` (job `deploy`).
- [`infra.yml`](../.github/workflows/infra.yml) — `terraform fmt -check` +
  `validate` on every push/PR (no apply; deploys are never done from CI).

Both deploy jobs swap in new output directly — image for backend, static
assets for frontend — without touching infra config. See the
`lifecycle.ignore_changes` note on `google_cloud_run_v2_service.backend` in
`run.tf` for how `backend.yml`'s image swap stays safe across a later
unrelated `terraform apply`.

Terraform provisions the keyless auth CI uses (Workload Identity Federation +
a scoped `phil-canvas-ci` service account) — see
[`infra/terraform/README.md`](../infra/terraform/README.md#notes) for what's
granted and why. Once step 1's `terraform apply` has run, copy the values
into GitHub (Settings → Secrets and variables → Actions):

- **Secrets** (from Terraform outputs):
  ```sh
  terraform -chdir=infra/terraform output -raw wif_provider        # -> GCP_WIF_PROVIDER
  terraform -chdir=infra/terraform output -raw ci_service_account  # -> GCP_DEPLOY_SA
  ```
- **Variables**:
  - `GCP_PROJECT_ID`, `GCP_REGION`.
  - Firebase web config for the production frontend build:
    `VITE_FIREBASE_API_KEY`, `VITE_FIREBASE_APP_ID`, and `VITE_PUBLISHED_BASE_URL`
    (the `published_base_url` output). `VITE_FIREBASE_PROJECT_ID` and
    `VITE_FIREBASE_AUTH_DOMAIN` are derived from `GCP_PROJECT_ID`.
  - `VITE_BACKEND_ORIGIN` — the `backend_url` output; required for the
    WebSocket relay (Hosting can't proxy WebSocket upgrades, so the frontend
    connects to the backend directly for that).

Set `github_repository` (owner/repo) in `terraform.tfvars` so only your repo
can mint tokens. Both deploy jobs assume infra already exists (so the run
rewrites resolve and the CI IAM is in place) — infra must be provisioned
(step 1) before the first push to `main`.

## 3. Enable Google sign-in (one-time, console)

Terraform enables Identity Platform, but the Google provider needs the OAuth
consent screen configured. In the Firebase console → **Authentication → Sign-in
method**, enable **Google**. (Terraform can then optionally manage the IdP via
`google_identity_platform_default_supported_idp_config` if you have an OAuth
client id/secret — omitted here to avoid a brittle console dependency.)

## 4. Frontend Hosting routing

Terraform creates the Hosting site (`hosting_site` output); ensure `firebase.json` rewrites, regions, and targets match the project's configuration.

## 5. Smoke test

1. Rewrites reach Cloud Run: `curl -i https://<domain>/rooms` → `200 []`.
2. `https://<domain>/` unauthenticated → published scene (or "Canvas"
   placeholder if nothing is published yet).
3. Sign in as an allowlisted admin → **Create room** → a code appears.
4. Second browser: open the room, enter the code → live sync (draw, cursors,
   laser). **Save** creates a Firestore version; **Publish** updates the root.
5. **Close room** as admin → the guest is bounced to the site (close code 4001).

## Teardown

Some resources are guarded against accidental deletion (`prevent_destroy`
lifecycle blocks, `force_destroy=false` on buckets). Once you actually intend
to tear down, clear those guards first, then:

```sh
cd infra/terraform && terraform destroy
```

Firestore databases and enabled APIs are left in place.

## Troubleshooting

- **WebSocket won't connect**: check `VITE_BACKEND_ORIGIN` is set (step 2) and
  the frontend was rebuilt since. Otherwise confirm Cloud Run is not HTTP/2
  end-to-end and `min_instances=1` is set (both are in `run.tf`).
- **Admin sign-in 403**: the account isn't in `admin_allowlist` (re-apply after
  editing the var).
- **404 on a route that exists in the code**: Cloud Run is still running a
  stale revision. Check `gcloud run services describe phil-canvas-backend
  --format='value(spec.template.spec.containers[0].image)'` against the
  latest commit's short SHA. `backend.yml`'s `deploy` job should keep this current;
  if it's stale, check the job's logs and that `run.developer` +
  `iam.serviceAccountUser` (`wif.tf`) are applied for the CI service account.
- **Save/Publish 500**: check the `published_base_url` bucket and that the
  service account bindings in `iam.tf` applied.
- **`build_push` fails**: ensure the Cloud Build API is enabled (it is, via
  `apis.tf`) and `gcloud` is authenticated; re-run `terraform apply`.
- **First `terraform apply` fails on an IAM resource** (`google_project_iam_member`
  in `iam.tf`/`wif.tf`) with "Cloud Resource Manager API has not been used in
  project ... or it is disabled": Google enables this API by default on new
  projects, which is why it's not in `apis.tf` — if it's ever off, Terraform
  can't self-bootstrap it. Enable it manually:
  `gcloud services enable cloudresourcemanager.googleapis.com`.
