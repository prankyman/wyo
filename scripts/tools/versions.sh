#!/usr/bin/env bash
# The one place where the versions of the Go tools are written. Read by install.sh and by the pipeline.
# A new version is a change of this file only. Verified on 2026-11-17:
#   golangci-lint  https://github.com/golangci/golangci-lint/releases (latest v2.15.0)
#   gofumpt        https://github.com/mvdan/gofumpt/releases (latest v0.12.0)
#   oapi-codegen   https://github.com/oapi-codegen/oapi-codegen/releases (v2.8.2)
#   sqlc           https://github.com/sqlc-dev/sqlc/releases (latest v1.31.1; the release publishes no checksum file, so the
#                  sha256 below was computed from the downloaded archives on 2026-11-17)
#   ShellCheck     https://github.com/koalaman/shellcheck/releases (latest v0.11.0; sha256 computed the same way)
# shellcheck disable=SC2034
GOLANGCI_LINT_VERSION="2.15.0"
GOFUMPT_VERSION="v0.12.0"
OAPI_CODEGEN_VERSION="v2.8.2"
SQLC_VERSION="1.31.1"
SQLC_SHA256_LINUX_AMD64="497ae4fcdfa64c5b0c311ffe4c2bd991e43991e82e5367792ed78bc2dca27354"
SQLC_SHA256_LINUX_ARM64="b7cae247740d0c51a1e657479e5b2d21e6fef428f596682a01bc55bf4ab8a23d"
# The Mac mini of the final runs (scripts/remote/): the checksum GitHub gives for the release asset.
SQLC_SHA256_DARWIN_ARM64="21602158c99eb1f2bae197a66abfb1941d1e9e50b23125bb193349c6b1acc71e"
SHELLCHECK_VERSION="0.11.0"
SHELLCHECK_SHA256_LINUX_AMD64="b7af85e41cc99489dcc21d66c6d5f3685138f06d34651e6d34b42ec6d54fe6f6"
SHELLCHECK_SHA256_LINUX_ARM64="68a8133197a50beb8803f8d42f9908d1af1c5540d4bb05fdfca8c1fa47decefc"
SHELLCHECK_SHA256_DARWIN_ARM64="339b930feb1ea764467013cc1f72d09cd6b869ebf1013296ba9055ab2ffbd26f"
