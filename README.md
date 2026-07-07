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
- Node.js
- npm

### Local Development

Create a local Kind cluster:

```bash
make kind-cluster
```

#### In-Cluster Flow

Run the full stack in Kind:

```bash
make dev
```

Expose the in-cluster UI and API locally:

```bash
make port-forward
```

Open the UI at `http://localhost:8080`.

#### UI Hot Reload Flow

Keep the API, controllers, jobs, Postgres, and RustFS in Kind via the Helm chart, but run the React UI locally for hot reloading:

```bash
make run
```

This command rebuilds and deploys the in-cluster backend with the UI disabled, port-forwards the in-cluster API to `http://localhost:8080`, and starts the React dev server at `http://localhost:5173`.
The Helm override for this flow lives in [hack/values.ui-hot-reload.yaml](hack/values.ui-hot-reload.yaml).

#### Sample Resources

In another terminal, apply one of the sample stacks:

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

#### BucketGit Sources

Magos can clone BucketGit repositories from Workspace source URLs. The default
job image includes the `bgit` binary and `git-remote-bgit` helper. Use an
explicit `bgit+` prefix when a URL should be fetched with `bgit clone` instead
of the built-in Go Git client:

```yaml
source:
  repoURL: bgit+https://broker.example.com/demo.git
  targetRevision: main
  path: infra
```

Local-broker storage URLs are supported through `bgit clone` as well:

```yaml
source:
  repoURL: bgit+file://demo.git
  targetRevision: main
```

```yaml
source:
  repoURL: s3://demo.git
  targetRevision: main
```

```yaml
source:
  repoURL: gs://demo.git
  targetRevision: main
```

Native Git remote-helper URLs are also supported when `git-remote-bgit` is on
the job image `PATH`:

```yaml
source:
  repoURL: bgit::https://broker.example.com/demo.git
  targetRevision: main
```

#### Tear Down

Uninstall Magos and delete the Kind cluster:

```bash
make uninstall
make kind-cluster-delete
```

## Contributing

We deeply value inner-source contributions, but ask you to approach them carefully—Magos's strength comes from stability, not flexibility. Every change must be evaluated against its impact on a large number of configurations, not just its technical brilliance. To ensure we maintain the project's clarity and reliability, we prioritize proposals over pull requests: this creates a record for discussion, and prevents wasted effort on misaligned work. While we strive to review every contribution (big or small), we're uncompromising about Magos's platform principles—we'll reject even clever solutions if they add significant complexity.

If you're considering a contribution, start with an issue or RFC—not code—so we can collaborate on the why before the how. This rigor is what keeps our Control Plane API stable and performant. Thank you for your understanding and commitment to making Magos better! 💛
