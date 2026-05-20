# Helm chart maturity (Kargo-shaped rewrite)

Date: 2026-05-20
Status: Draft
Scope: charts/magos, hack/local-values.yaml, docs/getting-started/installation

## Goal

The Magos Helm chart works but is shaped like a v0 spike: flat templates folder, no Ingress, no TLS story, no `global` block for cross-cutting pod settings, no PodDisruptionBudgets, no topology spread, no `extraObjects` escape hatch, postgres and rustfs settings tangled across `auth/service/persistence/external` siblings. Akuity's Kargo chart is the reference for what "mature" looks like in this neighborhood. This spec ports those conventions onto Magos.

Three things in the user ask drive the design:

1. Match Kargo's chart structure (per-component template folders, `global` block, helpers that merge global plus component labels and annotations, cert-manager based self-signed TLS, full ingress story).
2. Make "bring your own Postgres" a first class value path. Today external mode exists but lives behind a `postgres.external.*` block while embedded settings sit at `postgres.{auth,service,persistence}`, which obscures the toggle.
3. Same for RustFS.

This is a breaking rewrite. The chart is still 0.x and there are no production installs to protect. Upgrade notes ship with the change.

## Non goals

Out of scope for this iteration:

- OIDC, Dex, admin accounts. Authentication is covered by the separate 2026-05-19 spec and is not a chart concern at this layer beyond passthrough env.
- A webhook receiver server. Magos does not have one.
- Argo CD or Argo Rollouts integration. Magos is not built on those.
- ServiceMonitor (Prometheus operator). The chart already exposes a metrics service toggle. Adding a `ServiceMonitor` ships as a follow up.
- A separate `api.ingress`. The UI's nginx already proxies `/apis/*` to the API in-cluster, so a single front-door Ingress aimed at the UI Service covers UI and API on the same host with the same cert. Users who want raw API exposure can use `extraObjects`.
- Splitting the five controllers into five separate Deployment template files. They share the `manager` binary and 95% of the spec. The single ranging template stays.

## Template layout

The current flat templates directory becomes per-component folders. Each component owns every resource it needs.

```
charts/magos/templates/
  _helpers.tpl
  NOTES.txt
  crds.yaml
  extra-manifests.yaml
  common/
    cert-issuer.yaml
  api/
    cluster-role.yaml
    cluster-role-binding.yaml
    deployment.yaml
    pdb.yaml
    service.yaml
    service-account.yaml
  ui/
    deployment.yaml
    ingress.yaml
    ingress-cert.yaml
    pdb.yaml
    service.yaml
  controller/
    cluster-role.yaml
    cluster-role-binding.yaml
    deployments.yaml
    leader-election-role.yaml
    leader-election-role-binding.yaml
    metrics-service.yaml
    service-accounts.yaml
  job/
    cluster-role.yaml
    cluster-role-binding.yaml
    service-account.yaml
  postgres/
    secret.yaml
    service.yaml
    statefulset.yaml
  rustfs/
    deployment.yaml
    pvc.yaml
    secret.yaml
    service.yaml
  kyverno/
    validatingpolicy-crd.yaml
```

The empty `templates/garage/` directory is deleted as part of the change.

`crds.yaml` keeps its current shape (renders the CRDs from `charts/magos/resources/crds/` with the optional `helm.sh/resource-policy: keep` annotation).

`extra-manifests.yaml` mirrors Kargo's `extra-manifests.yaml`: iterates `.Values.extraObjects` and renders each entry, allowing strings (templated) or objects.

## Values shape

Full reference. Comments are abridged; the implementation will carry the long form Kargo uses with `## @param` annotations so the chart README can be regenerated from values.

