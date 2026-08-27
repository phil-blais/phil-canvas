output "backend_url" {
  description = "Cloud Run service URL. HTTP APIs go via Hosting rewrites; the WebSocket relay connects here directly (set as VITE_BACKEND_ORIGIN). Hosting does not proxy WebSocket upgrades."
  value       = google_cloud_run_v2_service.backend.uri
}

output "backend_service_account" {
  value = google_service_account.backend.email
}

output "backend_image" {
  description = "Image the Cloud Run service is currently running (reflects CI deploys, not just what Terraform created it with)."
  value       = google_cloud_run_v2_service.backend.template[0].containers[0].image
}

output "wif_provider" {
  description = "Set as the GCP_WIF_PROVIDER GitHub Actions secret."
  value       = google_iam_workload_identity_pool_provider.github.name
}

output "ci_service_account" {
  description = "Set as the GCP_DEPLOY_SA GitHub Actions secret."
  value       = google_service_account.deployer.email
}

output "storage_bucket" {
  description = "Bucket name — set as STORAGE_BUCKET (already wired into Cloud Run)."
  value       = google_storage_bucket.assets.name
}

output "published_base_url" {
  description = "Set as the VITE_PUBLISHED_BASE_URL GitHub Actions variable."
  value       = "https://firebasestorage.googleapis.com/v0/b/${google_storage_bucket.assets.name}/o"
}

output "hosting_site" {
  description = "Firebase Hosting site id."
  value       = google_firebase_hosting_site.default.site_id
}
