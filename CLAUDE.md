# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Magos is a Kubernetes-native operator that reconciles Terraform configurations. A `Workspace` CR points at a Git source and a Terraform version; the workspace controller turns each change into a Plan Job and an Apply Job that run `terraform` inside ephemeral pods.

Authoritative concept docs live next door at `~/github/magos/website/contents/docs/`. The `concepts/` and `reference/` sections in that tree are the source of truth for behavior and surface area. Read them before designing new features. Project root is `~/github/magos/magos`.

## Commands

All commands run from the repo root. The Makefile is the canonical entry point and self-installs every tool it needs into `./bin/`.

```
make build              # go build -o bin/manager cmd/main.go
make test               # envtest-backed unit/integration tests with coverage
make test-chainsaw      # chainsaw end-to-end controller tests (no chart install)
make test-chainsaw-chart  # chart install tests; requires helm install of magos in magos-system
make lint               # golangci-lint
make lint-fix
make manifests          # regenerate CRDs from ./types/...
make generate           # full codegen pipeline: deepcopy, clientset, OpenAPI, TS types
```

Running a single Go test:

```
go test ./internal/controller/workspace -run TestComputeNextReconcileTime -v
```

`make test` first runs `manifests generate fmt vet setup-envtest`; the standalone `go test` form above skips that and is faster when iterating on one test. `setup-envtest` downloads the kube API server binaries to `./bin/` keyed on the version of `k8s.io/api` in `go.mod`.

Running a single chainsaw test (note: chainsaw lives in `./bin/` after first install):

```
./bin/chainsaw test test/chainsaw/tests/workspace/plan-apply-end-to-end
```

Chainsaw tests use real CRs against an envtest cluster. They cover integration; Go unit tests stay small and use `testify/assert`.

### Local dev loop

```
make kind-cluster                  # one-time: creates Kind cluster magos-test with port maps
make docker-build && make kind-load  # build + load all four images into the cluster
make install                       # apply CRDs + render the local chart
make run                           # runs controller, API, and UI on host, with chart deps in-cluster
```

`make run` is the daily inner loop. It installs the chart from `hack/local-values.yaml`, which deliberately sets `api.enabled=false`, `ui.enabled=false`, and every `controllers.*.enabled=false`, then runs those components from the host with environment variables piped in from the in-cluster `magos-rustfs` (S3-compatible) and `magos-postgres` Secrets. Logs from controller, API, and UI all stream into the same stdout. UI: `http://localhost:5713`. API: `http://localhost:8080` (Swagger at `/docs`).

The controller binary runs with all five `--enable-*-controller` flags by default (`ARGS` in the Makefile). When iterating on one controller, override `ARGS` to disable the others so log noise drops.

## Architecture

### Two Go modules in one repo

The root module `github.com/magosproject/magos` contains the controllers, the job binary, the CRD types, and the shared `internal/logstore`. The API server is a separate module at `api/` (`github.com/magosproject/magos/api`) with its own `go.mod`. The API depends on the root module via a local replace; the root never depends on the API. `make deps` runs `go mod tidy` in both, plus `npm install` in `ui/`.

### Five controllers, one set of CRs, one annotation handshake

Controllers do not call each other. They watch the same resources and coordinate through annotations on `Workspace`. Each controller is enabled by a flag (`--enable-workspace-controller`, etc.) so they can be deployed as separate Deployments in production but co-located in `make run` for dev.

- **workspace**: turns spec changes into Plan and Apply Jobs. Jobs are named `<workspace>-plan-<specHash>` / `<workspace>-apply-<specHash>` so an approval cannot apply a plan that the spec no longer matches. Plan and Apply pods share a per-Workspace PVC mounted at `/workspace-data` so the binary plan file flows from plan to apply.
- **project**: picks orchestration mode. If a `Rollout` of the same name exists in the namespace, the project sets `status.reason=ManagedByRollout` and stops touching `magosproject.io/execution-allowed`. Otherwise it adds that annotation to every member Workspace (`DefaultParallel`).
- **rollout**: 1-to-1 with Project (CEL enforces `name == projectRef`). Walks `spec.strategy.steps` in order, granting `execution-allowed` only to the cohort matching the current step's selector. Halts on first failure in a cohort.
- **variableset**: validates that referenced Secret/ConfigMap keys exist. Resolved values never enter the controller process; the kubelet reads them at pod start via `SecretKeySelector`.
- **refwatcher**: polls remote Git refs for Workspaces whose `targetRevision` is a branch or tag, writes the resolved SHA to `magosproject.io/detected-revision`. The workspace controller treats a divergence between that annotation and `status.observedRevision` as a fresh run trigger.

Annotation surface (definitive list in `website/contents/docs/reference/annotations/`):

