# Terraform

Provisions the GCP/Firebase infrastructure for a Cloud Run backend: Artifact
Registry, IAM, and CI (Workload Identity Federation) as the core deploy path,
plus Firestore, Storage, security rules, and secrets to support it.

This is the infrastructure reference. For the end-to-end deploy sequence
(including the non-Terraform steps), follow [`../../docs/deploy.md`](../../docs/deploy.md).

## What it manages

| File | Resources |
|------|-----------|
| `apis.tf` | Enables required APIs; activates Firebase on the project |
| `backend.tf` | Remote state in GCS (partial config) |
| `run.tf` | Artifact Registry repo + Cloud Run v2 service + public invoker |
| `iam.tf` | Backend runtime service account + IAM (Firestore, bucket, secret) |
| `wif.tf` | CI service account + Workload Identity Federation for GitHub Actions |
| `firestore.tf` | Firestore (Native) database + rules from `../firebase/firestore.rules` |
| `storage.tf` | Assets bucket (+ CORS) + Firebase linkage + rules from `../firebase/storage.rules` |
| `secrets.tf` | Generated JWT secret in Secret Manager |
| `hosting.tf` | Firebase Hosting site |

The container image is **not** built here — Terraform creates the Cloud Run
service running a public placeholder image; GitHub Actions
(`../../.github/workflows/backend.yml`) builds the real one, pushes it, and
deploys it straight to the service.

## Inputs

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `project_id` | yes | — | GCP/Firebase project id (billing enabled) |
| `region` | no | `northamerica-northeast2` | Region for Run/AR/Firestore/bucket |
| `service_name` | no | `phil-canvas-backend` | Cloud Run name (must match `firebase.json` rewrites) |
| `github_repository` | yes | — | `owner/repo` allowed to push images via WIF |
| `admin_allowlist` | yes | — | Comma-separated admin emails/UIDs |
| `bucket_name` | yes | — | Globally-unique bucket for scenes/files |
| `cors_origins` | yes | — | Origins allowed to fetch published scene/files |
| `hosting_site_id` | no | `project_id` | Firebase Hosting site id |

## Outputs

| Output | Use |
|--------|-----|
| `backend_url` | Cloud Run URL (Hosting rewrites target); also `VITE_BACKEND_ORIGIN` GitHub variable |
| `backend_service_account` | Backend runtime SA email |
| `backend_image` | Full image ref Cloud Run runs |
| `wif_provider` | → `GCP_WIF_PROVIDER` GitHub secret |
| `ci_service_account` | → `GCP_DEPLOY_SA` GitHub secret |
| `storage_bucket` | Bucket name (already wired into the backend) |
| `published_base_url` | → `VITE_PUBLISHED_BASE_URL` GitHub variable |
| `hosting_site` | Hosting site id |

## Notes

- **State** lives in GCS (`backend.tf`, partial config) with locking; the state
  bucket must exist before `init`. State contains the generated JWT secret — keep
  the bucket access-controlled.
- **Credentials**: the runtime SA's ADC (see `iam.tf`) keeps
  `config.WithoutCredentials()` false in production, so persistence stays on.
- **WIF security**: the pool provider's attribute condition restricts tokens to
  `github_repository`'s owner, and only that repo can impersonate the CI
  service account (`wif.tf`).
- **CI permissions** (`wif.tf`): least-privilege grants scoped to pushing
  images, deploying Hosting, deploying this one Cloud Run service, and acting
  as its runtime SA. No Terraform state access — backend deploys go through
  CI, not `terraform apply` (see `run.tf`'s `lifecycle.ignore_changes` note).
- **WebSockets**: Cloud Run is HTTP/1 (no h2c), `timeout=3600s`,
  `min_instances=1`. Don't enable HTTP/2 end-to-end.

## Outside Terraform

1. Enable the Google sign-in provider (OAuth consent — Firebase console).
2. Set the two GitHub secrets + two variables above.
3. Push to `main` — `frontend.yml` builds and deploys Hosting (asset upload +
   rewrites); there's no manual `firebase deploy` step.
4. Bind `firebase.json`'s hosting `target` to the `hosting_site` output (`firebase target:apply hosting <target> <site-id>`).