```yaml
image:
  repository: ghcr.io/magosproject/magos/controller
  tag: ""
  pullPolicy: IfNotPresent
  pullSecrets: []

jobImage:
  repository: ghcr.io/magosproject/magos/job
  tag: ""
  pullPolicy: IfNotPresent

nameOverride: ""
fullnameOverride: ""

global:
  labels: {}
  annotations: {}
  podLabels: {}
  podAnnotations: {}
  env: []
  envFrom: []
  nodeSelector: {}
  tolerations: []
  affinity: {}
  # priorityClassName:
  securityContext: {}
  containerSecurityContext: {}

crds:
  install: true
  keep: true

rbac:
  installClusterRoles: true
  installClusterRoleBindings: true

leaderElection:
  enabled: true

healthProbe:
  port: 8081
  livenessPath: /healthz
  readinessPath: /readyz

metricsService:
  enabled: false
  port: 8080
  type: ClusterIP
  secure: false

ui:
  enabled: true
  replicas: 1
  host: localhost
  image:
    repository: ghcr.io/magosproject/magos/ui
    tag: ""
    pullPolicy: IfNotPresent
  service:
    type: ClusterIP
    port: 80
    annotations: {}
  ingress:
    enabled: false
    annotations: {}
    ingressClassName:
    pathType: ImplementationSpecific
    tls:
      enabled: true
      selfSignedCert: true
      secretName: magos-ui-ingress-cert
  tls:
    enabled: false
    selfSignedCert: true
    secretName: magos-ui-cert
    terminatedUpstream: false
  podDisruptionBudget:
    enabled: false
    minAvailable: 1
    maxUnavailable: ""
  topologySpreadConstraints: []
  labels: {}
  annotations: {}
  podLabels: {}
  podAnnotations: {}
  serviceAccount:
    create: true
    automount: true
    annotations: {}
    labels: {}
    name: ""
  env: []
  envFrom: []
  resources:
    limits:
      cpu: 100m
      memory: 64Mi
    requests:
      cpu: 50m
      memory: 64Mi
  nodeSelector: {}
  tolerations: []
  affinity: {}
  # priorityClassName:
  securityContext: {}
  containerSecurityContext: {}
  probes:
    enabled: true

api:
  enabled: true
  replicas: 1
  image:
    repository: ghcr.io/magosproject/magos/api
    tag: ""
    pullPolicy: IfNotPresent
  service:
    type: ClusterIP
    port: 80
    annotations: {}
  podDisruptionBudget:
    enabled: false
    minAvailable: 1
    maxUnavailable: ""
  topologySpreadConstraints: []
  labels: {}
  annotations: {}
  podLabels: {}
  podAnnotations: {}
  serviceAccount:
    create: true
    automount: true
    annotations: {}
    labels: {}
    name: ""
  env: []
  envFrom: []
  resources:
    limits:
      cpu: 250m
      memory: 128Mi
    requests:
      cpu: 100m
      memory: 128Mi
  nodeSelector: {}
  tolerations: []
  affinity: {}
  # priorityClassName:
  securityContext: {}
  containerSecurityContext: {}
  probes:
    enabled: true
  cabundle:
    configMapName: ""
    secretName: ""

controllers:
  workspace:
    enabled: true
    replicas: 1
    labels: {}
    annotations: {}
    podLabels: {}
    podAnnotations: {}
    serviceAccount:
      create: true
      automount: true
      annotations: {}
      labels: {}
      name: ""
    env: []
    envFrom: []
    resources:
      limits: { cpu: 250m, memory: 128Mi }
      requests: { cpu: 100m, memory: 128Mi }
    nodeSelector: {}
    tolerations: []
    affinity: {}
    # priorityClassName:
    securityContext: {}
    containerSecurityContext: {}
    probes:
      enabled: true
    cabundle:
      configMapName: ""
      secretName: ""
  project:
    enabled: true
    replicas: 1
    # ... same shape as workspace
  rollout:
    enabled: true
    replicas: 1
    # ... same shape as workspace
  variableset:
    enabled: true
    replicas: 1
    # ... same shape as workspace
  refwatcher:
    enabled: true
    replicas: 1
    defaultPollInterval: 30s
    workerCount: 20
    workQueueSize: 200
    # ... same shape as workspace

job:
  serviceAccount:
    create: true
    name: magos-job
    automount: true
    annotations: {}
    labels: {}
  colorOutput: true
  resources:
    requests:
      cpu: 125m
      memory: 128Mi
    limits:
      cpu: 250m
      memory: 256Mi

workspace:
  defaultPVCSize: 1Gi

postgres:
  mode: embedded
  embedded:
    image:
      repository: postgres
      tag: "16-alpine"
      pullPolicy: IfNotPresent
    auth:
      database: magos
      username: magos
      secret:
        # When secret.name is empty, the chart renders a Secret named
        # "<release>-postgres" with a random password on first install and
        # preserves it across upgrades. For production set secret.name to a
        # Secret you manage yourself.
        name: ""
        passwordKey: password
    service:
      type: ClusterIP
      port: 5432
      # nodePort:
    sslMode: disable
    persistence:
      enabled: true
      size: 10Gi
      # storageClass:
    resources:
      limits: { cpu: 500m, memory: 512Mi }
      requests: { cpu: 100m, memory: 256Mi }
    podSecurityContext:
      runAsNonRoot: true
      runAsUser: 70
      runAsGroup: 70
      fsGroup: 70
      seccompProfile:
        type: RuntimeDefault
    containerSecurityContext:
      allowPrivilegeEscalation: false
      readOnlyRootFilesystem: false
      capabilities:
        drop: [ALL]
    nodeSelector: {}
    tolerations: []
    affinity: {}
    # priorityClassName:
  external:
    host: ""
    port: 5432
    database: magos
    username: magos
    secret:
      name: ""
      passwordKey: password
    sslMode: disable

logs:
  storage:
    mode: embedded
    embedded:
      image:
        repository: rustfs/rustfs
        tag: latest
        pullPolicy: IfNotPresent
      secret:
        # When secret.name is empty, the chart renders a Secret named
        # "<release>-rustfs" with random access and secret keys on first
        # install and preserves them across upgrades. For production set
        # secret.name to a Secret you manage yourself with `accessKey` and
        # `secretKey` keys (overridable via accessKeyKey / secretKeyKey).
        name: ""
        accessKeyKey: accessKey
        secretKeyKey: secretKey
      service:
        type: ClusterIP
        port: 9000
        consolePort: 9001
        # nodePort:
        # consoleNodePort:
      persistence:
        size: 10Gi
        # storageClass:
      resources:
        limits: { cpu: 250m, memory: 256Mi }
        requests: { cpu: 100m, memory: 128Mi }
      podSecurityContext:
        runAsNonRoot: true
        runAsUser: 10001
        runAsGroup: 10001
        fsGroup: 10001
        seccompProfile:
          type: RuntimeDefault
      containerSecurityContext:
        allowPrivilegeEscalation: false
        readOnlyRootFilesystem: false
        capabilities:
          drop: [ALL]
      nodeSelector: {}
      tolerations: []
      affinity: {}
      # priorityClassName:
    external:
      endpoint: ""
      secret:
        name: ""
        accessKeyKey: accessKey
        secretKeyKey: secretKey

policy:
  kyverno:
    installCRD: true

extraObjects: []
```