- `magosproject.io/execution-allowed=true`: orchestrator -> workspace, "you may run"
- `magosproject.io/approved=true`: operator -> workspace, "apply the planned change" (only when `spec.autoApply=false`)
- `magosproject.io/reconcile-request=<unique>`: operator -> workspace, one-shot manual trigger
- `magosproject.io/reconcile-interval=<duration>`: operator -> workspace, override 3m drift interval
- `magosproject.io/detected-revision=<sha>`: refwatcher -> workspace, ref resolved to commit
- `magosproject.io/git-poll-interval=<duration>`: operator -> refwatcher
- `magosproject.io/tf-log-level=<level>`: operator -> workspace, sets `TF_LOG` on Pods

### The job binary is a separate program

`cmd/job/main.go` is the entry point for the `magos-job` image. The workspace controller never runs `terraform` itself; it builds a Job whose pod runs the job binary with everything it needs passed in via environment variables (`REPO_URL`, `TARGET_REVISION`, `TF_VERSION`, `MAGOS_JOB_TYPE=plan|apply`, `MAGOS_PLAN_FILE`, `MAGOS_RUN_ID`, `MAGOS_POLICY_SELECTOR`, ...). The job clones the repo (plan phase only; apply reuses the working tree on the PVC), downloads the requested Terraform version into the PVC's plugin cache, runs the phase, and exits. Required env is validated in `loadConfig`; missing values fail fast.

The job pod's ServiceAccount (`spec.serviceAccountName`, default `magos-job`) is what binds external identity (GKE Workload Identity, IRSA). The chart's `magos-job` SA carries `get;list;watch` on `validatingpolicies.policies.kyverno.io` because the plan phase evaluates Kyverno `ValidatingPolicy` resources against the terraform plan JSON in-process via the embedded CLI when `spec.validation.policySelector` matches.

### Storage

PostgreSQL holds run history (`status.currentRunID` and `Run` records with phase summaries, jobNames, podNames, timings). An S3-compatible bucket (`magos-rustfs` in dev, or any S3/GCS/MinIO in prod) holds gzipped pod logs keyed by run ID and phase. The workspace controller writes to both via `internal/logstore` (for object storage) and an HTTP `RunRecorder` that calls into the API server (for Postgres). Both stores are configured via `MAGOS_LOGS_*` and `MAGOS_POSTGRES_*` env vars; `make run` extracts those from the in-cluster Secrets automatically.

### API + UI

`api/internal/api/server.go` wires informer-backed services (`service/project_service.go`, `workspace_service.go`, ...) into stdlib `net/http` handlers. The server uses `h2c` so the same port speaks HTTP/1.1 and HTTP/2. SSE endpoints (`handlers/sse.go`) push live resource updates to the UI through a `service.Broadcaster`. OpenAPI is generated by `swag` from swaggo annotations on handler comments (`make generate-swagger`), embedded into the binary via `go:embed`, and consumed by the UI's `openapi-typescript` codegen to produce typed clients (`make generate-ui-types`).

The UI (`ui/`) is React Router v7 with file-based routes declared in `app/routes.ts`. Each resource type has a list route and a detail route. Live data flows through SSE hooks (`useSSEItem`, `useSSEStream`); workspace run logs additionally stream pod logs through a dedicated WebSocket-style stream surfaced as `WorkspaceLiveConsole`. UI port in dev: `5713`.

### Code generation pipeline

CRD types in `types/magosproject/v1alpha1/` are the single source of truth. The chain is:

```
types/ -> controller-gen -> charts/magos/resources/crds/*.yaml  (CRDs)
types/ -> controller-gen -> types/.../zz_generated.deepcopy.go  (deepcopy)
types/ -> kube_codegen   -> api/internal/generated/...          (typed clientset, listers, informers)
api/   -> swag           -> api/internal/api/docs/swagger.json  (OpenAPI 3.1)
api/   -> openapi-typescript -> ui/app/api/types.ts             (TS types)
```

After changing CR types: `make manifests generate`. The API regenerates after handler comment changes via `make generate-swagger`. After regenerating swagger, run `make generate-ui-types` to keep the UI's typed client in sync.

### Chart

`charts/magos/` is a single Helm chart that ships every component (CRDs, controllers, API, UI, RustFS, Postgres) behind individual `enabled` toggles. Each controller deployment is rendered conditionally so production installs can scale them independently. Tests for chart rendering and install live under `test/chainsaw/tests/chart/`.

## Conventions to know

- The `magosproject.io/finalizer` is added to Workspaces and Projects so `handleDeletion` runs before reaping. Don't bypass it.
- Spec changes change the spec hash; approval annotations don't. That is what guarantees an approved apply runs the exact reviewed plan.
- `status.observedRevision` is always the resolved commit SHA, not the branch name. Alerts and dashboards key off it.
- Variable precedence (lowest to highest): `terraform.tfvarsPath` < Project `variableSetRef` (in order) < Workspace `variableSetRef` (in order). Within the resolved set, later entries shadow earlier ones on conflicting names. Everything reaches Terraform as `TF_VAR_*`.
- `validation.policySelector` on a Workspace fully overrides the Project default rather than merging.
- Don't expect controllers to talk to each other directly. New cross-controller signals go through CR annotations or status, never function calls.
