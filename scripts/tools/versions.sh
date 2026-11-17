#!/usr/bin/env bash
# The one place where the versions of the Go tools are written. Read by install.sh and by the pipeline.
# A new version is a change of this file only. Verified on 2026-11-17:
#   golangci-lint  https://github.com/golangci/golangci-lint/releases (latest v2.15.0)
#   gofumpt        https://github.com/mvdan/gofumpt/releases (latest v0.12.0)
#   oapi-codegen   https://github.com/oapi-codegen/oapi-codegen/releases (v2.8.2)
# shellcheck disable=SC2034
GOLANGCI_LINT_VERSION="2.15.0"
GOFUMPT_VERSION="v0.12.0"
OAPI_CODEGEN_VERSION="v2.8.2"