### Shape changes from current values, by section

`replicaCount` becomes `replicas` everywhere (UI, API, controllers). Matches Kargo, matches every other modern chart.

`podDefaults` is removed. Its members move to `global.*`. The deepCopy-then-merge pattern from Kargo (`mergeOverwrite (deepCopy .Values.global.labels) .Values.api.labels`) replaces the current `default $root.Values.podDefaults.labels` lookups.

`postgres.{auth,service,persistence,sslMode,resources,image,podSecurityContext,containerSecurityContext}` collapses into `postgres.embedded.*`. `postgres.embedded.auth.password` is removed entirely (see "Embedded vs external Postgres and RustFS" below). The `existingSecret` / `passwordKey` flat pair becomes a nested `secret: { name, passwordKey }` block under `auth` (embedded) and directly under `external`. `postgres.mode` stays at the top, defaulted to `embedded`. The helper `magos.postgresMode` continues to gate everything.

`logs.storage.{auth,service,persistence,image,resources}` collapses into `logs.storage.embedded.*`. The raw `accessKey` and `secretKey` value fields are removed (see "Embedded vs external Postgres and RustFS" below); credentials are either auto-generated by the chart or supplied through `logs.storage.embedded.secret.name`. The `existingSecret` / `accessKeyKey` / `secretKeyKey` flat triple becomes a nested `secret: { name, accessKeyKey, secretKeyKey }` block. `logs.storage.mode` stays at the top.

`rbac.create` splits into `rbac.installClusterRoles` and `rbac.installClusterRoleBindings`, the same split Kargo uses. The default for both stays true.

