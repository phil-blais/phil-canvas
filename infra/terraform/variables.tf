variable "project_id" {
  type        = string
  description = "Existing GCP/Firebase project id (billing enabled)."
}

variable "region" {
  type        = string
  default     = "us-central1"
  description = "Region for Cloud Run, Artifact Registry, Firestore, and the bucket. Must match the region in firebase.json rewrites."
}

variable "service_name" {
  type        = string
  default     = "phil-canvas-backend"
  description = "Cloud Run service name. Must match the serviceId in firebase.json rewrites."
}

variable "github_repository" {
  type        = string
  description = "GitHub repo allowed to push images via Workload Identity Federation, as owner/repo."
}

variable "admin_allowlist" {
  type        = string
  description = "Comma-separated admin emails/UIDs that may receive an admin JWT."
}

variable "bucket_name" {
  type        = string
  description = "Globally-unique GCS bucket for scenes and image files (also the Firebase Storage bucket)."
}

variable "cors_origins" {
  type        = list(string)
  description = "Origins allowed to fetch the published scene/files from Storage (Hosting domain(s) + localhost for dev)."
}

variable "hosting_site_id" {
  type        = string
  default     = null
  description = "Firebase Hosting site id. Defaults to project_id (the default site) if unset."
}
