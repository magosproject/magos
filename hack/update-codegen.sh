#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd -P)"

MODULE="github.com/magosproject/magos"
CODE_GENERATOR_VERSION="${CODE_GENERATOR_VERSION:-v0.35.3}"
LOCALBIN="${LOCALBIN:-${REPO_ROOT}/bin}"

export PATH="${LOCALBIN}:${PATH}"

CODEGEN_PKG="$(go env GOMODCACHE)/k8s.io/code-generator@${CODE_GENERATOR_VERSION}"

# shellcheck source=/dev/null
# https://github.com/kubernetes/code-generator/blob/master/kube_codegen.sh
source "${CODEGEN_PKG}/kube_codegen.sh"

kube::codegen::gen_client \
    --with-watch \
    --output-dir "${REPO_ROOT}/api/internal/generated" \
    --output-pkg "${MODULE}/api/internal/generated" \
    --boilerplate "${REPO_ROOT}/hack/boilerplate.go.txt" \
    "${REPO_ROOT}/types"