Each component (`api`, `ui`, `controllers.*`) gains its own `labels`, `annotations`, `podLabels`, `podAnnotations`, `nodeSelector`, `tolerations`, `affinity`, `priorityClassName`, `securityContext`, `containerSecurityContext`, `env`, `envFrom`, `topologySpreadConstraints`, `podDisruptionBudget`, `probes.enabled`. Each defaults to empty and merges with `global.*` through helpers.

The UI gains `host`, `ingress.*`, and `tls.*` settings analogous to Kargo's `api.host`, `api.ingress.*`, `api.tls.*`. The API does not get an Ingress in this iteration (single front-door design).

## Helpers (`_helpers.tpl`)

The existing helpers stay but are renamed where they overlap with new Kargo-style helpers, and several new ones are added. Final list:

Name helpers, unchanged behavior:
- `magos.name`
- `magos.fullname`
- `magos.chart`

Image helpers, new:
- `magos.image` returns `<repository>:<tag-or-AppVersion>` for the controller binary.
- `magos.jobImage` returns the same for the job binary.
- `magos.api.image`, `magos.ui.image` for per-component images.

Label helpers, new:
- `magos.labels` returns the common label block plus merged `global.labels`.
- `magos.selectorLabels` unchanged.
- `magos.api.labels`, `magos.ui.labels`, `magos.controller.labels`, `magos.postgres.labels`, `magos.rustfs.labels` each return `app.kubernetes.io/component: <component>`.

Merge helpers, new (mirroring Kargo):
- `magos.annotations` accepts `(dict "root" . "annotations" $component.annotations)` and renders the merged annotations block, emitting nothing when empty.
- Inline `mergeOverwrite (deepCopy .Values.global.podLabels) .Values.api.podLabels` is used directly in templates for podLabels and podAnnotations since those need to be combined with checksum annotations.

TLS helpers, new:
- `magos.useTLS` accepts a service dict, returns "true" if `ingress.tls.enabled` or `tls.enabled` or `tls.terminatedUpstream`. Used to decide which port the Ingress backend points at and to construct base URLs.
- `magos.ui.baseURL` builds `https://<host>` or `http://<host>` from the resolved scheme.

Service account name helpers, unchanged behavior but cleaner:
- `magos.api.serviceAccountName`
- `magos.ui.serviceAccountName`
- `magos.controller.serviceAccountName` (accepts a dict with `root`, `name`, `controller`)
- `magos.job.serviceAccountName`

Storage wiring helpers, unchanged behavior, repointed at new value paths:
- `magos.postgresMode` validates `embedded` or `external`.
- `magos.logsStorageMode` validates `embedded` or `external`.
- `magos.postgresName`, `magos.postgresHeadlessName`, `magos.postgresSecretName`, `magos.postgresPasswordKey`.
- `magos.rustfsName`, `magos.rustfsSecretName`, `magos.rustfsAccessKeyKey`, `magos.rustfsSecretKeyKey`.
- `magos.postgresEnv` renders the five `MAGOS_POSTGRES_*` env vars, dispatching on mode.
- `magos.logstoreEnv` renders the three `MAGOS_LOGS_S3_*` env vars, dispatching on mode. Picks up `region` and `bucket` when external.

GOMAXPROCS helper, new:
- `magos.selectCpuResourceField` returns `limits.cpu` or `requests.cpu` based on which is set, falling back to `limits.cpu`. Same shape as Kargo's helper. Used to set `GOMAXPROCS` from `resourceFieldRef` on controller, API, and UI pods.

Cluster wide flags helpers, new:
- `magos.controllersEnabled` returns a non-empty string when at least one controller is enabled. Used to gate the shared ClusterRole and the leader election Role.

## Ingress and TLS

One Ingress, one host. The Ingress backs the UI Service on `ui.service.port`. The UI's nginx (`ui/nginx.conf.template`) already proxies `/apis/*` to the API Service in-cluster, so external clients reach the API through the same host.

Rendering rules:

