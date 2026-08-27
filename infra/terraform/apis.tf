# Enable every API the stack needs. disable_on_destroy=false so `terraform
# destroy` doesn't churn shared APIs.
resource "google_project_service" "apis" {
  for_each = toset([
    "run.googleapis.com",
    "artifactregistry.googleapis.com",
    "cloudbuild.googleapis.com",
    "firestore.googleapis.com",
    "firebase.googleapis.com",
    "firebasehosting.googleapis.com",
    "firebaserules.googleapis.com",
    "firebasestorage.googleapis.com",
    "storage.googleapis.com",
    "secretmanager.googleapis.com",
    "identitytoolkit.googleapis.com",
    "iam.googleapis.com",
    # Workload Identity Federation (token exchange)
    "iamcredentials.googleapis.com",
    "sts.googleapis.com",
    "serviceusage.googleapis.com",
  ])

  service            = each.value
  disable_on_destroy = false
}

# Activate Firebase features on the project (Hosting, Storage linkage, etc.).
resource "google_firebase_project" "default" {
  provider   = google-beta
  project    = var.project_id
  depends_on = [google_project_service.apis]
}
