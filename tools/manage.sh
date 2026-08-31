#!/usr/bin/env bash
#
# Build and push the kakao-bot Docker image to a private repository in the
# Oracle Cloud Infrastructure Registry (OCIR).
#
# Configuration is read from environment variables, falling back to the .env
# file at the repository root:
#
#   OCI_REGION             OCI region code, e.g. ap-seoul-1
#   OCI_TENANCY_NAMESPACE  Tenancy object storage namespace
#   OCI_USERNAME           Oracle Cloud username; federated users use
#                          oracleidentitycloudservice/<email>
#   OCI_AUTH_TOKEN         OCI auth token (not the account password)
#   OCI_REGISTRY           Optional registry endpoint override
#                          (default: <OCI_REGION>.ocir.io)
#   IMAGE_REPOSITORY       Repository name in OCIR (default: kakao-bot)
#   IMAGE_TAG              Image tag (default: git describe --always --dirty)
#   PLATFORMS              Target platforms, comma separated
#                          (default: native platform; multi-arch requires
#                          "push", which publishes the manifest directly)
#
# Usage:
#   ./tools/manage.sh login            Authenticate to OCIR
#   ./tools/manage.sh build [tag]      Build the image locally
#   ./tools/manage.sh push [tag]       Build and push to OCIR

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

IMAGE_REPOSITORY="${IMAGE_REPOSITORY:-kakao-bot}"
PLATFORMS="${PLATFORMS:-}"
BUILDER_NAME="kakaobot-builder"

die() {
    echo "error: $*" >&2
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

native_platform() {
    docker version --format '{{.Server.Os}}/{{.Server.Arch}}'
}

resolve_registry() {
    if [[ -z "${OCI_REGISTRY:-}" ]]; then
        require_vars OCI_REGION
        OCI_REGISTRY="${OCI_REGION}.ocir.io"
    fi
}

resolve_image_ref() {
    resolve_registry
    require_vars OCI_TENANCY_NAMESPACE
    IMAGE_REF="$OCI_REGISTRY/$OCI_TENANCY_NAMESPACE/$IMAGE_REPOSITORY"
}

resolve_version() {
    VERSION="${1:-${IMAGE_TAG:-}}"
    VERSION="${VERSION:-$(git_version)}"
}

is_multi_platform() {
    [[ "$PLATFORMS" == *,* ]]
}

ensure_builder() {
    docker buildx inspect "$BUILDER_NAME" >/dev/null 2>&1 ||
        docker buildx create --name "$BUILDER_NAME" --driver docker-container --bootstrap
}

build_args() {
    printf '%s\n' \
        --build-arg "VERSION=$VERSION" \
        --build-arg "COMMIT=$(git_commit)" \
        --build-arg "DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}

cmd_login() {
    resolve_registry
    require_vars OCI_TENANCY_NAMESPACE OCI_USERNAME OCI_AUTH_TOKEN

    printf '%s' "$OCI_AUTH_TOKEN" | docker login "$OCI_REGISTRY" \
        --username "$OCI_TENANCY_NAMESPACE/$OCI_USERNAME" \
        --password-stdin
    echo "Logged in to $OCI_REGISTRY"
}

cmd_build() {
    resolve_image_ref
    resolve_version "${1:-}"

    if is_multi_platform; then
        die "multi-platform builds (--platform $PLATFORMS) cannot be loaded locally; use: ./tools/manage.sh push"
    fi

    docker buildx build \
        --platform "${PLATFORMS:-$(native_platform)}" \
        $(build_args) \
        --tag "$IMAGE_REF:$VERSION" \
        --tag "$IMAGE_REF:latest" \
        --load \
        "$REPO_ROOT"

    echo "Built $IMAGE_REF:$VERSION"
}

cmd_push() {
    resolve_image_ref
    resolve_version "${1:-}"

    if is_multi_platform; then
        ensure_builder
        docker buildx build \
            --builder "$BUILDER_NAME" \
            --platform "$PLATFORMS" \
            $(build_args) \
            --tag "$IMAGE_REF:$VERSION" \
            --tag "$IMAGE_REF:latest" \
            --push \
            "$REPO_ROOT"
    else
        docker buildx build \
            --platform "${PLATFORMS:-$(native_platform)}" \
            $(build_args) \
            --tag "$IMAGE_REF:$VERSION" \
            --tag "$IMAGE_REF:latest" \
            --load \
            "$REPO_ROOT"
        docker push "$IMAGE_REF:$VERSION"
        docker push "$IMAGE_REF:latest"
    fi

    echo "Pushed $IMAGE_REF:$VERSION"
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
        help | -h | --help) usage ;;
        *) usage; die "unknown command: $cmd" ;;
    esac
}

main "$@"
