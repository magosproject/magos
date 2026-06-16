# magos

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: v0.1.0](https://img.shields.io/badge/AppVersion-v0.1.0-informational?style=flat-square)

Magos

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| Bruno Schaatsbergen | <b@bschaatsbergen.com> |  |

## Source Code

* <https://github.com/magosproject/magos>

## Parameters

### Image Parameters

| Name                | Description                                                                  | Value                                   |
| ------------------- | ---------------------------------------------------------------------------- | --------------------------------------- |
| `image.repository`  | Image repository for the Magos controller                                    | `ghcr.io/magosproject/magos/controller` |
| `image.tag`         | Overrides the image tag. The default tag is the value of `.Chart.AppVersion` | `""`                                    |
| `image.pullPolicy`  | Image pull policy                                                            | `IfNotPresent`                          |
| `image.pullSecrets` | List of image pull secret names                                              | `[]`                                    |

### Job Image Parameters

| Name                  | Description                                                                      | Value                            |
| --------------------- | -------------------------------------------------------------------------------- | -------------------------------- |
| `jobImage.repository` | Image repository for the Magos job runner                                        | `ghcr.io/magosproject/magos/job` |
| `jobImage.tag`        | Overrides the job image tag. The default tag is the value of `.Chart.AppVersion` | `""`                             |
| `jobImage.pullPolicy` | Job image pull policy                                                            | `IfNotPresent`                   |
| `nameOverride`        | Overrides the chart name used in resource names                                  | `""`                             |
| `fullnameOverride`    | Fully overrides the release-prefixed resource name                               | `""`                             |

### Global Parameters

These values apply to all components as defaults. Per-component values take
precedence when set. All pod scheduling fields (nodeSelector, tolerations,
affinity, priorityClassName) fall back to the global value when the
component-level field is empty.

| Name                    | Description                                                                 | Value |
| ----------------------- | --------------------------------------------------------------------------- | ----- |
| `global.labels`         | Labels to add to all resources                                              | `{}`  |
| `global.annotations`    | Annotations to add to all resources                                         | `{}`  |
| `global.podLabels`      | Labels to add to all pods                                                   | `{}`  |
| `global.podAnnotations` | Annotations to add to all pods                                              | `{}`  |
| `global.env`            | Environment variables to add to all Magos pods                              | `[]`  |
| `global.envFrom`        | Environment variables sourced from ConfigMaps or Secrets for all Magos pods | `[]`  |
| `global.nodeSelector`   | Default node selector for all Magos pods                                    | `{}`  |
| `global.tolerations`    | Default tolerations for all Magos pods                                      | `[]`  |
| `global.affinity`       | Default affinity rules for all Magos pods                                   | `{}`  |

### CRDs

| Name           | Description                                          | Value  |
| -------------- | ---------------------------------------------------- | ------ |
| `crds.install` | Install and upgrade CRDs as part of the Helm release | `true` |
| `crds.keep`    | Retain CRDs when the release is uninstalled          | `true` |

### RBAC

| Name                              | Description                          | Value  |
| --------------------------------- | ------------------------------------ | ------ |
| `rbac.installClusterRoles`        | Install ClusterRole resources        | `true` |
| `rbac.installClusterRoleBindings` | Install ClusterRoleBinding resources | `true` |

### Leader Election

| Name                     | Description                                       | Value  |
| ------------------------ | ------------------------------------------------- | ------ |
| `leaderElection.enabled` | Enable leader election for the controller manager | `true` |

### Health Probe

| Name                        | Description                                         | Value      |
| --------------------------- | --------------------------------------------------- | ---------- |
| `healthProbe.port`          | Port on which the health probe endpoints are served | `8081`     |
| `healthProbe.livenessPath`  | HTTP path for the liveness probe                    | `/healthz` |
| `healthProbe.readinessPath` | HTTP path for the readiness probe                   | `/readyz`  |

### Metrics Service

| Name                     | Description                                      | Value       |
| ------------------------ | ------------------------------------------------ | ----------- |
| `metricsService.enabled` | Expose a metrics Service for scraping            | `false`     |
| `metricsService.port`    | Port on which the metrics Service listens        | `8080`      |
| `metricsService.type`    | Kubernetes Service type for the metrics endpoint | `ClusterIP` |
| `metricsService.secure`  | Serve metrics over HTTPS                         | `false`     |

### UI Parameters

| Name                           | Description                                                            | Value       |
| ------------------------------ | ---------------------------------------------------------------------- | ----------- |
| `ui.enabled`                   | Deploy the Magos web UI                                                | `true`      |
| `ui.replicas`                  | Number of UI pod replicas                                              | `1`         |
| `ui.host`                      | Hostname used to construct the UI base URL (used in NOTES and ingress) | `localhost` |
| `ui.topologySpreadConstraints` | Topology spread constraints for the UI pods                            | `[]`        |
| `ui.labels`                    | Extra labels for the UI Deployment                                     | `{}`        |
| `ui.annotations`               | Extra annotations for the UI Deployment                                | `{}`        |
| `ui.podLabels`                 | Extra labels for UI pods                                               | `{}`        |
| `ui.podAnnotations`            | Extra annotations for UI pods                                          | `{}`        |
| `ui.env`                       | Additional environment variables for the UI container                  | `[]`        |
| `ui.envFrom`                   | Additional environment variable sources for the UI container           | `[]`        |
| `ui.nodeSelector`              | Node selector for UI pods                                              | `{}`        |
| `ui.tolerations`               | Tolerations for UI pods                                                | `[]`        |
| `ui.affinity`                  | Affinity rules for UI pods                                             | `{}`        |

### API Parameters

| Name                            | Description                                                                                                                                                                                                                                                                                                                                            | Value        |
| ------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------ |
| `auth.enabled`                  | Enable Magos API/UI authentication                                                                                                                                                                                                                                                                                                                     | `true`       |
| `auth.secret.name`              | Name of an existing Secret to use for auth credentials. When set, the chart does **not** generate a Secret. The Secret must contain the following keys: `sessionSigningKey` (random string ≥ 32 chars), `internalToken` (random string), and — when `auth.admin.enabled=true` — `adminPassword` and `adminPasswordHash` (bcrypt hash of the password). | `""`         |
| `auth.session.ttl`              | Duration for which a session cookie is valid (e.g. `24h`, `7d`).                                                                                                                                                                                                                                                                                       | `24h`        |
| `auth.session.cookieSecure`     | Set the `Secure` flag on session cookies. Enable when the UI is served over HTTPS.                                                                                                                                                                                                                                                                     | `false`      |
| `auth.session.allowedOrigins`   | List of allowed CORS origins for credentialed requests (e.g. `["https://magos.example.com"]`).                                                                                                                                                                                                                                                         | `[]`         |
| `auth.admin.enabled`            | Enable the built-in `admin` account. Convenient for development but should be disabled in production in favour of OIDC (`auth.oidc.enabled=true`).                                                                                                                                                                                                     | `true`       |
| `auth.oidc.enabled`             | Enable OIDC authentication. Recommended for production instead of the built-in admin account.                                                                                                                                                                                                                                                          | `false`      |
| `auth.oidc.issuerURL`           | OIDC issuer URL (e.g. https://accounts.google.com)                                                                                                                                                                                                                                                                                                     | `""`         |
| `auth.oidc.clientID`            | OIDC client ID                                                                                                                                                                                                                                                                                                                                         | `""`         |
| `auth.oidc.additionalScopes`    | Additional OAuth2 scopes to request alongside `openid profile email`.                                                                                                                                                                                                                                                                                  | `["groups"]` |
| `api.enabled`                   | Deploy the Magos API server                                                                                                                                                                                                                                                                                                                            | `true`       |
| `api.replicas`                  | Number of API pod replicas                                                                                                                                                                                                                                                                                                                             | `1`          |
| `api.topologySpreadConstraints` | Topology spread constraints for the API pods                                                                                                                                                                                                                                                                                                           | `[]`         |
| `api.labels`                    | Extra labels for the API Deployment                                                                                                                                                                                                                                                                                                                    | `{}`         |
| `api.annotations`               | Extra annotations for the API Deployment                                                                                                                                                                                                                                                                                                               | `{}`         |
| `api.podLabels`                 | Extra labels for API pods                                                                                                                                                                                                                                                                                                                              | `{}`         |
| `api.podAnnotations`            | Extra annotations for API pods                                                                                                                                                                                                                                                                                                                         | `{}`         |
| `api.env`                       | Additional environment variables for the API container                                                                                                                                                                                                                                                                                                 | `[]`         |
| `api.envFrom`                   | Additional environment variable sources for the API container                                                                                                                                                                                                                                                                                          | `[]`         |
| `api.nodeSelector`              | Node selector for API pods                                                                                                                                                                                                                                                                                                                             | `{}`         |
| `api.tolerations`               | Tolerations for API pods                                                                                                                                                                                                                                                                                                                               | `[]`         |
| `api.affinity`                  | Affinity rules for API pods                                                                                                                                                                                                                                                                                                                            | `{}`         |

### Controllers Parameters

Magos ships five controllers: workspace, project, rollout, variableset, and
refwatcher. All five share the same configuration shape (enabled, replicas,
labels, annotations, podLabels, podAnnotations, serviceAccount, env, envFrom,
resources, nodeSelector, tolerations, affinity, priorityClassName,
securityContext, containerSecurityContext, probes, cabundle).

The refwatcher controller has three additional fields:
- defaultPollInterval: how often to poll remote Git refs (default 30s)
- workerCount: number of concurrent polling workers (default 20)
- workQueueSize: size of the work queue (default 200)

Only controllers.workspace is annotated in full below. The other four
controllers follow an identical shape and are skipped.

| Name                                   | Description                                                                    | Value  |
| -------------------------------------- | ------------------------------------------------------------------------------ | ------ |
| `controllers.workspace.enabled`        | Deploy the workspace controller                                                | `true` |
| `controllers.workspace.replicas`       | Number of workspace controller replicas                                        | `1`    |
| `controllers.workspace.labels`         | Extra labels for the workspace controller Deployment                           | `{}`   |
| `controllers.workspace.annotations`    | Extra annotations for the workspace controller Deployment                      | `{}`   |
| `controllers.workspace.podLabels`      | Extra labels for workspace controller pods                                     | `{}`   |
| `controllers.workspace.podAnnotations` | Extra annotations for workspace controller pods                                | `{}`   |
| `controllers.workspace.env`            | Additional environment variables for the workspace controller container        | `[]`   |
| `controllers.workspace.envFrom`        | Additional environment variable sources for the workspace controller container | `[]`   |
| `controllers.workspace.nodeSelector`   | Node selector for workspace controller pods                                    | `{}`   |
| `controllers.workspace.tolerations`    | Tolerations for workspace controller pods                                      | `[]`   |
| `controllers.workspace.affinity`       | Affinity rules for workspace controller pods                                   | `{}`   |

### Job Parameters

| Name              | Description                         | Value  |
| ----------------- | ----------------------------------- | ------ |
| `job.colorOutput` | Enable color output in job pod logs | `true` |

### Workspace Parameters

| Name                       | Description                                                   | Value |
| -------------------------- | ------------------------------------------------------------- | ----- |
| `workspace.defaultPVCSize` | Default PersistentVolumeClaim size for workspace data volumes | `1Gi` |

### Postgres Parameters

Set `postgres.mode` to `embedded` (default) to deploy a PostgreSQL
StatefulSet managed by this chart, or to `external` to point at a
pre-existing PostgreSQL instance.

When using embedded mode, the chart auto-generates a random password on
first install and preserves it across upgrades. For production workloads,
set `postgres.embedded.auth.secret.name` to a Secret you manage.

| Name                             | Description                                                               | Value      |
| -------------------------------- | ------------------------------------------------------------------------- | ---------- |
| `postgres.mode`                  | Postgres deployment mode. Either `embedded` (chart-managed) or `external` | `embedded` |
| `postgres.embedded.sslMode`      | SSL mode passed to Postgres clients (disable, require, verify-full, etc.) | `disable`  |
| `postgres.embedded.nodeSelector` | Node selector for the embedded Postgres pod                               | `{}`       |
| `postgres.embedded.tolerations`  | Tolerations for the embedded Postgres pod                                 | `[]`       |
| `postgres.embedded.affinity`     | Affinity rules for the embedded Postgres pod                              | `{}`       |
| `postgres.external.host`         | Hostname or IP address of the external Postgres server                    | `""`       |
| `postgres.external.port`         | Port of the external Postgres server                                      | `5432`     |
| `postgres.external.database`     | Database name on the external Postgres server                             | `magos`    |
| `postgres.external.username`     | Username for connecting to the external Postgres server                   | `magos`    |
| `postgres.external.sslMode`      | SSL mode for external Postgres connections                                | `disable`  |

### Logs Storage Parameters

Set `logs.storage.mode` to `embedded` (default) to deploy RustFS (an
S3-compatible object store) managed by this chart, or to `external` to
point at an existing S3-compatible endpoint (AWS S3, GCS, MinIO, etc.).

When using embedded mode, the chart auto-generates random access and secret
keys on first install and preserves them across upgrades. For production
workloads, set `logs.storage.embedded.secret.name` to a Secret you manage.

| Name                                 | Description                                                                         | Value      |
| ------------------------------------ | ----------------------------------------------------------------------------------- | ---------- |
| `logs.storage.mode`                  | Log storage deployment mode. Either `embedded` (chart-managed RustFS) or `external` | `embedded` |
| `logs.storage.embedded.nodeSelector` | Node selector for the embedded RustFS pod                                           | `{}`       |
| `logs.storage.embedded.tolerations`  | Tolerations for the embedded RustFS pod                                             | `[]`       |
| `logs.storage.embedded.affinity`     | Affinity rules for the embedded RustFS pod                                          | `{}`       |
| `logs.storage.external.endpoint`     | S3-compatible endpoint URL for external log storage                                 | `""`       |

### Policy Parameters

| Name                        | Description                                                                          | Value  |
| --------------------------- | ------------------------------------------------------------------------------------ | ------ |
| `policy.kyverno.installCRD` | Install the Kyverno ValidatingPolicy CRD required for policy evaluation in plan jobs | `true` |

### Extra Objects

| Name           | Description                                                                                                           | Value |
| -------------- | --------------------------------------------------------------------------------------------------------------------- | ----- |
| `extraObjects` | List of extra Kubernetes manifests to deploy alongside the chart. Each entry can be a string (templated) or an object | `[]`  |

