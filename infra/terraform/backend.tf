# Remote state in GCS (with built-in state locking and, when the bucket has
# versioning enabled, state history).
#
# Partial configuration: the bucket is supplied at init time (see
# backend.hcl.example) so this repo stays project-agnostic.
# The state bucket must already exist — Terraform can't create the bucket
# that stores its own state (bootstrap problem):
#
#   gcloud storage buckets create gs://<STATE_BUCKET> \
#     --project=<PROJECT_ID> --location=<REGION> --uniform-bucket-level-access
#   gcloud storage buckets update gs://<STATE_BUCKET> --versioning
#
# This block merges with the terraform{} block in versions.tf (only one backend
# is allowed across the whole configuration).
terraform {
  backend "gcs" {
    prefix = "phil-canvas/state"
  }
}
