resource "google_firebase_hosting_site" "default" {
  provider   = google-beta
  project    = var.project_id
  site_id    = coalesce(var.hosting_site_id, var.project_id)
  depends_on = [google_firebase_project.default]
}
