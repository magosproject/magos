#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd -P)"

MODULE="github.com/magosproject/magos"

CODEGEN_PKG="$(cd "${REPO_ROOT}" && go list -m -f '{{.Dir}}' k8s.io/code-generator)"

# shellcheck source=/dev/null
source "${CODEGEN_PKG}/kube_codegen.sh"

kube::codegen::gen_client \
    --with-watch \
    --output-dir "${REPO_ROOT}/api/internal/generated" \
    --output-pkg "${MODULE}/api/internal/generated" \
    --boilerplate "${REPO_ROOT}/hack/boilerplate.go.txt" \
    "${REPO_ROOT}/types"

