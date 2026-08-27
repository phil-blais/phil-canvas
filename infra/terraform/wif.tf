# Workload Identity Federation for GitHub Actions — keyless auth scoped to one
# CI service account; the pool provider trusts OIDC tokens from this repo only.

resource "google_service_account" "deployer" {
  account_id   = "phil-canvas-ci"
  display_name = "Phil Canvas CI (GitHub Actions)"
  depends_on   = [google_project_service.apis]
}

# Repo-scoped push permission (least privilege — not project-wide).
resource "google_artifact_registry_repository_iam_member" "deployer_writer" {
  location   = google_artifact_registry_repository.backend.location
  repository = google_artifact_registry_repository.backend.name
  role       = "roles/artifactregistry.writer"
  member     = "serviceAccount:${google_service_account.deployer.email}"
}

# Deploy Firebase Hosting from CI.
resource "google_project_iam_member" "deployer_hosting" {
  project = var.project_id
  role    = "roles/firebasehosting.admin"
  member  = "serviceAccount:${google_service_account.deployer.email}"
}

# Let a Hosting deploy resolve the Cloud Run rewrite target.
resource "google_project_iam_member" "deployer_run_viewer" {
  project = var.project_id
  role    = "roles/run.viewer"
  member  = "serviceAccount:${google_service_account.deployer.email}"
}

# Deploy new Cloud Run revisions (scoped to this one service, not project-wide).
resource "google_cloud_run_v2_service_iam_member" "deployer_run_developer" {
  name     = google_cloud_run_v2_service.backend.name
  location = google_cloud_run_v2_service.backend.location
  role     = "roles/run.developer"
  member   = "serviceAccount:${google_service_account.deployer.email}"
}

# Deploying a revision that runs as the backend SA requires actAs on it, even
# when redeploying with an unchanged service account.
resource "google_service_account_iam_member" "deployer_actas_backend" {
  service_account_id = google_service_account.backend.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.deployer.email}"
}

resource "google_iam_workload_identity_pool" "github" {
  workload_identity_pool_id = "github-actions"
  display_name              = "GitHub Actions"
  depends_on                = [google_project_service.apis]
}

resource "google_iam_workload_identity_pool_provider" "github" {
  workload_identity_pool_id          = google_iam_workload_identity_pool.github.workload_identity_pool_id
  workload_identity_pool_provider_id = "github"
  display_name                       = "GitHub"

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }

  attribute_mapping = {
    "google.subject"       = "assertion.sub"
    "attribute.repository" = "assertion.repository"
  }

  # Required: only tokens from this exact repo may use the provider.
  attribute_condition = "assertion.repository == \"${var.github_repository}\""
}

# Allow the specific GitHub repo to impersonate the CI service account.
resource "google_service_account_iam_member" "deployer_wif" {
  service_account_id = google_service_account.deployer.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github.name}/attribute.repository/${var.github_repository}"
}
