#!/usr/bin/env bash
# Installs the pinned Go tools into .tools/bin (not committed).
#  - golangci-lint: the official release archive, its sha256 checked against the release's own checksums file
#    (https://golangci-lint.run/docs/welcome/install/#binaries).
#  - sqlc and shellcheck: the official release archive, its sha256 checked against the value written in versions.sh
#    (neither release publishes a checksum file). Linux, and macOS on Apple silicon for the Mac mini of the final runs
#    (scripts/remote/).
#  - gofumpt and oapi-codegen: `go install <module>@<version>`; Go checks the module against the public checksum database.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$root"
# shellcheck source=scripts/tools/versions.sh
source "$root/scripts/tools/versions.sh"

bin="$root/.tools/bin"
mkdir -p "$bin"

# A download that stops (less than 10 kB/s for 30 s) is dropped and tried again, up to 5 times: the Mac mini of the final runs once
# waited 4 minutes on a transfer that never moved (2026-11-19).
fetch() { curl -fsSL --retry 5 --retry-all-errors --connect-timeout 20 --speed-limit 10000 --speed-time 30 "$@"; }

case "$(uname -s)" in
Linux) os=linux ;;
Darwin) os=darwin ;;
*)
    echo "unsupported operating system: $(uname -s)" >&2
    exit 1
    ;;
esac
case "$(uname -m)" in
x86_64 | amd64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
*)
    echo "unsupported processor: $(uname -m)" >&2
    exit 1
    ;;
esac

installed_version() {
    # $1 = binary, $2 = the argument that prints its version, $3 = expected text in the output
    [ -x "$bin/$1" ] && "$bin/$1" "$2" 2>&1 | grep -q -F -- "$3"
}

install_golangci_lint() {
    if installed_version golangci-lint version "version $GOLANGCI_LINT_VERSION"; then
        echo "golangci-lint $GOLANGCI_LINT_VERSION is installed"
        return
    fi
    local name="golangci-lint-$GOLANGCI_LINT_VERSION-$os-$arch"
    local base="https://github.com/golangci/golangci-lint/releases/download/v$GOLANGCI_LINT_VERSION"
    local tmp
    tmp="$(mktemp -d)"
    trap 'rm -rf "$tmp"; trap - RETURN' RETURN
    fetch -o "$tmp/$name.tar.gz" "$base/$name.tar.gz"
    fetch -o "$tmp/checksums.txt" "$base/golangci-lint-$GOLANGCI_LINT_VERSION-checksums.txt"
    (
        cd "$tmp"
        awk -v f="$name.tar.gz" '$2 == f' checksums.txt >one.txt
        [ -s one.txt ] || {
            echo "no checksum for $name.tar.gz" >&2
            exit 1
        }
        if command -v sha256sum >/dev/null 2>&1; then
            sha256sum -c one.txt
        else
            shasum -a 256 -c one.txt
        fi
        tar -xzf "$name.tar.gz"
    )
    install -m 0755 "$tmp/$name/golangci-lint" "$bin/golangci-lint"
    echo "golangci-lint $GOLANGCI_LINT_VERSION installed"
}

install_go_tool() {
    # $1 = binary, $2 = module path, $3 = version, $4 = argument printing the version
    if installed_version "$1" "$4" "$3"; then
        echo "$1 $3 is installed"
        return
    fi
    GOBIN="$bin" go install "$2@$3"
    echo "$1 $3 installed"
}

# install_archive <binary> <version text> <url> <sha256> <path of the binary inside the archive>
install_archive() {
    local name="$1" want="$2" url="$3" sum="$4" inside="$5"
    if [ -x "$bin/$name" ] && "$bin/$name" --version 2>&1 | grep -q -F -- "$want"; then
        echo "$name $want is installed"
        return
    fi
    if [ -z "$sum" ]; then
        echo "$name: no archive for $os/$arch is pinned in versions.sh" >&2
        exit 1
    fi
    local tmp
    tmp="$(mktemp -d)"
    trap 'rm -rf "$tmp"; trap - RETURN' RETURN
    fetch -o "$tmp/archive.tar.gz" "$url"
    if command -v sha256sum >/dev/null 2>&1; then
        echo "$sum  $tmp/archive.tar.gz" | sha256sum -c - >/dev/null
    else
        echo "$sum  $tmp/archive.tar.gz" | shasum -a 256 -c - >/dev/null
    fi
    tar -xzf "$tmp/archive.tar.gz" -C "$tmp"
    install -m 0755 "$tmp/$inside" "$bin/$name"
    echo "$name $want installed"
}

install_sqlc() {
    local sum=""
    case "$os/$arch" in
    linux/amd64) sum="$SQLC_SHA256_LINUX_AMD64" ;;
    linux/arm64) sum="$SQLC_SHA256_LINUX_ARM64" ;;
    darwin/arm64) sum="$SQLC_SHA256_DARWIN_ARM64" ;;
    esac
    install_archive sqlc "v$SQLC_VERSION" \
        "https://github.com/sqlc-dev/sqlc/releases/download/v$SQLC_VERSION/sqlc_${SQLC_VERSION}_${os}_$arch.tar.gz" \
        "$sum" sqlc
}

install_shellcheck() {
    local sum="" machine=x86_64
    [ "$arch" = arm64 ] && machine=aarch64
    case "$os/$arch" in
    linux/amd64) sum="$SHELLCHECK_SHA256_LINUX_AMD64" ;;
    linux/arm64) sum="$SHELLCHECK_SHA256_LINUX_ARM64" ;;
    darwin/arm64) sum="$SHELLCHECK_SHA256_DARWIN_ARM64" ;;
    esac
    install_archive shellcheck "version: $SHELLCHECK_VERSION" \
        "https://github.com/koalaman/shellcheck/releases/download/v$SHELLCHECK_VERSION/shellcheck-v$SHELLCHECK_VERSION.$os.$machine.tar.gz" \
        "$sum" "shellcheck-v$SHELLCHECK_VERSION/shellcheck"
}

install_golangci_lint
install_sqlc
install_shellcheck
install_go_tool gofumpt mvdan.cc/gofumpt "$GOFUMPT_VERSION" --version
install_go_tool oapi-codegen github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen "$OAPI_CODEGEN_VERSION" --version
