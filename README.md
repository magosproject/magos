# Magos

Magos is a Kubernetes-native operator for the declarative management of Terraform configurations.

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
make kind-cluster
```

Install the CRDs into the cluster:

```bash
make install
```

Build the Magos job image and load it into the cluster:

```bash
make docker-build && make kind-load
```

Build and locally run the operator:

```bash
make run
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

Visit the Magos UI at `http://localhost:5713` to see your Workspaces in action!

## Contributing

We deeply value inner-source contributions, but ask you to approach them carefully—Magos's strength comes from stability, not flexibility. Every change must be evaluated against its impact on a large number of configurations, not just its technical brilliance. To ensure we maintain the project's clarity and reliability, we prioritize proposals over pull requests: this creates a record for discussion, and prevents wasted effort on misaligned work. While we strive to review every contribution (big or small), we're uncompromising about Magos's platform principles—we'll reject even clever solutions if they add significant complexity.

If you're considering a contribution, start with an issue or RFC—not code—so we can collaborate on the why before the how. This rigor is what keeps our Control Plane API stable and performant. Thank you for your understanding and commitment to making Magos better! 💛
