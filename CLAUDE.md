# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this project is

Magos is a Kubernetes operator for declarative management of Terraform configurations. Users define custom resources (Projects, Workspaces, Rollouts, VariableSets) that drive Terraform plan/apply runs inside ephemeral Kubernetes Pods.

## Components

The system has four independently deployable components:

- **Controller** (`/cmd/main.go`, `/internal/controller/`) — Kubernetes operator built on controller-runtime. Runs five reconcilers: Workspace, Project, Rollout, VariableSet, RefWatcher.
- **Job executor** (`/cmd/job/main.go`) — Runs inside the ephemeral Pod that the Workspace controller creates. Clones the Git repo, installs Terraform/OpenTofu, runs plan or apply, and ships logs to S3.
- **API server** (`/api/`) — Standalone Go module. HTTP/2 server exposing REST endpoints for all resource types plus SSE for real-time updates. Typed Kubernetes clients are auto-generated under `/api/internal/generated/`.
- **UI** (`/ui/`) — React Router 7 app in TypeScript. Consumes the API via a generated OpenAPI client (`/ui/app/api/`). Uses Mantine for UI components and TailwindCSS for styling.

Log output from runs is stored in an S3-compatible store. Locally this is RustFS (a Rust S3 implementation in `/rustfs/`). In production any S3-compatible endpoint works.

## Code generation pipeline

Changes to CRD types in `/types/` require running the full generation pipeline:

```
make generate          # deepcopy + K8s clients + OpenAPI spec + TS types
make manifests         # CRD / RBAC / webhook manifests
```

Each step depends on the previous one: types → manifests/deepcopy → Kubernetes clients → OpenAPI spec → TypeScript types. After changing API handlers or adding endpoints, regenerate the OpenAPI spec with `make generate-swagger`, then regenerate TS types with `make generate-ui-types`.

## Common commands

```bash
# Go
make build             # compile controller binary
make fmt               # gofmt
make vet               # go vet
make lint              # golangci-lint
make lint-fix          # golangci-lint with auto-fixes
make test              # unit tests with coverage

# UI
make -C ui lint        # ESLint + tsc type check
make -C ui build       # production build

# Local development (all components together)
make deps              # install Go and npm dependencies
make run               # start controller + API + UI + RustFS in parallel
make run-controller    # controller only
make run-api           # API server only
make run-ui            # UI dev server only
make run-rustfs        # RustFS only

# Kubernetes / testing
make kind-cluster      # create a local Kind cluster
make install           # install CRDs into current cluster
make test-e2e          # e2e tests against Kind
make test-chainsaw     # Chainsaw behavioral tests
```

To run a single Go test: `go test ./internal/controller/workspace/... -run TestName`

## Architecture details worth knowing

**Workspace reconciler** is the most complex controller. It creates and monitors Pods that run the job executor. The executor is a separate binary and container image from the controller.

**Rollout controller** selects Workspaces by label selector and drives them through execution phases. This is the main abstraction for bulk operations.

**API server** is a separate Go module at `/api/` with its own `go.mod`. It imports the CRD types from the root module. Generated clients live under `/api/internal/generated/` and are produced by `k8s.io/code-generator` — do not edit these files by hand.

**SSE endpoints** in the API server push real-time status updates to the UI. The UI uses these rather than polling.

**RustFS** (`/rustfs/`) is a Git submodule. For local development it is started via Docker by `make run-rustfs`. The log store abstraction is at `/internal/logstore/`.

**Helm chart** at `/charts/magos/` is the production deployment method. Chainsaw tests install via this chart on a Kind cluster.
