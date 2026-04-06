# Magos

Magos is a Kubernetes-native operator for the declarative management of Terraform and OpenTofu infrastructure.

## Overview

The Magos operator manages the lifecycle of your infrastructure directly from Kubernetes. By defining a `Workspace` custom resource, Magos automates `terraform` plans and applies within your cluster.

| Controller  | Resource      | Description                                                                 |
|------------|---------------|-----------------------------------------------------------------------------|
| project    | `Project`     | Defines the boundary for related Workspaces, VariableSets, and Rollouts.   |
| workspace  | `Workspace`   | Runs Terraform plans and applies in isolated, ephemeral Pods.              |
| rollout    | `Rollout`     | Bound to a Project and orchestrates matching Workspaces via label selectors. |
| variableset| `VariableSet` | Defines reusable variables and configuration shared across Projects and Workspaces. |

## Development

### Prerequisites

- Go 1.25+
- Docker
- Kind
- Helm
- kubectl

### Local Development

Create a local Kind cluster:

```bash
kind create cluster
```

Build and locally run the operator:

```bash
make run-controller
```

Apply the sample resources:

```bash
kubectl apply -f samples/
```

Observe what's running:

```bash
kubectl get projects
kubectl get workspaces
kubectl get jobs
kubectl rollouts
```

## Running E2E Tests Locally

E2E tests deploy the operator to a Kind cluster and run Workspace reconciliation.

### Run the Tests

```bash
# Run all E2E tests
make test-e2e

# Run with cleanup skipped (for debugging)
SKIP_CLEANUP=true make test-e2e

# Skip infrastructure installation if already installed
SKIP_INFRA_INSTALL=true make test-e2e

# Skip credential validation (for basic deployment tests)
SKIP_CREDENTIAL_CHECK=true make test-e2e
```

By default all the test resources will be run. However, we can filter scenarios with the `SCENARIO_FILTER` flag. This filters based on the ginkgo labels. This can be used to e.g. only run tests for specific Workspaces with `SCENARIO_FILTER=workspaces make test-e2e`. The filters are additive.

## Contributing

We deeply value inner-source contributions, but ask you to approach them carefully—Magos's strength comes from stability, not flexibility. Every change must be evaluated against its impact on a large number of configurations, not just its technical brilliance. To ensure we maintain the project's clarity and reliability, we prioritize proposals over pull requests: this creates a record for discussion, and prevents wasted effort on misaligned work. While we strive to review every contribution (big or small), we're uncompromising about Magos's platform principles—we'll reject even clever solutions if they add significant complexity.

If you're considering a contribution, start with an issue or RFC—not code—so we can collaborate on the why before the how. This rigor is what keeps our Control Plane API stable and performant. Thank you for your understanding and commitment to making Magos better! 💛