- `ui.ingress.enabled` gates the Ingress resource.
- When the Ingress is enabled, the `host` field is `ui.host` (default `localhost`). Both the rules block and the optional `tls` block use that host.
- `ui.ingress.tls.enabled` (default true) gates the Ingress `tls:` block. The secret name is `ui.ingress.tls.secretName` (default `magos-ui-ingress-cert`).
- `ui.ingress.tls.selfSignedCert` (default true) gates a `cert-manager.io/v1 Certificate` resource that issues the secret named above through the shared `magos-selfsigned-cert-issuer`. When false, the user supplies the Secret out of band.
- `ui.tls.enabled` (default false) is the terminate-TLS-on-the-pod path for installs without an Ingress controller. When true, the UI nginx config gets an additional `listen 443 ssl` server block fed from the Secret named by `ui.tls.secretName`. Same selfSignedCert toggle and same shared Issuer.
- `ui.ingress.ingressClassName` and `ui.ingress.annotations` flow through to the Ingress unchanged.
- `ui.ingress.pathType` defaults to `ImplementationSpecific`. AWS ALB users override to `Prefix`.

The shared self-signed Issuer lives at `templates/common/cert-issuer.yaml` and renders when any of `ui.ingress.tls.selfSignedCert`, `ui.tls.selfSignedCert` are true. Same pattern Kargo uses across `api.tls`, `api.ingress.tls`, `dex.tls`, `webhooksServer.tls`. The Issuer is `cert-manager.io/v1 Issuer` (namespaced, not ClusterIssuer) so the chart does not need cluster-scoped cert-manager permissions.

The chart does not declare a hard dependency on cert-manager. When all `selfSignedCert` toggles are false, no cert-manager resources are rendered and the cert-manager CRDs do not need to be present. When any toggle is true, cert-manager CRDs must be installed first or `helm install` will fail. The README documents this. NOTES.txt prints a warning if a selfSignedCert toggle is true and the user is on a fresh cluster.

The nginx config template gains a TLS variant. Today nginx.conf.template is rendered from environment variables (`MAGOS_API_HOST`, `MAGOS_API_PORT`) inside the container's entrypoint. The TLS-on-pod path adds two more environment variables (`MAGOS_TLS_CERT`, `MAGOS_TLS_KEY`) and a server block guarded by `ui.tls.enabled`. The Deployment mounts the cert Secret at `/etc/magos/tls/`.

## Embedded vs external Postgres and RustFS

This is the user's second explicit ask. The current values surface mixes embedded and external under the same prefix, which makes "BYO Postgres" feel hidden. The new shape makes the toggle obvious.

### Postgres

`postgres.mode` is the toggle. Default `embedded`. Validated by `magos.postgresMode`, which fails the install with a clear message if neither value is used.

Embedded mode renders:
- `postgres/statefulset.yaml` (one replica, single PVC, postgres 16-alpine by default).
- `postgres/service.yaml` (one ClusterIP service plus a headless service for the StatefulSet).
- `postgres/secret.yaml` only when `postgres.embedded.auth.secret.name` is empty. The chart auto-generates a random password using `randAlphaNum` and preserves it across upgrades via `lookup`. When `secret.name` is set, the chart-managed Secret is not rendered and `magos.postgresEnv` reads the password from the user-supplied Secret using `secret.passwordKey` (default `password`). The raw `password` value field is gone; the only paths to set the password are the chart's auto-generation (default install) and `secret.name` (BYO).

External mode renders nothing under `postgres/`. Instead `magos.postgresEnv` points the API and the workspace controller at `postgres.external.host:port` and pulls the password from `postgres.external.secret.name` (required, validated through `required` in the helper).

Both modes share the same five `MAGOS_POSTGRES_*` env vars (`HOST`, `PORT`, `DATABASE`, `USER`, `PASSWORD`, `SSLMODE`). The Go side does not need to know which mode is active.

### RustFS

`logs.storage.mode` is the toggle. Default `embedded`. Validated by `magos.logsStorageMode`.

Embedded mode renders:
- `rustfs/deployment.yaml`
- `rustfs/pvc.yaml`
- `rustfs/service.yaml`
- `rustfs/secret.yaml` only when `logs.storage.embedded.secret.name` is empty. The chart auto-generates random `accessKey` and `secretKey` using `randAlphaNum` and preserves them across upgrades via `lookup`. When `secret.name` is set, the chart-managed Secret is not rendered and `magos.logstoreEnv` reads credentials from the user-supplied Secret using `secret.accessKeyKey` / `secret.secretKeyKey` (defaults `accessKey` / `secretKey`). Raw `accessKey` / `secretKey` value fields are gone; the only paths to set credentials are the chart's auto-generation (default install) and `secret.name` (BYO).

