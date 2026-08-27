# Runtime service account for Cloud Run — the backend authenticates via this
# account's ADC in production (no GOOGLE_APPLICATION_CREDENTIALS override),
# which keeps persistence enabled.
resource "google_service_account" "backend" {
  account_id   = "phil-canvas-backend"
  display_name = "Phil Canvas backend (Cloud Run)"
  depends_on   = [google_project_service.apis]
}

# Firestore read/write.
resource "google_project_iam_member" "backend_datastore" {
  project = var.project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${google_service_account.backend.email}"
}

# Read/write objects in the assets bucket (scoped to the bucket, not the project).
resource "google_storage_bucket_iam_member" "backend_objects" {
  bucket = google_storage_bucket.assets.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.backend.email}"
}

# Read the JWT secret.
resource "google_secret_manager_secret_iam_member" "backend_jwt" {
  secret_id = google_secret_manager_secret.jwt.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.backend.email}"
}
