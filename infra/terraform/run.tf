resource "google_artifact_registry_repository" "backend" {
  location      = var.region
  repository_id = "phil-canvas"
  format        = "DOCKER"
  depends_on    = [google_project_service.apis]
}

resource "google_cloud_run_v2_service" "backend" {
  name                = var.service_name
  location            = var.region
  deletion_protection = false
  ingress             = "INGRESS_TRAFFIC_ALL"

  template {
    service_account = google_service_account.backend.email

    # Max request timeout so long-lived WebSocket connections aren't killed.
    timeout = "3600s"

    # Keep one warm instance: rooms are in-memory, and scale-to-zero would drop
    # active sessions.
    scaling {
      min_instance_count = 1
    }

    containers {
      # Public placeholder until CI (backend.yml) deploys the real image
      # straight to Cloud Run.
      image = "us-docker.pkg.dev/cloudrun/container/hello"

      # Default HTTP/1 (no h2c port name) — WebSocket upgrades need HTTP/1.1.
      ports {
        container_port = 8080
      }

      env {
        name  = "FIREBASE_PROJECT_ID"
        value = var.project_id
      }
      env {
        name  = "STORAGE_BUCKET"
        value = google_storage_bucket.assets.name
      }
      env {
        name  = "ADMIN_ALLOWLIST"
        value = var.admin_allowlist
      }
      env {
        name = "JWT_SECRET"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.jwt.secret_id
            version = "latest"
          }
        }
      }
    }
  }

  depends_on = [
    google_secret_manager_secret_version.jwt,
    google_secret_manager_secret_iam_member.backend_jwt,
  ]

  # Ongoing deploys are done imperatively by CI (deploy job in backend.yml).
  # Lifecycle ignore_changes stops future applies from reverting it back to
  # the placeholder above.
  lifecycle {
    ignore_changes = [template[0].containers[0].image]
  }
}

# Public site: allow unauthenticated invocation (auth is enforced in-app by JWT).
resource "google_cloud_run_v2_service_iam_member" "public" {
  name     = google_cloud_run_v2_service.backend.name
  location = var.region
  role     = "roles/run.invoker"
  member   = "allUsers"
}
