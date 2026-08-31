#!/usr/bin/env bash
#
# Build and push the kakao-bot Docker image to a private repository in the
# Oracle Cloud Infrastructure Registry (OCIR) for ARM64 deployment on a2.oci.0xc0de1ab.dev.
#
# Configuration is read from environment variables, falling back to the .env
# file at the repository root:
#
#   OCI_REGISTRY           Registry endpoint (default: icn.ocir.io)
#   OCI_TENANCY_NAMESPACE  Tenancy namespace (default: cnywk2t2q7tb)
#   OCI_USERNAME           Oracle Cloud username (e.g. oracleidentitycloudservice/dh.kam)
#   OCI_AUTH_TOKEN         OCI auth token
#   IMAGE_REPOSITORY       Repository name in OCIR (default: kakao-bot)
#   IMAGE_TAG              Image tag (default: YYYYMMDD-HHMMSS-arm64)
#   PLATFORMS              Target platform (default: linux/arm64)
#
# Usage:
#   ./tools/manage.sh login            Authenticate to OCIR
#   ./tools/manage.sh build [tag]      Build the ARM64 image locally
#   ./tools/manage.sh push [tag]       Build and push ARM64 image to OCIR
#   ./tools/manage.sh release [tag]    Run release build and push to OCIR
#

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

OCI_REGISTRY="${OCI_REGISTRY:-icn.ocir.io}"
OCI_TENANCY_NAMESPACE="${OCI_TENANCY_NAMESPACE:-cnywk2t2q7tb}"
IMAGE_REPOSITORY="${IMAGE_REPOSITORY:-kakao-bot}"
PLATFORMS="${PLATFORMS:-linux/arm64}"
BUILDER_NAME="kakaobot-builder"

die() {
    echo "ERROR: $*" >&2
    exit 1
}

load_env_file() {
    local env_file="$1"
    [[ -f "$env_file" ]] || return 0

    local line key val
    while IFS= read -r line || [[ -n "$line" ]]; do
        line="${line%$'\r'}"
        line="${line#"${line%%[![:space:]]*}"}"
        line="${line%"${line##*[![:space:]]}"}"
        [[ -z "$line" || "$line" == \#* ]] && continue
        line="${line#export }"
        key="${line%%=*}"
        [[ "$line" == *=* && "$key" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || continue
        val="${line#*=}"
        val="${val#\"}"; val="${val%\"}"
        val="${val#\'}"; val="${val%\'}"
        [[ -n "${!key:-}" ]] || export "$key=$val"
    done < "$env_file"
}

require_vars() {
    local name
    for name in "$@"; do
        [[ -n "${!name:-}" ]] || die "required variable $name is not set (export it or add it to $REPO_ROOT/.env)"
    done
}

git_commit() {
    git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null || echo none
}

git_version() {
    git -C "$REPO_ROOT" describe --tags --always --dirty 2>/dev/null || echo dev
}

resolve_image_ref() {
    IMAGE_REF="$OCI_REGISTRY/$OCI_TENANCY_NAMESPACE/$IMAGE_REPOSITORY"
}

resolve_version() {
    if [[ -n "${1:-}" ]]; then
        VERSION="$1"
    elif [[ -n "${IMAGE_TAG:-}" ]]; then
        VERSION="$IMAGE_TAG"
    else
        VERSION="$(date +%Y%m%d-%H%M%S)-arm64"
    fi
}

ensure_builder() {
    if ! docker buildx inspect "$BUILDER_NAME" >/dev/null 2>&1; then
        docker buildx create --name "$BUILDER_NAME" --driver docker-container --bootstrap
    fi
}

build_args() {
    local arch="${PLATFORMS#linux/}"
    printf '%s\n' \
        --build-arg "VERSION=$VERSION" \
        --build-arg "COMMIT=$(git_commit)" \
        --build-arg "DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
        --build-arg "TARGETOS=linux" \
        --build-arg "TARGETARCH=${arch:-arm64}"
}

cmd_login() {
    require_vars OCI_USERNAME OCI_AUTH_TOKEN

    printf '%s' "$OCI_AUTH_TOKEN" | docker login "$OCI_REGISTRY" \
        --username "$OCI_TENANCY_NAMESPACE/$OCI_USERNAME" \
        --password-stdin
    echo "✅ Logged in to $OCI_REGISTRY"
}

cmd_build() {
    resolve_image_ref
    resolve_version "${1:-}"

    echo "🔨 Building Docker image ($PLATFORMS) for $IMAGE_REF:$VERSION..."

    ensure_builder
    docker buildx build \
        --builder "$BUILDER_NAME" \
        --platform "$PLATFORMS" \
        $(build_args) \
        --tag "$IMAGE_REF:$VERSION" \
        --tag "$IMAGE_REF:latest-arm64" \
        --load \
        "$REPO_ROOT"

    echo "✅ Built $IMAGE_REF:$VERSION ($PLATFORMS)"
}

cmd_push() {
    resolve_image_ref
    resolve_version "${1:-}"

    echo "🚀 Building and pushing Docker image ($PLATFORMS) to $IMAGE_REF:$VERSION..."

    ensure_builder
    docker buildx build \
        --builder "$BUILDER_NAME" \
        --platform "$PLATFORMS" \
        $(build_args) \
        --tag "$IMAGE_REF:$VERSION" \
        --tag "$IMAGE_REF:latest-arm64" \
        --push \
        "$REPO_ROOT"

    echo "✅ Pushed $IMAGE_REF:$VERSION to OCIR"
}

cmd_release() {
    resolve_version "${1:-}"

    echo "📦 Building standalone release binary for linux-arm64..."
    make -C "$REPO_ROOT" linux-arm64-release

    cmd_push "$VERSION"
}

usage() {
    awk 'NR > 1 { if ($0 !~ /^#/) exit; sub(/^# ?/, ""); print }' "${BASH_SOURCE[0]}"
}

main() {
    load_env_file "$REPO_ROOT/.env"

    local cmd="${1:-help}"
    shift || true

    case "$cmd" in
        login) cmd_login "$@" ;;
        build) cmd_build "$@" ;;
        push) cmd_push "$@" ;;
        release) cmd_release "$@" ;;
        help | -h | --help) usage ;;
        *) usage; die "unknown command: $cmd" ;;
    esac
}

main "$@"
