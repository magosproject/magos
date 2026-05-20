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

Build all images, load them into the cluster, and install the chart:

```bash
make dev
```

Expose the UI and API to your browser:

```bash
make port-forward
```

In another terminal, apply the sample resources:

```bash
kubectl apply -f samples/marketplace-edge/
# or
kubectl apply -f samples/money-movement/
```

Observe what's running:

```bash
kubectl get projects,workspaces,rollouts -A
kubectl get jobs -A
```

Visit the Magos UI at `http://localhost:8080` to see your Workspaces in action.

Tear down:

```bash
make uninstall
make kind-cluster-delete
```

## Contributing

We deeply value inner-source contributions, but ask you to approach them carefully—Magos's strength comes from stability, not flexibility. Every change must be evaluated against its impact on a large number of configurations, not just its technical brilliance. To ensure we maintain the project's clarity and reliability, we prioritize proposals over pull requests: this creates a record for discussion, and prevents wasted effort on misaligned work. While we strive to review every contribution (big or small), we're uncompromising about Magos's platform principles—we'll reject even clever solutions if they add significant complexity.

If you're considering a contribution, start with an issue or RFC—not code—so we can collaborate on the why before the how. This rigor is what keeps our Control Plane API stable and performant. Thank you for your understanding and commitment to making Magos better! 💛
