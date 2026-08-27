# Bucket for saved scenes, image files, and the published scene. Public read
# of public/** is granted by the Firebase Storage rules below, not by bucket
# IAM.
resource "google_storage_bucket" "assets" {
  name                        = var.bucket_name
  location                    = var.region
  uniform_bucket_level_access = true
  force_destroy               = false

  cors {
    origin          = var.cors_origins
    method          = ["GET"]
    response_header = ["Content-Type"]
    max_age_seconds = 3600
  }

  # Block accidental destroys; deliberate teardown removes this first.
  lifecycle {
    prevent_destroy = true
  }

  depends_on = [google_project_service.apis]
}

# Register the bucket with Firebase so Storage security rules apply to it.
resource "google_firebase_storage_bucket" "default" {
  provider   = google-beta
  project    = var.project_id
  bucket_id  = google_storage_bucket.assets.name
  depends_on = [google_firebase_project.default]
}

# Deploy the repo's Storage rules (public read on public/**, deny client writes).
resource "google_firebaserules_ruleset" "storage" {
  project = var.project_id
  source {
    files {
      name    = "storage.rules"
      content = file("${path.module}/../firebase/storage.rules")
    }
  }
  depends_on = [google_firebase_storage_bucket.default]
}

resource "google_firebaserules_release" "storage" {
  name         = "firebase.storage/${google_storage_bucket.assets.name}"
  ruleset_name = google_firebaserules_ruleset.storage.name
  project      = var.project_id
}
