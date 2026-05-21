#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

KIND_BIN="${KIND:-${ROOT_DIR}/bin/kind}"
KUBECTL_BIN="${KUBECTL:-kubectl}"
HELM_BIN="${HELM:-helm}"
CONTAINER_TOOL_BIN="${CONTAINER_TOOL:-docker}"
JOB_IMAGE="${JOB_IMG:-ghcr.io/magosproject/magos/job:local}"

KIND_CLUSTER_NAME="${KIND_CLUSTER:-magos-test}"
MAGOS_NAMESPACE="${MAGOS_NAMESPACE:-magos-system}"
MAGOS_RELEASE="${MAGOS_RELEASE:-magos}"

POSTGRES_LOCAL_PORT="${MAGOS_POSTGRES_LOCAL_PORT:-15432}"
LOGS_LOCAL_PORT="${MAGOS_LOGS_LOCAL_PORT:-9000}"
LOGS_CONSOLE_LOCAL_PORT="${MAGOS_LOGS_CONSOLE_LOCAL_PORT:-9001}"
API_PORT="${MAGOS_API_PORT:-8080}"
UI_PORT="${MAGOS_UI_PORT:-5173}"
CONTROLLER_HEALTH_PORT="${MAGOS_CONTROLLER_HEALTH_PORT:-8081}"

b64decode() {
  base64 --decode 2>/dev/null || base64 -D
}

ensure_kind_cluster() {
  if ! "${KIND_BIN}" get clusters | grep -qx "${KIND_CLUSTER_NAME}"; then
    echo "ERROR: kind cluster '${KIND_CLUSTER_NAME}' not found. Run 'make kind-cluster' first." >&2
    exit 1
  fi
}

wait_for_port() {
  local port="$1"

  for _ in $(seq 1 50); do
    if nc -z 127.0.0.1 "${port}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.2
  done

  echo "ERROR: timed out waiting for localhost:${port}" >&2
  return 1
}

get_secret_value() {
  local secret_name="$1"
  local field="$2"

  "${KUBECTL_BIN}" -n "${MAGOS_NAMESPACE}" get secret "${secret_name}" -o "jsonpath={.data.${field}}" | b64decode
}

start_port_forward() {
  local pid_var="$1"
  local service_name="$2"
  shift 2

  "${KUBECTL_BIN}" -n "${MAGOS_NAMESPACE}" port-forward "svc/${service_name}" "$@" >/tmp/"${service_name}"-port-forward.log 2>&1 &
  printf -v "${pid_var}" '%s' "$!"
}

wait_for_processes() {
  while true; do
    for pid in "${PIDS[@]}"; do
      if ! kill -0 "${pid}" 2>/dev/null; then
        return 0
      fi
    done
    sleep 1
  done
}

ensure_kind_cluster

"${CONTAINER_TOOL_BIN}" build -t "${JOB_IMAGE}" -f cmd/job/Dockerfile .
"${KIND_BIN}" load docker-image "${JOB_IMAGE}" --name "${KIND_CLUSTER_NAME}"

"${KUBECTL_BIN}" get namespace "${MAGOS_NAMESPACE}" >/dev/null 2>&1 || "${KUBECTL_BIN}" create namespace "${MAGOS_NAMESPACE}"
"${HELM_BIN}" upgrade --install "${MAGOS_RELEASE}" charts/magos/ \
  --namespace "${MAGOS_NAMESPACE}" \
  --set jobImage.tag=local --set jobImage.pullPolicy=Never \
  --set ui.enabled=false \
  --set api.enabled=false \
  --set controllers.workspace.enabled=false \
  --set controllers.project.enabled=false \
  --set controllers.rollout.enabled=false \
  --set controllers.variableset.enabled=false \
  --set controllers.refwatcher.enabled=false \
  --wait --timeout=5m

echo "UI:             http://127.0.0.1:${UI_PORT}"
echo "API:            http://127.0.0.1:${API_PORT}"
echo "Postgres:       127.0.0.1:${POSTGRES_LOCAL_PORT}"
echo "RustFS S3:      http://127.0.0.1:${LOGS_LOCAL_PORT}"
echo "RustFS Console: http://127.0.0.1:${LOGS_CONSOLE_LOCAL_PORT}"

trap 'kill 0' EXIT

start_port_forward postgres_pf_pid "${MAGOS_RELEASE}-postgres" "${POSTGRES_LOCAL_PORT}:5432"
start_port_forward rustfs_pf_pid "${MAGOS_RELEASE}-rustfs" "${LOGS_LOCAL_PORT}:9000" "${LOGS_CONSOLE_LOCAL_PORT}:9001"

for port in "${POSTGRES_LOCAL_PORT}" "${LOGS_LOCAL_PORT}" "${LOGS_CONSOLE_LOCAL_PORT}"; do
  wait_for_port "${port}"
done

MAGOS_POSTGRES_USER="$(get_secret_value "${MAGOS_RELEASE}-postgres" username)"
MAGOS_POSTGRES_DATABASE="$(get_secret_value "${MAGOS_RELEASE}-postgres" database)"
MAGOS_POSTGRES_PASSWORD="$(get_secret_value "${MAGOS_RELEASE}-postgres" password)"
MAGOS_LOGS_S3_ACCESS_KEY_ID="$(get_secret_value "${MAGOS_RELEASE}-rustfs" accessKey)"
MAGOS_LOGS_S3_SECRET_ACCESS_KEY="$(get_secret_value "${MAGOS_RELEASE}-rustfs" secretKey)"

export MAGOS_JOB_IMAGE="${JOB_IMAGE}"
export MAGOS_WORKSPACE_PVC_SIZE_DEFAULT=1Gi
export MAGOS_WORKSPACE_JOB_CPU_REQUEST=125m
export MAGOS_WORKSPACE_JOB_MEMORY_REQUEST=128Mi
export MAGOS_WORKSPACE_JOB_CPU_LIMIT=250m
export MAGOS_WORKSPACE_JOB_MEMORY_LIMIT=256Mi
export MAGOS_LOGS_API_URL="http://127.0.0.1:${API_PORT}"
export MAGOS_LOGS_S3_ENDPOINT="http://127.0.0.1:${LOGS_LOCAL_PORT}"
export MAGOS_POSTGRES_HOST=127.0.0.1
export MAGOS_POSTGRES_PORT="${POSTGRES_LOCAL_PORT}"
export MAGOS_POSTGRES_SSLMODE=disable
export MAGOS_POSTGRES_USER
export MAGOS_POSTGRES_DATABASE
export MAGOS_POSTGRES_PASSWORD
export MAGOS_LOGS_S3_ACCESS_KEY_ID
export MAGOS_LOGS_S3_SECRET_ACCESS_KEY

go run ./cmd/main.go \
  --enable-workspace-controller \
  --enable-project-controller \
  --enable-rollout-controller \
  --enable-variableset-controller \
  --enable-refwatcher-controller \
  --health-probe-bind-address=":${CONTROLLER_HEALTH_PORT}" &
controller_pid=$!

(cd api && PORT="${API_PORT}" go run ./cmd/api/main.go) &
api_pid=$!

(cd ui && npm run dev -- --host 127.0.0.1 --port "${UI_PORT}") &
ui_pid=$!

PIDS=(
  "${controller_pid}"
  "${api_pid}"
  "${ui_pid}"
  "${postgres_pf_pid}"
  "${rustfs_pf_pid}"
)

wait_for_processes

status=0
for pid in "${PIDS[@]}"; do
  if ! kill -0 "${pid}" 2>/dev/null; then
    wait "${pid}" || status=$?
    break
  fi
done

exit "${status}"