External mode renders nothing under `rustfs/`. `magos.logstoreEnv` points at `logs.storage.external.endpoint` and reads access and secret keys from `logs.storage.external.secret.name` (required when external).

The bucket name (`magos-run-logs`) and region (`us-east-1`, ignored by most S3-compatible backends) are hardcoded in `internal/logstore/logstore.go` today. Exposing those at the chart layer is a separate change tracked as a follow-up; this iteration keeps the chart envelope to `endpoint`, access key, secret key.

### Validation

Both mode helpers (`magos.postgresMode`, `magos.logsStorageMode`) fail at template time if mode is anything other than `embedded` or `external`. The required-when-external bits use Helm's `required` function so the failure message names the exact field. Example:

```
postgres.external.host is required when postgres.mode=external
postgres.external.secret.name is required when postgres.mode=external
logs.storage.external.endpoint is required when logs.storage.mode=external
logs.storage.external.secret.name is required when logs.storage.mode=external
```

## Cross cutting Kargo conventions

The following Kargo patterns are adopted verbatim, ported to Magos names. They are listed here so they are not forgotten during implementation.

### Pod template annotations

Each Deployment and StatefulSet template renders two checksum annotations on the pod template:

```yaml
annotations:
  configmap/checksum: {{ pick (include (print $.Template.BasePath "/api/configmap.yaml") . | fromYaml) "data" | toYaml | sha256sum }}
  secret/checksum: {{ pick (include (print $.Template.BasePath "/api/secret.yaml") . | fromYaml) "stringData" | toYaml | sha256sum }}
```

Magos does not have a per-component ConfigMap or Secret today. The API picks up its config through env. Until that changes (the OIDC spec adds an `api/secret.yaml`), the checksum annotations are emitted only when the referenced file exists. Implementation: the API template includes the checksum block guarded by an `if` over the OIDC values.

### Topology spread

`topologySpreadConstraints` is a top-level field on `ui`, `api`, and each entry of `controllers`. Renders only when non-empty. No defaults. Example in values comments.

### Priority class

`priorityClassName` is on the same components. Falls back to `global.priorityClassName` through `default`. Renders only when non-empty.

### Pod disruption budget

`api.podDisruptionBudget` and `ui.podDisruptionBudget`. Controller PDBs are skipped (leader election handles availability differently and PDBs over single-replica workloads do not help). The PDB template renders `policy/v1 PodDisruptionBudget` with either `minAvailable` or `maxUnavailable`, the same alternation Kargo uses.

### Probes toggle

`api.probes.enabled`, `ui.probes.enabled`, `controllers.<name>.probes.enabled`. Default true. When false, the liveness and readiness blocks are skipped. Useful during local development.

### CA bundle support

`api.cabundle.configMapName` / `api.cabundle.secretName` and the same keys on each controller. When set, the deployment template adds:

1. A `parse-cabundle` init container that splits the multi-cert PEM into individual files and runs `update-ca-certificates`. Exact copy of Kargo's init container.
2. A `cabundle` projected volume (Secret or ConfigMap, secret wins if both set).
3. A `certs` emptyDir mounted at `/etc/ssl/certs` in both the init container and the main container.

The chart README documents the use case: Git over HTTPS against a private CA, OCI registries with private CAs.

### Extra manifests

`extraObjects: []` at the top level. Each entry is either a YAML object or a string. The template renders each:

```yaml
{{ range .Values.extraObjects }}
---
{{- if kindIs "string" . }}
{{ tpl . $ }}
{{- else }}
{{ tpl (toYaml .) $ }}
{{- end }}
{{ end }}
```

Same shape as Kargo's `extra-manifests.yaml`.

### NOTES.txt

The chart prints a NOTES.txt that:

1. Reports which components are enabled.
2. Prints the front-door URL when `ui.ingress.enabled` is true, computed through `magos.ui.baseURL`.
3. Prints a port-forward example when the Ingress is disabled.
4. Reports the mode for postgres and logs storage.
5. Warns about cert-manager CRDs when a selfSignedCert toggle is on.

### Pull policy and pull secrets

