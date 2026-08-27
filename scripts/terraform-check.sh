#!/usr/bin/env bash
# Runs `terraform fmt -check`, `init -backend=false`, and `validate` against
# infra/terraform using the official hashicorp/terraform Docker image, so
# nobody needs Terraform installed locally. Mirrors the checks
# .github/workflows/infra.yml runs in CI.
#
# Usage:
#   scripts/terraform-check.sh          # fmt check + validate
#   scripts/terraform-check.sh --write  # apply fmt fixes in place, then validate
#
# TERRAFORM_VERSION can be set to override the version; defaults to the same
# 1.9.8 infra.yml pins via hashicorp/setup-terraform@v3.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Mount the infra/ parent, not just infra/terraform: firestore.tf/storage.tf
# read ../firebase/*.rules via file(), which validate must be able to resolve.
INFRA_DIR="$(cd "$SCRIPT_DIR/../infra" && pwd)"
TERRAFORM_VERSION="${TERRAFORM_VERSION:-1.9.8}"

fmt_args=(fmt -diff -recursive)
if [[ "${1:-}" != "--write" ]]; then
  fmt_args+=(-check)
fi

run_tf() {
  docker run --rm \
    --user "$(id -u):$(id -g)" \
    --volume "$INFRA_DIR:/workspace" \
    --workdir /workspace/terraform \
    "hashicorp/terraform:${TERRAFORM_VERSION}" \
    "$@"
}

run_tf "${fmt_args[@]}"
# -backend=false: validate config + providers without touching the GCS
# backend or needing credentials.
run_tf init -backend=false
run_tf validate