Image pull policy is exposed per image (`image.pullPolicy`, `jobImage.pullPolicy`, `ui.image.pullPolicy`, `api.image.pullPolicy`, `postgres.embedded.image.pullPolicy`, `logs.storage.embedded.image.pullPolicy`). Pull secrets are exposed once at `image.pullSecrets` and flow into every pod that the chart renders. Per-component pull secret overrides are not added in this iteration. Yagni unless someone asks.

## Migration from current values

Breaking value path changes. Documented in NOTES.txt and the installation docs page.

| Old path | New path |
| --- | --- |
| `ui.replicaCount` | `ui.replicas` |
| `api.replicaCount` | `api.replicas` |
| `controllers.<n>.replicaCount` | `controllers.<n>.replicas` |
| `podDefaults.annotations` | `global.podAnnotations` |
| `podDefaults.labels` | `global.podLabels` |
| `podDefaults.securityContext` | `global.securityContext` |
| `podDefaults.containerSecurityContext` | `global.containerSecurityContext` |
| `podDefaults.resources` | per-component `resources` (no global fallback for resources) |
| `podDefaults.nodeSelector` | `global.nodeSelector` |
| `podDefaults.tolerations` | `global.tolerations` |
| `podDefaults.affinity` | `global.affinity` |
| `podDefaults.env` | `global.env` |
| `rbac.create` | `rbac.installClusterRoles` and `rbac.installClusterRoleBindings` |
| `postgres.auth.{database,username}` | `postgres.embedded.auth.{database,username}` |
| `postgres.auth.existingSecret` | `postgres.embedded.auth.secret.name` |
| `postgres.auth.passwordKey` | `postgres.embedded.auth.secret.passwordKey` |
| `postgres.auth.password` | removed; chart auto-generates and preserves via `lookup`, or set `postgres.embedded.auth.secret.name` |
| `postgres.image` | `postgres.embedded.image` |
| `postgres.service` | `postgres.embedded.service` |
| `postgres.persistence` | `postgres.embedded.persistence` |
| `postgres.sslMode` | `postgres.embedded.sslMode` |
| `postgres.resources` | `postgres.embedded.resources` |
| `postgres.podSecurityContext` | `postgres.embedded.podSecurityContext` |
| `postgres.containerSecurityContext` | `postgres.embedded.containerSecurityContext` |
| `logs.storage.image` | `logs.storage.embedded.image` |
| `logs.storage.auth.{accessKey,secretKey}` | removed; chart auto-generates and preserves via `lookup`, or set `logs.storage.embedded.secret.name` |
| `logs.storage.external.existingSecret` | `logs.storage.external.secret.name` |
| `logs.storage.external.{accessKeyKey,secretKeyKey}` | `logs.storage.external.secret.{accessKeyKey,secretKeyKey}` |
| `logs.storage.service` | `logs.storage.embedded.service` |
| `logs.storage.persistence` | `logs.storage.embedded.persistence` |
| `logs.storage.resources` | `logs.storage.embedded.resources` |
| `jobServiceAccount.*` | `job.serviceAccount.*` |

`hack/local-values.yaml` is updated in the same PR to reflect the new paths. The current file sets `logs.storage.auth.{accessKey,secretKey}` and `postgres.auth.password` directly so `make run` can extract known credentials from the in-cluster Secrets. Those overrides are replaced with a pre-created Secret manifest committed under `hack/` (e.g. `hack/dev-postgres-secret.yaml`, `hack/dev-rustfs-secret.yaml`) that the dev install applies before `helm install`. The Helm values then point at those Secrets via `existingSecret`. This keeps `make run` deterministic without reintroducing raw credential value paths into the chart.

## Testing

Two test layers, both in tree.

`test/chainsaw/tests/chart/` is the existing chart install test directory. The new tests added there:

1. `chart-default-install`: install with defaults, assert the embedded Postgres StatefulSet, embedded RustFS Deployment, API Deployment, UI Deployment, and five controller Deployments come up. Assert no Ingress.
2. `chart-ingress-self-signed`: install with `ui.ingress.enabled=true` and `ui.ingress.tls.selfSignedCert=true`. Assert the Ingress, the cert-manager Issuer, the Certificate, and the chart-generated Secret all render. Requires cert-manager in the test cluster.
3. `chart-ingress-byo-secret`: install with `ui.ingress.enabled=true`, `ui.ingress.tls.selfSignedCert=false`, and a pre-created Secret. Assert the Ingress points at the user Secret and no cert-manager resources are rendered.
4. `chart-postgres-external`: install with `postgres.mode=external`, `postgres.external.host=test-postgres.default`, `postgres.external.existingSecret=test-pg-secret`. Assert no embedded Postgres StatefulSet renders and the API Deployment has the env pointed at the external host. Requires a fake Postgres deployment in the test cluster as the chainsaw test target.
5. `chart-logs-external`: install with `logs.storage.mode=external` against a MinIO sidecar deployed in the test cluster.
6. `chart-extra-objects`: install with `extraObjects:` containing one ConfigMap and one templated string. Assert both render.

Each test exists as a chainsaw test under `test/chainsaw/tests/chart/<name>/`. They run under `make test-chainsaw-chart`.

Helm unit tests live next to the chart in `charts/magos/tests/` as `helm/chart-testing` would discover them. Two are added:

1. `lint`: `helm lint charts/magos --strict` runs in CI.
2. `template-default`: `helm template magos charts/magos` succeeds and produces a non-empty output. Done as a chainsaw `chart-render` test if helm/chart-testing is not yet wired up.

## Documentation

`docs/getting-started/installation/index.mdx` gets a rewrite to describe:

1. The new value shape, with two short example values blocks (defaults install vs production install with external Postgres, external S3, and Ingress with self signed cert).
2. The cert-manager prerequisite for the self-signed path.
3. The `mode: embedded | external` toggle.
4. The single front-door URL story (UI proxies API at `/apis/`).
5. A short upgrade-from-0.1.x section listing the renamed paths.

The chart's own README is generated from the `## @param` annotations in values.yaml, the same way Kargo's chart README is generated. Tooling: `helm-docs` (or `readme-generator-for-helm`, both work). Picked in implementation, not now.

## Risk

The breaking value path changes mean every existing user has to rewrite their `values.yaml`. Mitigations:

1. A `templates/_validate.tpl` partial runs at the start of rendering and uses `fail` with a pointer to the migration table when any deprecated key is present. The partial probes the old paths directly through `.Values` (Helm merges user-supplied values into `.Values`, so the deprecated key only appears when the user set it). Example: `{{ if hasKey .Values "podDefaults" }}{{ fail "podDefaults has been removed in favor of global. See: ..." }}{{ end }}`. The full list covers `podDefaults`, `rbac.create`, the old `postgres.*` flat keys, the old `logs.storage.*` flat keys, and `replicaCount` fields. The partial is included from `templates/_helpers.tpl` so every rendered file triggers it once.
2. The installation docs page carries the migration table and a one shot `yq` script that rewrites a values file from the old shape to the new one.

cert-manager dependency for the self-signed path. The chart does not pull in cert-manager. Users without it must either provide their own Secret (`selfSignedCert: false`) or install cert-manager first. The README and NOTES.txt say this.

The single-host Ingress means external API consumers reach the API through the UI's nginx. nginx has been holding up SSE and WebSocket traffic for the live console for months (`feat(nginx): support HTTP 2.0`, commit 6e57c82), so this is well exercised. If a future CLI needs direct API access, an opt-in `api.ingress` can be added without disturbing the front-door design.

## Order of work

Implementation breaks into roughly these batches, in order. Each is a separate PR.

1. Helpers, label and annotation merge, image helpers, mode helpers refactor. No template moves. Lands first to give the rest a foundation.
2. Template folder restructure. Mechanical move plus updates to include paths. No behavior change.
3. Values shape rewrite (`global` block, `replicas`, `postgres.{embedded,external}`, `logs.storage.{embedded,external}`, per-component env/labels/etc.). All templates updated to read from the new paths. Chainsaw tests updated.
4. Ingress, cert-manager Issuer, ingress-cert.yaml. New cross-cutting feature.
5. UI on-pod TLS variant of the nginx config (`ui.tls.enabled` path).
6. PDB, topology spread, priorityClassName, probes toggle, cabundle.
7. NOTES.txt, extraObjects, deprecation probes, README annotations.
8. Installation docs rewrite. Migration table.

Each batch has its own chainsaw coverage where applicable.
