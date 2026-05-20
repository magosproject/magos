# Helm chart maturity (Kargo-shaped rewrite) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restructure `charts/magos` along Akuity Kargo's chart conventions: per-component template folders, `global` block, label/annotation merge helpers, ingress with cert-manager based TLS, embedded vs external Postgres and RustFS surfaced through `mode` toggles, raw credential value paths replaced with `secret.name` BYO, plus PodDisruptionBudget, topology spread, priorityClassName, probes toggle, cabundle, extraObjects, NOTES.txt, and a rewritten installation docs page.

**Architecture:** Eight ordered batches (helpers, template moves, values rewrite, ingress, on-pod TLS, cross-cutting Kargo-isms, NOTES + docs polish, installation docs). Each batch is a separate logical commit checkpoint. Chart is still 0.x: the rewrite is intentionally breaking and uses a `_validate.tpl` partial that hard-fails the install when any deprecated value path is present, pointing the user at the migration table in NOTES.txt.

**Tech Stack:** Helm v3, cert-manager.io/v1 (optional, only when `selfSignedCert: true` anywhere), chainsaw for chart install tests, helm-docs for README generation, `make test-chainsaw-chart` as the install test gate. No new Go code is introduced by this plan.

**Conventions for this plan:**

- The user drives git. Tasks list a "Commit checkpoint" marker showing the intended commit boundary, but the plan never runs `git` itself. The executor stages and commits at those markers.
- Every change to a template is validated by `helm lint charts/magos --strict` and `helm template magos charts/magos -f hack/local-values.yaml > /tmp/render.yaml` before the task is considered done.
- Install assertions live as chainsaw tests under `test/chainsaw/tests/chart/<name>/chainsaw-test.yaml`. They are exercised by `make test-chainsaw-chart`, which expects the chart already installed in `magos-system`.
- Spec reference: `docs/superpowers/specs/2026-05-20-chart-maturity-design.md`.

---

## Batch 1: Helpers (`_helpers.tpl`)

The foundation. New helpers land first so later tasks can use them. No template-file moves, no values shape change yet.

### Task 1: Image helpers

**Files:**
- Modify: `charts/magos/templates/_helpers.tpl`

- [ ] **Step 1: Add four image helpers to `_helpers.tpl`**

Append after the existing `magos.chart` helper:

```gotemplate
{{/*
Image reference helpers. Each returns "<repository>:<tag-or-AppVersion>".
The per-component helpers fall back to the chart AppVersion when the
component image.tag is empty, the same way Kargo's chart does.
*/}}
{{- define "magos.image" -}}
{{- $tag := default .Chart.AppVersion .Values.image.tag -}}
{{- printf "%s:%s" .Values.image.repository $tag -}}
{{- end -}}

{{- define "magos.jobImage" -}}
{{- $tag := default .Chart.AppVersion .Values.jobImage.tag -}}
{{- printf "%s:%s" .Values.jobImage.repository $tag -}}
{{- end -}}

{{- define "magos.api.image" -}}
{{- $tag := default .Chart.AppVersion .Values.api.image.tag -}}
{{- printf "%s:%s" .Values.api.image.repository $tag -}}
{{- end -}}

{{- define "magos.ui.image" -}}
{{- $tag := default .Chart.AppVersion .Values.ui.image.tag -}}
{{- printf "%s:%s" .Values.ui.image.repository $tag -}}
{{- end -}}
```

- [ ] **Step 2: Verify the helpers render**

Run: `helm template magos charts/magos -f hack/local-values.yaml > /tmp/render.yaml && echo OK`
Expected: `OK` (helpers parse, even though they are not used yet).

- [ ] **Step 3: Verify lint passes**

Run: `helm lint charts/magos --strict`
Expected: `1 chart(s) linted, 0 chart(s) failed`.

### Task 2: Per-component label helpers

**Files:**
- Modify: `charts/magos/templates/_helpers.tpl`

- [ ] **Step 1: Append per-component label helpers**

Add after the existing `magos.selectorLabels`:

```gotemplate
{{- define "magos.api.labels" -}}
app.kubernetes.io/component: api
{{- end -}}

{{- define "magos.ui.labels" -}}
app.kubernetes.io/component: ui
{{- end -}}

{{- define "magos.controller.labels" -}}
app.kubernetes.io/component: controller
{{- end -}}

{{- define "magos.postgres.labels" -}}
app.kubernetes.io/component: postgres
{{- end -}}

{{- define "magos.rustfs.labels" -}}
app.kubernetes.io/component: rustfs
{{- end -}}

{{- define "magos.job.labels" -}}
app.kubernetes.io/component: job
{{- end -}}
```

- [ ] **Step 2: Verify lint and render still pass**

Run: `helm lint charts/magos --strict && helm template magos charts/magos -f hack/local-values.yaml > /dev/null`
Expected: lint passes, template renders.

### Task 3: Annotation merge helper

**Files:**
- Modify: `charts/magos/templates/_helpers.tpl`

- [ ] **Step 1: Add `magos.annotations`**

Append:

```gotemplate
{{/*
Render a complete metadata annotations block by merging
.Values.global.annotations with a per-component annotations map.

Call:
  {{- include "magos.annotations" (dict "root" . "annotations" .Values.api.annotations) | nindent 2 }}

When the merged result is empty the helper emits nothing.

For resources with no component-specific annotations, omit the
"annotations" key or pass an empty dict.
*/}}
{{- define "magos.annotations" -}}
{{- with (mergeOverwrite (deepCopy (default dict .root.Values.global.annotations)) (.annotations | default dict)) -}}
annotations:
  {{- range $key, $value := . }}
  {{ $key }}: {{ $value | quote }}
  {{- end }}
{{- end }}
{{- end -}}
```

- [ ] **Step 2: Verify lint and render**

Run: `helm lint charts/magos --strict && helm template magos charts/magos -f hack/local-values.yaml > /dev/null`
Expected: pass.

### Task 4: TLS and base URL helpers

**Files:**
- Modify: `charts/magos/templates/_helpers.tpl`

- [ ] **Step 1: Add `magos.useTLS` and `magos.ui.baseURL`**

Append:

```gotemplate
{{/*
Returns "true" when the given service dict resolves to HTTPS.
The service dict is expected to look like .Values.ui or .Values.api,
carrying optional ingress.{enabled,tls.enabled} and tls.{enabled,terminatedUpstream}.
*/}}
{{- define "magos.useTLS" -}}
{{- $service := . -}}
{{- $useIngressTLS := and (default false $service.ingress.enabled) (default false $service.ingress.tls.enabled) -}}
{{- $useDirectTLS := and (not (default false $service.ingress.enabled)) (default false $service.tls.enabled) -}}
{{- $terminatedUpstream := default false $service.tls.terminatedUpstream -}}
{{- if or $useIngressTLS $useDirectTLS $terminatedUpstream -}}true{{- end -}}
{{- end -}}

{{/*
Returns the base URL ("http://host" or "https://host") for a service
described by .service plus a .host string.
*/}}
{{- define "magos.baseURL" -}}
{{- $service := .service -}}
{{- $host := .host -}}
{{- if eq (include "magos.useTLS" $service) "true" -}}
https://{{ $host }}
{{- else -}}
http://{{ $host }}
{{- end -}}
{{- end -}}

{{- define "magos.ui.baseURL" -}}
{{- include "magos.baseURL" (dict "service" .Values.ui "host" .Values.ui.host) -}}
{{- end -}}
```

- [ ] **Step 2: Verify lint and render**

Run: `helm lint charts/magos --strict && helm template magos charts/magos -f hack/local-values.yaml > /dev/null`
Expected: pass. The helpers are defined but not yet referenced.

### Task 5: GOMAXPROCS source-field helper

**Files:**
- Modify: `charts/magos/templates/_helpers.tpl`

- [ ] **Step 1: Add `magos.selectCpuResourceField`**

Append:

```gotemplate
{{/*
Returns "limits.cpu" or "requests.cpu" based on which is set in the given
resources dict. Falls back to "limits.cpu". Used as the resourceFieldRef
divisor source for GOMAXPROCS.

Call:
  {{ include "magos.selectCpuResourceField" (dict "resources" .Values.api.resources) }}
*/}}
{{- define "magos.selectCpuResourceField" -}}
{{- $resources := .resources -}}
{{- $field := "limits.cpu" -}}
{{- if $resources -}}
{{- if and $resources.limits $resources.limits.cpu -}}
{{- $field = "limits.cpu" -}}
{{- else if and $resources.requests $resources.requests.cpu -}}
{{- $field = "requests.cpu" -}}
{{- end -}}
{{- end -}}
{{- $field -}}
{{- end -}}
```

- [ ] **Step 2: Verify lint and render**

Run: `helm lint charts/magos --strict && helm template magos charts/magos -f hack/local-values.yaml > /dev/null`
Expected: pass.

### Task 6: Deprecated-path validation partial

**Files:**
- Create: `charts/magos/templates/_validate.tpl`
- Modify: `charts/magos/templates/_helpers.tpl`

This partial is added now and stays empty until Batch 3 introduces the renamed paths. The stub gives later tasks a known include point.

- [ ] **Step 1: Create `_validate.tpl`**

Write `charts/magos/templates/_validate.tpl`:

```gotemplate
{{/*
Hard-fails the install when any deprecated value path is set.
Populated in Batch 3 (values shape rewrite); empty for now.

The partial is included from every top-level template via "magos.validate".
*/}}
{{- define "magos.validate" -}}
{{- end -}}
```

- [ ] **Step 2: Confirm `_helpers.tpl` does not need updates yet**

The include of `magos.validate` will be wired into individual top-level templates in Batch 3. No change to `_helpers.tpl` in this task.

- [ ] **Step 3: Verify lint and render**

Run: `helm lint charts/magos --strict && helm template magos charts/magos -f hack/local-values.yaml > /dev/null`
Expected: pass.

### Task 7: Batch 1 commit checkpoint

- [ ] **Step 1: Render the chart with default values and stash the output**

Run: `helm template magos charts/magos > /tmp/render-batch1.yaml`
Expected: file written, non-empty.

- [ ] **Step 2: Commit checkpoint**

Stage `charts/magos/templates/_helpers.tpl` and `charts/magos/templates/_validate.tpl`.
Suggested message: `chart: add image, label, annotation, tls, gomaxprocs helpers`.

The chart's behavior is unchanged (no template references the new helpers yet). Output of `helm template` is identical to before this batch.

---

## Batch 2: Template folder restructure

Mechanical move of templates into per-component folders. No behavior change. Each move is `git mv` so history follows.

### Task 8: Move templates into per-component folders

**Files:**
- Create: `charts/magos/templates/api/`
- Create: `charts/magos/templates/ui/`
- Create: `charts/magos/templates/controller/`
- Create: `charts/magos/templates/job/`
- Create: `charts/magos/templates/postgres/`
- Create: `charts/magos/templates/rustfs/`
- Create: `charts/magos/templates/kyverno/`
- Create: `charts/magos/templates/common/` (empty placeholder for Batch 4 cert-issuer)
- Move: every existing yaml under `charts/magos/templates/` into the corresponding subfolder
- Delete: `charts/magos/templates/garage/` (empty leftover)

- [ ] **Step 1: Create the new folders**

Run:

```
mkdir -p charts/magos/templates/api \
         charts/magos/templates/ui \
         charts/magos/templates/controller \
         charts/magos/templates/job \
         charts/magos/templates/postgres \
         charts/magos/templates/rustfs \
         charts/magos/templates/kyverno \
         charts/magos/templates/common
```

- [ ] **Step 2: Move API templates**

```
git mv charts/magos/templates/api-clusterrole.yaml         charts/magos/templates/api/cluster-role.yaml
git mv charts/magos/templates/api-clusterrolebinding.yaml  charts/magos/templates/api/cluster-role-binding.yaml
git mv charts/magos/templates/api-deployment.yaml          charts/magos/templates/api/deployment.yaml
git mv charts/magos/templates/api-service.yaml             charts/magos/templates/api/service.yaml
git mv charts/magos/templates/api-serviceaccount.yaml      charts/magos/templates/api/service-account.yaml
```

- [ ] **Step 3: Move UI templates**

```
git mv charts/magos/templates/ui-deployment.yaml charts/magos/templates/ui/deployment.yaml
git mv charts/magos/templates/ui-service.yaml    charts/magos/templates/ui/service.yaml
```

- [ ] **Step 4: Move controller templates**

```
git mv charts/magos/templates/controllers-clusterrole.yaml                  charts/magos/templates/controller/cluster-role.yaml
git mv charts/magos/templates/controllers-clusterrolebinding.yaml           charts/magos/templates/controller/cluster-role-binding.yaml
git mv charts/magos/templates/controllers-deployments.yaml                  charts/magos/templates/controller/deployments.yaml
git mv charts/magos/templates/controllers-leader-election-role.yaml         charts/magos/templates/controller/leader-election-role.yaml
git mv charts/magos/templates/controllers-leader-election-rolebinding.yaml  charts/magos/templates/controller/leader-election-role-binding.yaml
git mv charts/magos/templates/controllers-metrics-services.yaml             charts/magos/templates/controller/metrics-service.yaml
git mv charts/magos/templates/controllers-serviceaccounts.yaml              charts/magos/templates/controller/service-accounts.yaml
```

- [ ] **Step 5: Move job templates**

```
git mv charts/magos/templates/job-clusterrole.yaml        charts/magos/templates/job/cluster-role.yaml
git mv charts/magos/templates/job-clusterrolebinding.yaml charts/magos/templates/job/cluster-role-binding.yaml
git mv charts/magos/templates/job-serviceaccount.yaml     charts/magos/templates/job/service-account.yaml
```

- [ ] **Step 6: Move Postgres templates**

```
git mv charts/magos/templates/postgres-secret.yaml      charts/magos/templates/postgres/secret.yaml
git mv charts/magos/templates/postgres-services.yaml    charts/magos/templates/postgres/service.yaml
git mv charts/magos/templates/postgres-statefulset.yaml charts/magos/templates/postgres/statefulset.yaml
```

- [ ] **Step 7: Move RustFS templates**

```
git mv charts/magos/templates/rustfs-deployment.yaml charts/magos/templates/rustfs/deployment.yaml
git mv charts/magos/templates/rustfs-pvc.yaml        charts/magos/templates/rustfs/pvc.yaml
git mv charts/magos/templates/rustfs-secret.yaml     charts/magos/templates/rustfs/secret.yaml
git mv charts/magos/templates/rustfs-service.yaml    charts/magos/templates/rustfs/service.yaml
```

- [ ] **Step 8: Move Kyverno ValidatingPolicy CRD template**

```
git mv charts/magos/templates/kyverno-validatingpolicy-crd.yaml charts/magos/templates/kyverno/validatingpolicy-crd.yaml
```

- [ ] **Step 9: Delete the empty `garage/` directory**

```
rmdir charts/magos/templates/garage
```

(`rmdir` fails noisily if anything sneaked in; that is intentional.)

- [ ] **Step 10: Compare rendered output before and after the move**

Run: `helm template magos charts/magos > /tmp/render-batch2.yaml && diff -u /tmp/render-batch1.yaml /tmp/render-batch2.yaml`
Expected: empty diff. The move is mechanical; Helm walks `templates/` recursively, so output is unchanged.

If diff is non-empty, the most likely cause is a stray template not moved. Find it with `ls charts/magos/templates/*.yaml` and complete the missed move.

- [ ] **Step 11: Lint**

Run: `helm lint charts/magos --strict`
Expected: pass.

- [ ] **Step 12: Commit checkpoint**

Stage all moves and the empty `common/` directory creation.
Suggested message: `chart: move templates into per-component folders`.

---

## Batch 3: Values shape rewrite

This is the biggest batch. New `values.yaml`, updated templates, updated helpers for storage wiring, new dev secrets, validation partial populated, three new chainsaw tests.

### Task 9: Write the new `values.yaml`

**Files:**
- Modify: `charts/magos/values.yaml`

- [ ] **Step 1: Replace `charts/magos/values.yaml` end-to-end**

Use the values shape from the spec verbatim (Section "Values shape" of `docs/superpowers/specs/2026-05-20-chart-maturity-design.md`). The full file is:

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
  project:
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
  rollout:
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
  variableset:
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
  refwatcher:
    enabled: true
    replicas: 1
    defaultPollInterval: 30s
    workerCount: 20
    workQueueSize: 200
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
      limits:
        cpu: 500m
        memory: 512Mi
      requests:
        cpu: 100m
        memory: 256Mi
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
        # secret.name to a Secret you manage yourself.
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
        limits:
          cpu: 250m
          memory: 256Mi
        requests:
          cpu: 100m
          memory: 128Mi
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

- [ ] **Step 2: Lint will fail at this point**

Run: `helm lint charts/magos --strict`
Expected: lint FAILS because templates still read from old paths (`replicaCount`, `podDefaults`, `postgres.auth.*`, etc.). That is expected; subsequent tasks in this batch fix it.

### Task 10: Update storage-wiring helpers for new value paths

**Files:**
- Modify: `charts/magos/templates/_helpers.tpl`

- [ ] **Step 1: Replace mode and name helpers in `_helpers.tpl`**

Find this block in `_helpers.tpl`:

```gotemplate
{{- define "magos.postgresMode" -}}
{{- $mode := default "embedded" .Values.postgres.mode -}}
...
{{- end }}
```

Replace it (and `magos.logsStorageMode`, `magos.postgresName`, `magos.postgresHeadlessName`, `magos.postgresSecretName`, `magos.postgresPasswordKey`, `magos.rustfsName`, `magos.rustfsSecretName`, `magos.rustfsAccessKeyKey`, `magos.rustfsSecretKeyKey`, `magos.postgresEnv`, `magos.logstoreEnv`) with:

```gotemplate
{{/*
Resolve and validate the postgres mode.
*/}}
{{- define "magos.postgresMode" -}}
{{- $mode := default "embedded" .Values.postgres.mode -}}
{{- if and (ne $mode "embedded") (ne $mode "external") -}}
{{- fail (printf "postgres.mode must be either embedded or external, got %q" $mode) -}}
{{- end -}}
{{- $mode -}}
{{- end -}}

{{/*
Resolve and validate the logs storage mode.
*/}}
{{- define "magos.logsStorageMode" -}}
{{- $mode := default "embedded" .Values.logs.storage.mode -}}
{{- if and (ne $mode "embedded") (ne $mode "external") -}}
{{- fail (printf "logs.storage.mode must be either embedded or external, got %q" $mode) -}}
{{- end -}}
{{- $mode -}}
{{- end -}}

{{- define "magos.postgresName" -}}
{{- printf "%s-postgres" (include "magos.fullname" .) -}}
{{- end -}}

{{- define "magos.postgresHeadlessName" -}}
{{- printf "%s-headless" (include "magos.postgresName" .) -}}
{{- end -}}

{{/*
Returns the Secret name that holds the postgres password. In embedded mode
this is either the user-supplied embedded.auth.secret.name or, when empty,
the chart-managed "<release>-postgres" Secret. In external mode the
external.secret.name field is required.
*/}}
{{- define "magos.postgresSecretName" -}}
{{- if eq (include "magos.postgresMode" .) "external" -}}
{{- required "postgres.external.secret.name is required when postgres.mode=external" .Values.postgres.external.secret.name -}}
{{- else -}}
{{- default (include "magos.postgresName" .) .Values.postgres.embedded.auth.secret.name -}}
{{- end -}}
{{- end -}}

{{- define "magos.postgresPasswordKey" -}}
{{- if eq (include "magos.postgresMode" .) "external" -}}
{{- default "password" .Values.postgres.external.secret.passwordKey -}}
{{- else -}}
{{- default "password" .Values.postgres.embedded.auth.secret.passwordKey -}}
{{- end -}}
{{- end -}}

{{- define "magos.rustfsName" -}}
{{- printf "%s-rustfs" (include "magos.fullname" .) -}}
{{- end -}}

{{- define "magos.rustfsSecretName" -}}
{{- if eq (include "magos.logsStorageMode" .) "external" -}}
{{- required "logs.storage.external.secret.name is required when logs.storage.mode=external" .Values.logs.storage.external.secret.name -}}
{{- else -}}
{{- default (include "magos.rustfsName" .) .Values.logs.storage.embedded.secret.name -}}
{{- end -}}
{{- end -}}

{{- define "magos.rustfsAccessKeyKey" -}}
{{- if eq (include "magos.logsStorageMode" .) "external" -}}
{{- default "accessKey" .Values.logs.storage.external.secret.accessKeyKey -}}
{{- else -}}
{{- default "accessKey" .Values.logs.storage.embedded.secret.accessKeyKey -}}
{{- end -}}
{{- end -}}

{{- define "magos.rustfsSecretKeyKey" -}}
{{- if eq (include "magos.logsStorageMode" .) "external" -}}
{{- default "secretKey" .Values.logs.storage.external.secret.secretKeyKey -}}
{{- else -}}
{{- default "secretKey" .Values.logs.storage.embedded.secret.secretKeyKey -}}
{{- end -}}
{{- end -}}

{{/*
Environment variables for the run-summary database.
*/}}
{{- define "magos.postgresEnv" -}}
{{- if eq (include "magos.postgresMode" .) "external" }}
- name: MAGOS_POSTGRES_HOST
  value: {{ required "postgres.external.host is required when postgres.mode=external" .Values.postgres.external.host | quote }}
- name: MAGOS_POSTGRES_PORT
  value: {{ .Values.postgres.external.port | quote }}
- name: MAGOS_POSTGRES_DATABASE
  value: {{ .Values.postgres.external.database | quote }}
- name: MAGOS_POSTGRES_USER
  value: {{ .Values.postgres.external.username | quote }}
- name: MAGOS_POSTGRES_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ include "magos.postgresSecretName" . }}
      key: {{ include "magos.postgresPasswordKey" . }}
- name: MAGOS_POSTGRES_SSLMODE
  value: {{ .Values.postgres.external.sslMode | quote }}
{{- else }}
- name: MAGOS_POSTGRES_HOST
  value: {{ include "magos.postgresName" . | quote }}
- name: MAGOS_POSTGRES_PORT
  value: {{ .Values.postgres.embedded.service.port | quote }}
- name: MAGOS_POSTGRES_DATABASE
  value: {{ .Values.postgres.embedded.auth.database | quote }}
- name: MAGOS_POSTGRES_USER
  value: {{ .Values.postgres.embedded.auth.username | quote }}
- name: MAGOS_POSTGRES_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ include "magos.postgresSecretName" . }}
      key: {{ include "magos.postgresPasswordKey" . }}
- name: MAGOS_POSTGRES_SSLMODE
  value: {{ .Values.postgres.embedded.sslMode | quote }}
{{- end }}
{{- end -}}

{{/*
Environment variables for the log store.
*/}}
{{- define "magos.logstoreEnv" -}}
{{- if eq (include "magos.logsStorageMode" .) "external" }}
- name: MAGOS_LOGS_S3_ENDPOINT
  value: {{ required "logs.storage.external.endpoint is required when logs.storage.mode=external" .Values.logs.storage.external.endpoint | quote }}
{{- else }}
- name: MAGOS_LOGS_S3_ENDPOINT
  value: {{ printf "http://%s:%v" (include "magos.rustfsName" .) .Values.logs.storage.embedded.service.port | quote }}
{{- end }}
- name: MAGOS_LOGS_S3_ACCESS_KEY_ID
  valueFrom:
    secretKeyRef:
      name: {{ include "magos.rustfsSecretName" . }}
      key: {{ include "magos.rustfsAccessKeyKey" . }}
- name: MAGOS_LOGS_S3_SECRET_ACCESS_KEY
  valueFrom:
    secretKeyRef:
      name: {{ include "magos.rustfsSecretName" . }}
      key: {{ include "magos.rustfsSecretKeyKey" . }}
{{- end -}}
```

- [ ] **Step 2: Add the `magos.controllersEnabled` helper if not present**

The existing `magos.controllersEnabled` already does what we need. Verify it is unchanged.

- [ ] **Step 3: Lint will still fail for now**

Run: `helm lint charts/magos --strict`
Expected: still fails because templates have not been updated to read from `global.*`. That is expected; continue.

### Task 11: Update Postgres templates for new value paths and `secret.name`

**Files:**
- Modify: `charts/magos/templates/postgres/statefulset.yaml`
- Modify: `charts/magos/templates/postgres/service.yaml`
- Modify: `charts/magos/templates/postgres/secret.yaml`

- [ ] **Step 1: Update `postgres/statefulset.yaml`**

Replace the file with:

```yaml
{{- if eq (include "magos.postgresMode" .) "embedded" }}
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: {{ include "magos.postgresName" . }}
  labels:
    {{- include "magos.labels" . | nindent 4 }}
    {{- include "magos.postgres.labels" . | nindent 4 }}
  {{- include "magos.annotations" (dict "root" . "annotations" dict) | nindent 2 }}
spec:
  serviceName: {{ include "magos.postgresHeadlessName" . }}
  replicas: 1
  selector:
    matchLabels:
      {{- include "magos.selectorLabels" . | nindent 6 }}
      {{- include "magos.postgres.labels" . | nindent 6 }}
  template:
    metadata:
      labels:
        {{- include "magos.labels" . | nindent 8 }}
        {{- include "magos.postgres.labels" . | nindent 8 }}
    spec:
      {{- with .Values.image.pullSecrets }}
      imagePullSecrets:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      securityContext:
        {{- toYaml .Values.postgres.embedded.podSecurityContext | nindent 8 }}
      containers:
      - name: postgres
        image: "{{ .Values.postgres.embedded.image.repository }}:{{ .Values.postgres.embedded.image.tag }}"
        imagePullPolicy: {{ .Values.postgres.embedded.image.pullPolicy }}
        ports:
        - name: postgres
          containerPort: {{ .Values.postgres.embedded.service.port }}
          protocol: TCP
        env:
        - name: POSTGRES_DB
          value: {{ .Values.postgres.embedded.auth.database | quote }}
        - name: POSTGRES_USER
          value: {{ .Values.postgres.embedded.auth.username | quote }}
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: {{ include "magos.postgresSecretName" . }}
              key: {{ include "magos.postgresPasswordKey" . }}
        - name: PGDATA
          value: /var/lib/postgresql/data/pgdata
        readinessProbe:
          exec:
            command:
            - /bin/sh
            - -c
            - pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB"
          initialDelaySeconds: 10
          periodSeconds: 5
        livenessProbe:
          exec:
            command:
            - /bin/sh
            - -c
            - pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB"
          initialDelaySeconds: 30
          periodSeconds: 10
        securityContext:
          {{- toYaml .Values.postgres.embedded.containerSecurityContext | nindent 10 }}
        resources:
          {{- toYaml .Values.postgres.embedded.resources | nindent 10 }}
        volumeMounts:
        - name: data
          mountPath: /var/lib/postgresql/data
      {{- with .Values.postgres.embedded.nodeSelector }}
      nodeSelector:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.postgres.embedded.tolerations }}
      tolerations:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.postgres.embedded.affinity }}
      affinity:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- if not .Values.postgres.embedded.persistence.enabled }}
      volumes:
      - name: data
        emptyDir: {}
      {{- end }}
  {{- if .Values.postgres.embedded.persistence.enabled }}
  volumeClaimTemplates:
  - metadata:
      name: data
      labels:
        {{- include "magos.selectorLabels" . | nindent 8 }}
        {{- include "magos.postgres.labels" . | nindent 8 }}
    spec:
      accessModes:
      - ReadWriteOnce
      {{- with .Values.postgres.embedded.persistence.storageClass }}
      storageClassName: {{ . | quote }}
      {{- end }}
      resources:
        requests:
          storage: {{ .Values.postgres.embedded.persistence.size }}
  {{- end }}
{{- end }}
```

- [ ] **Step 2: Update `postgres/service.yaml`**

Open the file and replace every `.Values.postgres.service` with `.Values.postgres.embedded.service`. Replace every `.Values.postgres.auth` with `.Values.postgres.embedded.auth`. Wrap the entire body in `{{- if eq (include "magos.postgresMode" .) "embedded" }} ... {{- end }}`. Use `magos.labels` + `magos.postgres.labels` for label blocks.

- [ ] **Step 3: Update `postgres/secret.yaml` for `secret.name` BYO**

Replace the file with:

```yaml
{{- if and (eq (include "magos.postgresMode" .) "embedded") (not .Values.postgres.embedded.auth.secret.name) }}
{{- $secretName := include "magos.postgresSecretName" . -}}
{{- $passwordKey := include "magos.postgresPasswordKey" . -}}
{{- $existing := lookup "v1" "Secret" .Release.Namespace $secretName -}}
apiVersion: v1
kind: Secret
metadata:
  name: {{ $secretName }}
  labels:
    {{- include "magos.labels" . | nindent 4 }}
    {{- include "magos.postgres.labels" . | nindent 4 }}
type: Opaque
data:
  {{- if $existing }}
  {{- range $key, $value := omit $existing.data "username" "database" $passwordKey }}
  {{ $key }}: {{ $value | quote }}
  {{- end }}
  {{- end }}
  username: {{ .Values.postgres.embedded.auth.username | b64enc | quote }}
  database: {{ .Values.postgres.embedded.auth.database | b64enc | quote }}
  {{ $passwordKey }}: {{ if and $existing (index $existing.data $passwordKey) }}{{ index $existing.data $passwordKey | quote }}{{ else }}{{ randAlphaNum 32 | b64enc | quote }}{{ end }}
{{- end }}
```

(The raw `.Values.postgres.auth.password` path is gone. Only the auto-generation path remains.)

- [ ] **Step 4: Render and confirm shape**

Run:

```
helm template magos charts/magos --set postgres.mode=embedded > /tmp/render-pg-embedded.yaml
grep -c "MAGOS_POSTGRES_HOST" /tmp/render-pg-embedded.yaml
```

Expected: a positive integer (env wiring still emitted). The render will fail elsewhere until the API/controllers/UI templates are updated; that is fine for this task. If the postgres section itself renders cleanly, the task is done.

### Task 12: Update RustFS templates for new value paths and `secret.name`

**Files:**
- Modify: `charts/magos/templates/rustfs/deployment.yaml`
- Modify: `charts/magos/templates/rustfs/pvc.yaml`
- Modify: `charts/magos/templates/rustfs/secret.yaml`
- Modify: `charts/magos/templates/rustfs/service.yaml`

- [ ] **Step 1: Update `rustfs/deployment.yaml`**

Open the file. Every reference to `.Values.logs.storage.image|auth|service|persistence|resources` becomes `.Values.logs.storage.embedded.<that key>`. Add `magos.labels` + `magos.rustfs.labels` to the metadata and selector. Use `.Values.logs.storage.embedded.podSecurityContext` and `.Values.logs.storage.embedded.containerSecurityContext`. Wrap the entire body in `{{- if eq (include "magos.logsStorageMode" .) "embedded" }} ... {{- end }}`.

The two `RUSTFS_ACCESS_KEY` / `RUSTFS_SECRET_KEY` env entries now reference the Secret using `magos.rustfsAccessKeyKey` / `magos.rustfsSecretKeyKey` instead of hardcoded `accessKey` / `secretKey`:

```yaml
- name: RUSTFS_ACCESS_KEY
  valueFrom:
    secretKeyRef:
      name: {{ include "magos.rustfsSecretName" . }}
      key: {{ include "magos.rustfsAccessKeyKey" . }}
- name: RUSTFS_SECRET_KEY
  valueFrom:
    secretKeyRef:
      name: {{ include "magos.rustfsSecretName" . }}
      key: {{ include "magos.rustfsSecretKeyKey" . }}
```

- [ ] **Step 2: Update `rustfs/pvc.yaml` to read from `.Values.logs.storage.embedded.persistence`**

Same mechanical change. Wrap in the mode check.

- [ ] **Step 3: Update `rustfs/service.yaml` for `.Values.logs.storage.embedded.service`**

Same mechanical change. Wrap in the mode check.

- [ ] **Step 4: Update `rustfs/secret.yaml` for `secret.name` BYO**

Replace with:

```yaml
{{- if and (eq (include "magos.logsStorageMode" .) "embedded") (not .Values.logs.storage.embedded.secret.name) }}
{{- $secretName := include "magos.rustfsSecretName" . -}}
{{- $accessKeyKey := include "magos.rustfsAccessKeyKey" . -}}
{{- $secretKeyKey := include "magos.rustfsSecretKeyKey" . -}}
{{- $existing := lookup "v1" "Secret" .Release.Namespace $secretName -}}
apiVersion: v1
kind: Secret
metadata:
  name: {{ $secretName }}
  labels:
    {{- include "magos.labels" . | nindent 4 }}
    {{- include "magos.rustfs.labels" . | nindent 4 }}
type: Opaque
data:
  {{- if $existing }}
  {{- range $key, $value := omit $existing.data $accessKeyKey $secretKeyKey }}
  {{ $key }}: {{ $value }}
  {{- end }}
  {{- end }}
  {{ $accessKeyKey }}: {{ if and $existing (index $existing.data $accessKeyKey) }}{{ index $existing.data $accessKeyKey }}{{ else }}{{ randAlphaNum 20 | b64enc }}{{ end }}
  {{ $secretKeyKey }}: {{ if and $existing (index $existing.data $secretKeyKey) }}{{ index $existing.data $secretKeyKey }}{{ else }}{{ randAlphaNum 40 | b64enc }}{{ end }}
{{- end }}
```

(Raw `.Values.logs.storage.auth.accessKey` and `secretKey` paths are gone.)

### Task 13: Update API templates for new value paths

**Files:**
- Modify: `charts/magos/templates/api/deployment.yaml`
- Modify: `charts/magos/templates/api/service.yaml`
- Modify: `charts/magos/templates/api/service-account.yaml`
- Modify: `charts/magos/templates/api/cluster-role.yaml`
- Modify: `charts/magos/templates/api/cluster-role-binding.yaml`

- [ ] **Step 1: Rewrite `api/deployment.yaml`**

Replace the file with the version below. Key changes from the existing template: `replicaCount` -> `replicas`, image references go through `magos.api.image`, image pull secrets read `.Values.image.pullSecrets`, `securityContext` / `containerSecurityContext` use the component values with `global.*` fallback, `env` / `envFrom` are appended (global plus component), labels merge with `global.labels`, pod labels and annotations merge with their `global.*` analogues, `nodeSelector` / `tolerations` / `affinity` / `priorityClassName` fall back to `global.*`, probes are guarded by `.Values.api.probes.enabled`. GOMAXPROCS and GOMEMLIMIT env entries are added, using `magos.selectCpuResourceField`.

```yaml
{{- if .Values.api.enabled }}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "magos.fullname" . }}-api
  labels:
    {{- include "magos.labels" . | nindent 4 }}
    {{- include "magos.api.labels" . | nindent 4 }}
    {{- with (mergeOverwrite (deepCopy (default dict .Values.global.labels)) (.Values.api.labels | default dict)) }}
    {{- range $key, $value := . }}
    {{ $key }}: {{ $value | quote }}
    {{- end }}
    {{- end }}
  {{- include "magos.annotations" (dict "root" . "annotations" .Values.api.annotations) | nindent 2 }}
spec:
  replicas: {{ .Values.api.replicas | default 1 }}
  selector:
    matchLabels:
      {{- include "magos.selectorLabels" . | nindent 6 }}
      {{- include "magos.api.labels" . | nindent 6 }}
  template:
    metadata:
      labels:
        {{- include "magos.labels" . | nindent 8 }}
        {{- include "magos.api.labels" . | nindent 8 }}
        {{- with (mergeOverwrite (deepCopy (default dict .Values.global.podLabels)) (.Values.api.podLabels | default dict)) }}
        {{- range $key, $value := . }}
        {{ $key }}: {{ $value | quote }}
        {{- end }}
        {{- end }}
      {{- with (mergeOverwrite (deepCopy (default dict .Values.global.podAnnotations)) (.Values.api.podAnnotations | default dict)) }}
      annotations:
        {{- range $key, $value := . }}
        {{ $key }}: {{ $value | quote }}
        {{- end }}
      {{- end }}
    spec:
      {{- with .Values.image.pullSecrets }}
      imagePullSecrets:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      serviceAccountName: {{ include "magos.apiServiceAccountName" . }}
      {{- with .Values.api.securityContext | default .Values.global.securityContext }}
      securityContext:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      containers:
      - name: api
        image: {{ include "magos.api.image" . }}
        imagePullPolicy: {{ .Values.api.image.pullPolicy }}
        ports:
        - containerPort: 8080
          name: http
          protocol: TCP
        env:
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: GOMEMLIMIT
          valueFrom:
            resourceFieldRef:
              containerName: api
              divisor: "1"
              resource: limits.memory
        - name: GOMAXPROCS
          valueFrom:
            resourceFieldRef:
              containerName: api
              divisor: "1"
              resource: {{ include "magos.selectCpuResourceField" (dict "resources" .Values.api.resources) }}
        {{- include "magos.logstoreEnv" . | nindent 8 }}
        {{- include "magos.postgresEnv" . | nindent 8 }}
        {{- with (concat (default list .Values.global.env) (default list .Values.api.env)) }}
        {{- toYaml . | nindent 8 }}
        {{- end }}
        {{- with (concat (default list .Values.global.envFrom) (default list .Values.api.envFrom)) }}
        envFrom:
          {{- toYaml . | nindent 8 }}
        {{- end }}
        {{- with .Values.api.containerSecurityContext | default .Values.global.containerSecurityContext }}
        securityContext:
          {{- toYaml . | nindent 10 }}
        {{- end }}
        {{- if .Values.api.probes.enabled }}
        livenessProbe:
          httpGet:
            path: /healthz
            port: http
          initialDelaySeconds: 15
          periodSeconds: 20
        readinessProbe:
          httpGet:
            path: /readyz
            port: http
          initialDelaySeconds: 5
          periodSeconds: 10
        {{- end }}
        resources:
          {{- toYaml .Values.api.resources | nindent 10 }}
      {{- with .Values.api.nodeSelector | default .Values.global.nodeSelector }}
      nodeSelector:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.api.tolerations | default .Values.global.tolerations }}
      tolerations:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.api.affinity | default .Values.global.affinity }}
      affinity:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.api.priorityClassName | default .Values.global.priorityClassName }}
      priorityClassName: {{ . }}
      {{- end }}
      terminationGracePeriodSeconds: 30
{{- end }}
```

- [ ] **Step 2: Update `api/service.yaml`**

Adopt `magos.labels` + `magos.api.labels`. Add the `service.annotations` merge with `global.annotations`. Read port from `.Values.api.service.port`. No `replicas`-style fields here.

- [ ] **Step 3: Update `api/service-account.yaml`**

Read `.Values.api.serviceAccount.{create,name,annotations,labels,automount}`. Merge labels and annotations with `global.*` the same way the deployment does.

- [ ] **Step 4: Refresh `api/cluster-role.yaml` and `api/cluster-role-binding.yaml`**

Gate rendering with `.Values.rbac.installClusterRoles` (role) and `.Values.rbac.installClusterRoleBindings` (binding). Replace `.Values.rbac.create` with these two new fields.

### Task 14: Update UI templates for new value paths

**Files:**
- Modify: `charts/magos/templates/ui/deployment.yaml`
- Modify: `charts/magos/templates/ui/service.yaml`

- [ ] **Step 1: Rewrite `ui/deployment.yaml`**

Same pattern as the API deployment. `replicaCount` -> `replicas`, image via `magos.ui.image`, env merge via `global.env` plus `ui.env`, labels and annotations merge with `global.*`, probes guarded by `ui.probes.enabled`, security contexts fall back to `global.*`. Keep the existing `cache` / `run` / `tmp` emptyDir volumes. Add a `MAGOS_TLS_ENABLED` env var defaulted from `ui.tls.enabled` (the nginx config template will read it in Batch 5; for now the env is present but no TLS Secret is mounted yet).

```yaml
env:
- name: MAGOS_API_HOST
  value: {{ include "magos.fullname" . }}-api
- name: MAGOS_API_PORT
  value: "{{ .Values.api.service.port }}"
- name: MAGOS_TLS_ENABLED
  value: {{ .Values.ui.tls.enabled | quote }}
{{- with (concat (default list .Values.global.env) (default list .Values.ui.env)) }}
{{- toYaml . | nindent 8 }}
{{- end }}
```

- [ ] **Step 2: Update `ui/service.yaml`**

Adopt `magos.labels` + `magos.ui.labels`. Honor `ui.service.annotations` merged with `global.annotations`.

### Task 15: Update controller templates for new value paths

**Files:**
- Modify: `charts/magos/templates/controller/deployments.yaml`
- Modify: `charts/magos/templates/controller/service-accounts.yaml`
- Modify: `charts/magos/templates/controller/cluster-role.yaml`
- Modify: `charts/magos/templates/controller/cluster-role-binding.yaml`
- Modify: `charts/magos/templates/controller/leader-election-role.yaml`
- Modify: `charts/magos/templates/controller/leader-election-role-binding.yaml`
- Modify: `charts/magos/templates/controller/metrics-service.yaml`

- [ ] **Step 1: Rewrite `controller/deployments.yaml`**

Same loop as today over `.Values.controllers`. Update every field reference to the new shape: `replicaCount` -> `replicas`, `podDefaults.*` -> `global.*` with per-controller overrides, image via `magos.image`, log store env via `magos.logstoreEnv`, postgres env (workspace only) via `magos.postgresEnv`, probes guarded by `<controller>.probes.enabled`, env merge `global.env` plus `<controller>.env`, label merges with `global.labels`. Add `MAGOS_LOGS_API_URL` to the workspace controller env using `magos.fullname`.

The label merge pattern inside the loop:

```yaml
labels:
  {{- include "magos.labels" $root | nindent 4 }}
  {{- include "magos.controller.labels" $root | nindent 4 }}
  app.kubernetes.io/component: {{ $name }}-controller
  control-plane: magos
  {{- with (mergeOverwrite (deepCopy (default dict $root.Values.global.labels)) ($controller.labels | default dict)) }}
  {{- range $key, $value := . }}
  {{ $key }}: {{ $value | quote }}
  {{- end }}
  {{- end }}
```

(`app.kubernetes.io/component: <name>-controller` stays in addition to the generic `magos.controller.labels` block so per-controller selectors still match what's in the cluster today.)

- [ ] **Step 2: Update `controller/service-accounts.yaml`**

Iterate over `.Values.controllers`, render a ServiceAccount per controller when `serviceAccount.create` is true. Read `serviceAccount.{automount,annotations,labels,name}`.

- [ ] **Step 3: Refresh RBAC files**

`controller/cluster-role.yaml`, `controller/cluster-role-binding.yaml`, `controller/leader-election-role.yaml`, `controller/leader-election-role-binding.yaml`: gate with `.Values.rbac.installClusterRoles` / `.Values.rbac.installClusterRoleBindings`, render once for the controller group as today (the cluster role is shared across the five controllers, the binding iterates over enabled controllers).

- [ ] **Step 4: Refresh `controller/metrics-service.yaml`**

Read from `.Values.metricsService.{enabled,port,type,secure}` (unchanged keys). Use `magos.controller.labels`.

### Task 16: Update job templates for new value paths

**Files:**
- Modify: `charts/magos/templates/job/service-account.yaml`
- Modify: `charts/magos/templates/job/cluster-role.yaml`
- Modify: `charts/magos/templates/job/cluster-role-binding.yaml`

- [ ] **Step 1: Update `job/service-account.yaml`**

Read `.Values.job.serviceAccount.{create,name,automount,annotations,labels}` (path moved from `.Values.jobServiceAccount.*`).

- [ ] **Step 2: Update `job/cluster-role.yaml` and `cluster-role-binding.yaml`**

Gate on `.Values.rbac.installClusterRoles` and `.Values.rbac.installClusterRoleBindings`. Reference the new `.Values.job.serviceAccount.name` (default `magos-job`).

### Task 17: Populate `_validate.tpl` with deprecated-path probes

**Files:**
- Modify: `charts/magos/templates/_validate.tpl`
- Modify: every top-level template (api, ui, controller, postgres, rustfs, job, kyverno) to include `magos.validate` at the top

- [ ] **Step 1: Fill in `_validate.tpl`**

Replace with:

```gotemplate
{{/*
Hard-fail the install when a deprecated value path is set. Pre-batch-3
charts used `podDefaults`, `replicaCount`, flat `postgres.{auth,...}`,
flat `logs.storage.{auth,...}` and `rbac.create`. All of those moved to
new paths in batch 3. The partial probes the legacy paths and fails with
a pointer to the migration table.
*/}}
{{- define "magos.validate" -}}
{{- if hasKey .Values "podDefaults" -}}
{{- fail "values: `podDefaults` was removed. Move its members under `global.*`. See docs/getting-started/installation for the migration table." -}}
{{- end -}}
{{- if and (hasKey .Values "rbac") (hasKey .Values.rbac "create") -}}
{{- fail "values: `rbac.create` was split into `rbac.installClusterRoles` and `rbac.installClusterRoleBindings`. Set both true (the new default) for the same behavior." -}}
{{- end -}}
{{- if hasKey .Values "jobServiceAccount" -}}
{{- fail "values: `jobServiceAccount.*` was renamed to `job.serviceAccount.*`." -}}
{{- end -}}
{{- if and (hasKey .Values "ui") (hasKey .Values.ui "replicaCount") -}}
{{- fail "values: `ui.replicaCount` was renamed to `ui.replicas`." -}}
{{- end -}}
{{- if and (hasKey .Values "api") (hasKey .Values.api "replicaCount") -}}
{{- fail "values: `api.replicaCount` was renamed to `api.replicas`." -}}
{{- end -}}
{{- if hasKey .Values "controllers" -}}
{{- range $name, $controller := .Values.controllers -}}
{{- if and (kindIs "map" $controller) (hasKey $controller "replicaCount") -}}
{{- fail (printf "values: controllers.%s.replicaCount was renamed to controllers.%s.replicas" $name $name) -}}
{{- end -}}
{{- end -}}
{{- end -}}
{{- if hasKey .Values "postgres" -}}
{{- $pg := .Values.postgres -}}
{{- if or (hasKey $pg "auth") (hasKey $pg "image") (hasKey $pg "service") (hasKey $pg "persistence") (hasKey $pg "sslMode") (hasKey $pg "resources") (hasKey $pg "podSecurityContext") (hasKey $pg "containerSecurityContext") -}}
{{- fail "values: flat `postgres.{auth,image,service,persistence,sslMode,resources,podSecurityContext,containerSecurityContext}` were moved under `postgres.embedded.*`. Raw `postgres.auth.password` is gone; set `postgres.embedded.auth.secret.name` or rely on chart auto-generation." -}}
{{- end -}}
{{- if and (hasKey $pg "external") (or (hasKey $pg.external "existingSecret") (hasKey $pg.external "passwordKey")) -}}
{{- fail "values: `postgres.external.existingSecret` / `passwordKey` were grouped under `postgres.external.secret.{name,passwordKey}`." -}}
{{- end -}}
{{- end -}}
{{- if and (hasKey .Values "logs") (hasKey .Values.logs "storage") -}}
{{- $ls := .Values.logs.storage -}}
{{- if or (hasKey $ls "image") (hasKey $ls "auth") (hasKey $ls "service") (hasKey $ls "persistence") (hasKey $ls "resources") -}}
{{- fail "values: flat `logs.storage.{image,auth,service,persistence,resources}` were moved under `logs.storage.embedded.*`. Raw `auth.accessKey`/`auth.secretKey` are gone; set `logs.storage.embedded.secret.name` or rely on chart auto-generation." -}}
{{- end -}}
{{- if and (hasKey $ls "external") (or (hasKey $ls.external "existingSecret") (hasKey $ls.external "accessKeyKey") (hasKey $ls.external "secretKeyKey")) -}}
{{- fail "values: `logs.storage.external.{existingSecret,accessKeyKey,secretKeyKey}` were grouped under `logs.storage.external.secret.{name,accessKeyKey,secretKeyKey}`." -}}
{{- end -}}
{{- end -}}
{{- end -}}
```

- [ ] **Step 2: Wire the include from every top-level template**

At the top of each rendered template file, prepend `{{- include "magos.validate" . -}}`. Files:

- `charts/magos/templates/api/deployment.yaml`
- `charts/magos/templates/api/service.yaml`
- `charts/magos/templates/api/service-account.yaml`
- `charts/magos/templates/api/cluster-role.yaml`
- `charts/magos/templates/api/cluster-role-binding.yaml`
- `charts/magos/templates/ui/deployment.yaml`
- `charts/magos/templates/ui/service.yaml`
- `charts/magos/templates/controller/deployments.yaml`
- `charts/magos/templates/controller/service-accounts.yaml`
- `charts/magos/templates/controller/cluster-role.yaml`
- `charts/magos/templates/controller/cluster-role-binding.yaml`
- `charts/magos/templates/controller/leader-election-role.yaml`
- `charts/magos/templates/controller/leader-election-role-binding.yaml`
- `charts/magos/templates/controller/metrics-service.yaml`
- `charts/magos/templates/job/service-account.yaml`
- `charts/magos/templates/job/cluster-role.yaml`
- `charts/magos/templates/job/cluster-role-binding.yaml`
- `charts/magos/templates/postgres/statefulset.yaml`
- `charts/magos/templates/postgres/service.yaml`
- `charts/magos/templates/postgres/secret.yaml`
- `charts/magos/templates/rustfs/deployment.yaml`
- `charts/magos/templates/rustfs/pvc.yaml`
- `charts/magos/templates/rustfs/service.yaml`
- `charts/magos/templates/rustfs/secret.yaml`
- `charts/magos/templates/kyverno/validatingpolicy-crd.yaml`
- `charts/magos/templates/crds.yaml`

The include emits nothing on success.

- [ ] **Step 3: Verify the partial fires**

Run:

```
helm template magos charts/magos --set podDefaults.foo=bar 2>&1 | grep -c "podDefaults"
```

Expected: > 0. The chart hard-fails with the migration message.

Reset:

```
helm template magos charts/magos -f hack/local-values.yaml > /dev/null
```

Expected: render succeeds (after the values rewrite below).

### Task 18: Update `hack/local-values.yaml` and add dev Secret manifests

**Files:**
- Modify: `charts/magos/templates/kyverno/validatingpolicy-crd.yaml` (no behavior change; already moved in Batch 2)
- Modify: `hack/local-values.yaml`
- Create: `hack/dev-postgres-secret.yaml`
- Create: `hack/dev-rustfs-secret.yaml`
- Modify: `Makefile` (the `install` and `run` targets apply the new dev secrets before `helm install`)

- [ ] **Step 1: Replace `hack/local-values.yaml`**

```yaml
# Local development chart overlay.
#
# `make run` starts the API, UI, and controllers from the host, so the
# component enabled flags below are disabled. Chart installs should keep
# those components enabled and should use this file only for the local
# developer setup.
api:
  enabled: false

ui:
  enabled: false

controllers:
  workspace:
    enabled: false
  project:
    enabled: false
  rollout:
    enabled: false
  variableset:
    enabled: false
  refwatcher:
    enabled: false

logs:
  storage:
    embedded:
      secret:
        name: magos-dev-rustfs
      service:
        type: NodePort
        nodePort: 31900
      persistence:
        size: 1Gi

postgres:
  embedded:
    auth:
      secret:
        name: magos-dev-postgres
    service:
      type: NodePort
      nodePort: 31432
```

- [ ] **Step 2: Create `hack/dev-postgres-secret.yaml`**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: magos-dev-postgres
  namespace: magos-system
type: Opaque
stringData:
  password: ChangeMe123!
```

- [ ] **Step 3: Create `hack/dev-rustfs-secret.yaml`**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: magos-dev-rustfs
  namespace: magos-system
type: Opaque
stringData:
  accessKey: rustfsadmin
  secretKey: ChangeMe123!
```

- [ ] **Step 4: Wire the new Secrets into the Makefile**

Modify the `install` (and any `run`/`kind-load` target that applies the chart) so the dev Secrets are applied before `helm install`. Add:

```
$(KUBECTL) apply -f hack/dev-postgres-secret.yaml
$(KUBECTL) apply -f hack/dev-rustfs-secret.yaml
```

immediately before the line that runs `helm upgrade --install magos ...`.

- [ ] **Step 5: Verify the dev render**

Run:

```
helm template magos charts/magos -f hack/local-values.yaml > /tmp/render-dev.yaml
grep -E "name: magos-dev-postgres|name: magos-dev-rustfs" /tmp/render-dev.yaml
```

Expected: both Secret names appear in `secretKeyRef` blocks for the Postgres password and RustFS access/secret keys. The chart does not render its own `magos-rustfs` / `magos-postgres` Secrets (because `secret.name` is set).

### Task 19: chainsaw: default-install

**Files:**
- Create: `test/chainsaw/tests/chart/chart-default-install/chainsaw-test.yaml`

- [ ] **Step 1: Write the test**

```yaml
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: chart-default-install
spec:
  description: |
    Asserts that a default chart install brings up the API Deployment,
    UI Deployment, five controller Deployments, the embedded Postgres
    StatefulSet, and the embedded RustFS Deployment. No Ingress is
    rendered by default.
  steps:
  - try:
    - assert:
        resource:
          apiVersion: apps/v1
          kind: Deployment
          metadata:
            name: magos-api
            namespace: magos-system
    - assert:
        resource:
          apiVersion: apps/v1
          kind: Deployment
          metadata:
            name: magos-ui
            namespace: magos-system
    - assert:
        resource:
          apiVersion: apps/v1
          kind: StatefulSet
          metadata:
            name: magos-postgres
            namespace: magos-system
    - assert:
        resource:
          apiVersion: apps/v1
          kind: Deployment
          metadata:
            name: magos-rustfs
            namespace: magos-system
    - script:
        content: |
          for c in workspace project rollout variableset refwatcher; do
            kubectl -n magos-system get deployment magos-$c -o name >/dev/null || exit 1
          done
    - script:
        content: |
          if kubectl -n magos-system get ingress 2>/dev/null | grep -q magos; then
            echo "expected no Ingress in default install"
            exit 1
          fi
```

- [ ] **Step 2: Run it**

Run: `./bin/chainsaw test test/chainsaw/tests/chart/chart-default-install`
Expected: PASS (assuming the chart was installed in `magos-system` via `make install`).

### Task 20: chainsaw: postgres-external

**Files:**
- Create: `test/chainsaw/tests/chart/chart-postgres-external/chainsaw-test.yaml`

- [ ] **Step 1: Write the test**

```yaml
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: chart-postgres-external
spec:
  description: |
    Asserts that when postgres.mode=external the chart does not render
    the embedded StatefulSet and the API Deployment is wired against
    the user-supplied host and Secret.
  steps:
  - try:
    - apply:
        resource:
          apiVersion: v1
          kind: Secret
          metadata:
            name: chart-test-pg
            namespace: magos-system
          stringData:
            password: dummy
    - script:
        content: |
          helm template magos ../../charts/magos \
            --set postgres.mode=external \
            --set postgres.external.host=test-pg.default \
            --set postgres.external.secret.name=chart-test-pg \
            > /tmp/render.yaml
          if grep -q "kind: StatefulSet" /tmp/render.yaml | head; then
            echo "embedded Postgres StatefulSet should not render in external mode"
            exit 1
          fi
          grep -q "value: test-pg.default" /tmp/render.yaml || (echo "external host not wired"; exit 1)
          grep -q "name: chart-test-pg" /tmp/render.yaml || (echo "external secret not wired"; exit 1)
```

- [ ] **Step 2: Run it**

Run: `./bin/chainsaw test test/chainsaw/tests/chart/chart-postgres-external`
Expected: PASS.

### Task 21: chainsaw: logs-external

**Files:**
- Create: `test/chainsaw/tests/chart/chart-logs-external/chainsaw-test.yaml`

- [ ] **Step 1: Write the test**

```yaml
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: chart-logs-external
spec:
  description: |
    Asserts that when logs.storage.mode=external the chart does not
    render the embedded RustFS Deployment and the API and workspace
    controller are wired against the user-supplied endpoint and Secret.
  steps:
  - try:
    - apply:
        resource:
          apiVersion: v1
          kind: Secret
          metadata:
            name: chart-test-s3
            namespace: magos-system
          stringData:
            accessKey: ak
            secretKey: sk
    - script:
        content: |
          helm template magos ../../charts/magos \
            --set logs.storage.mode=external \
            --set logs.storage.external.endpoint=https://s3.example.com \
            --set logs.storage.external.secret.name=chart-test-s3 \
            > /tmp/render.yaml
          if grep -q "name: magos-rustfs$" /tmp/render.yaml; then
            echo "embedded RustFS resources should not render in external mode"
            exit 1
          fi
          grep -q "value: https://s3.example.com" /tmp/render.yaml || (echo "external endpoint not wired"; exit 1)
          grep -q "name: chart-test-s3" /tmp/render.yaml || (echo "external secret not wired"; exit 1)
```

- [ ] **Step 2: Run it**

Run: `./bin/chainsaw test test/chainsaw/tests/chart/chart-logs-external`
Expected: PASS.

### Task 22: Batch 3 commit checkpoint

- [ ] **Step 1: Full chart render + lint**

Run:

```
helm lint charts/magos --strict
helm template magos charts/magos > /tmp/render-default.yaml
helm template magos charts/magos -f hack/local-values.yaml > /tmp/render-dev.yaml
```

Expected: all three succeed.

- [ ] **Step 2: Full chainsaw chart suite**

Run: `make install && make test-chainsaw-chart`
Expected: PASS.

- [ ] **Step 3: Commit checkpoint**

Suggested message: `chart: rewrite values shape (global block, embedded vs external, secret.name BYO)`.

---

## Batch 4: Ingress and cert-manager

Single-host front door against the UI service. cert-manager based self-signed cert by default, with BYO Secret as the opt-out.

### Task 23: Shared self-signed Issuer

**Files:**
- Create: `charts/magos/templates/common/cert-issuer.yaml`

- [ ] **Step 1: Write the Issuer**

```yaml
{{- if or (and .Values.ui.enabled .Values.ui.tls.enabled .Values.ui.tls.selfSignedCert)
       (and .Values.ui.enabled .Values.ui.ingress.enabled .Values.ui.ingress.tls.enabled .Values.ui.ingress.tls.selfSignedCert) }}
{{- include "magos.validate" . -}}
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: magos-selfsigned-cert-issuer
  namespace: {{ .Release.Namespace }}
spec:
  selfSigned: {}
{{- end }}
```

- [ ] **Step 2: Verify it renders**

Run:

```
helm template magos charts/magos --set ui.ingress.enabled=true > /tmp/render-issuer.yaml
grep -A1 "kind: Issuer" /tmp/render-issuer.yaml
```

Expected: the Issuer renders.

Run:

```
helm template magos charts/magos > /tmp/render-noissuer.yaml
grep -c "kind: Issuer" /tmp/render-noissuer.yaml
```

Expected: 0.

### Task 24: UI front-door Ingress

**Files:**
- Create: `charts/magos/templates/ui/ingress.yaml`

- [ ] **Step 1: Write the Ingress**

```yaml
{{- if and .Values.ui.enabled .Values.ui.ingress.enabled }}
{{- include "magos.validate" . -}}
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ include "magos.fullname" . }}-ui
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "magos.labels" . | nindent 4 }}
    {{- include "magos.ui.labels" . | nindent 4 }}
  {{- include "magos.annotations" (dict "root" . "annotations" .Values.ui.ingress.annotations) | nindent 2 }}
spec:
  {{- with .Values.ui.ingress.ingressClassName }}
  ingressClassName: {{ . }}
  {{- end }}
  rules:
  - host: {{ quote .Values.ui.host }}
    http:
      paths:
      - pathType: {{ .Values.ui.ingress.pathType | default "ImplementationSpecific" }}
        path: /
        backend:
          service:
            name: {{ include "magos.fullname" . }}-ui
            port:
              {{- if .Values.ui.tls.enabled }}
              number: 443
              {{- else }}
              number: {{ .Values.ui.service.port }}
              {{- end }}
  {{- if .Values.ui.ingress.tls.enabled }}
  tls:
  - hosts:
    - {{ quote .Values.ui.host }}
    secretName: {{ .Values.ui.ingress.tls.secretName }}
  {{- end }}
{{- end }}
```

### Task 25: UI Ingress cert-manager Certificate

**Files:**
- Create: `charts/magos/templates/ui/ingress-cert.yaml`

- [ ] **Step 1: Write the Certificate**

```yaml
{{- if and .Values.ui.enabled .Values.ui.ingress.enabled .Values.ui.ingress.tls.enabled .Values.ui.ingress.tls.selfSignedCert }}
{{- include "magos.validate" . -}}
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: {{ include "magos.fullname" . }}-ui-ingress
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "magos.labels" . | nindent 4 }}
    {{- include "magos.ui.labels" . | nindent 4 }}
spec:
  dnsNames:
  - {{ quote .Values.ui.host }}
  issuerRef:
    group: cert-manager.io
    kind: Issuer
    name: magos-selfsigned-cert-issuer
  secretName: {{ .Values.ui.ingress.tls.secretName }}
{{- end }}
```

### Task 26: chainsaw: ingress-self-signed

**Files:**
- Create: `test/chainsaw/tests/chart/chart-ingress-self-signed/chainsaw-test.yaml`

- [ ] **Step 1: Write the test**

```yaml
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: chart-ingress-self-signed
spec:
  description: |
    Asserts that ui.ingress.enabled + ui.ingress.tls.selfSignedCert
    renders the Ingress, the cert-manager Issuer, the Certificate,
    and that cert-manager produces the named Secret.
  steps:
  - try:
    - script:
        content: |
          helm template magos ../../charts/magos \
            --set ui.ingress.enabled=true \
            --set ui.host=magos.example.test \
            > /tmp/render.yaml
          grep -q "kind: Ingress" /tmp/render.yaml || exit 1
          grep -q "kind: Issuer" /tmp/render.yaml || exit 1
          grep -q "kind: Certificate" /tmp/render.yaml || exit 1
          grep -q "secretName: magos-ui-ingress-cert" /tmp/render.yaml || exit 1
```

- [ ] **Step 2: Run it**

Run: `./bin/chainsaw test test/chainsaw/tests/chart/chart-ingress-self-signed`
Expected: PASS.

### Task 27: chainsaw: ingress-byo-secret

**Files:**
- Create: `test/chainsaw/tests/chart/chart-ingress-byo-secret/chainsaw-test.yaml`

- [ ] **Step 1: Write the test**

```yaml
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: chart-ingress-byo-secret
spec:
  description: |
    Asserts that ui.ingress.enabled with selfSignedCert=false produces
    an Ingress that references the user-supplied Secret and that no
    cert-manager resources are rendered.
  steps:
  - try:
    - script:
        content: |
          helm template magos ../../charts/magos \
            --set ui.ingress.enabled=true \
            --set ui.host=magos.example.test \
            --set ui.ingress.tls.selfSignedCert=false \
            --set ui.ingress.tls.secretName=my-own-cert \
            > /tmp/render.yaml
          grep -q "secretName: my-own-cert" /tmp/render.yaml || exit 1
          if grep -q "kind: Issuer" /tmp/render.yaml; then
            echo "no Issuer should render when selfSignedCert=false"
            exit 1
          fi
          if grep -q "kind: Certificate" /tmp/render.yaml; then
            echo "no Certificate should render when selfSignedCert=false"
            exit 1
          fi
```

- [ ] **Step 2: Run it**

Run: `./bin/chainsaw test test/chainsaw/tests/chart/chart-ingress-byo-secret`
Expected: PASS.

### Task 28: Batch 4 commit checkpoint

- [ ] **Step 1: Full chainsaw chart suite**

Run: `make install && make test-chainsaw-chart`
Expected: PASS.

- [ ] **Step 2: Commit checkpoint**

Suggested message: `chart: add ingress, cert-manager issuer, ingress certificate`.

---

## Batch 5: UI on-pod TLS

For installs without an Ingress controller. UI nginx listens on 443 directly.

### Task 29: nginx config template

**Files:**
- Modify: `ui/nginx.conf.template`

- [ ] **Step 1: Add the conditional 443 server block**

Inside the `http {}` block, after the existing `server { listen 8080; ... }`, append:

```
    server {
        listen 443 ssl;
        server_name _;
        ssl_certificate     /etc/magos/tls/tls.crt;
        ssl_certificate_key /etc/magos/tls/tls.key;

        root  /usr/share/nginx/html;
        index index.html;

        location /apis/ {
            proxy_pass http://${MAGOS_API_HOST}:${MAGOS_API_PORT};
            proxy_http_version 2;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
            proxy_set_header Connection '';
            proxy_buffering off;
            proxy_cache off;
        }

        location / {
            try_files $uri /index.html;
        }
    }
```

The container entrypoint already substitutes `${MAGOS_API_HOST}` and `${MAGOS_API_PORT}` via `envsubst`. The 443 server block is rendered unconditionally by `envsubst` but only serves traffic when the listener is reachable, which only happens when the chart mounts the cert Secret.

- [ ] **Step 2: Update the container entrypoint to skip the 443 block when TLS is off**

The entrypoint lives inline in `ui/Dockerfile` as a single-line `CMD`. Today it is:

```
CMD ["sh", "-c", "envsubst '${MAGOS_API_HOST} ${MAGOS_API_PORT}' < /etc/nginx/templates/nginx.conf.template > /tmp/nginx.conf && nginx -c /tmp/nginx.conf -g 'daemon off;'"]
```

That CMD is no longer readable once branching gets involved, so add an entrypoint script and reference it. Create `ui/docker-entrypoint.sh`:

```sh
#!/bin/sh
set -eu

TEMPLATE=/etc/nginx/templates/nginx.conf.template
OUTPUT=/tmp/nginx.conf

if [ "${MAGOS_TLS_ENABLED:-false}" = "true" ]; then
    envsubst '${MAGOS_API_HOST} ${MAGOS_API_PORT}' < "$TEMPLATE" > "$OUTPUT"
else
    # Strip the optional `server { listen 443 ssl; ... }` block.
    awk '
        /listen 443 ssl;/ { skip = 1 }
        skip && /^    }$/ { skip = 0; next }
        !skip
    ' "$TEMPLATE" | envsubst '${MAGOS_API_HOST} ${MAGOS_API_PORT}' > "$OUTPUT"
fi

exec nginx -c "$OUTPUT" -g 'daemon off;'
```

Then replace the `CMD` in `ui/Dockerfile` with:

```
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh
CMD ["/docker-entrypoint.sh"]
```

### Task 30: Mount the cert Secret in the UI Deployment when `ui.tls.enabled`

**Files:**
- Modify: `charts/magos/templates/ui/deployment.yaml`

- [ ] **Step 1: Add the conditional volume and volumeMount**

After the existing `volumes:` block (cache, run, tmp), add:

```yaml
{{- if .Values.ui.tls.enabled }}
- name: tls
  secret:
    secretName: {{ .Values.ui.tls.secretName }}
{{- end }}
```

And in the container's `volumeMounts:`:

```yaml
{{- if .Values.ui.tls.enabled }}
- name: tls
  mountPath: /etc/magos/tls
  readOnly: true
{{- end }}
```

And add `containerPort: 443` next to the existing `containerPort: 8080`:

```yaml
{{- if .Values.ui.tls.enabled }}
- containerPort: 443
  name: https
  protocol: TCP
{{- end }}
```

### Task 31: Direct-TLS cert-manager Certificate

**Files:**
- Create: `charts/magos/templates/ui/cert.yaml`

- [ ] **Step 1: Write the Certificate**

```yaml
{{- if and .Values.ui.enabled .Values.ui.tls.enabled .Values.ui.tls.selfSignedCert }}
{{- include "magos.validate" . -}}
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: {{ include "magos.fullname" . }}-ui
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "magos.labels" . | nindent 4 }}
    {{- include "magos.ui.labels" . | nindent 4 }}
spec:
  dnsNames:
  - {{ quote .Values.ui.host }}
  issuerRef:
    group: cert-manager.io
    kind: Issuer
    name: magos-selfsigned-cert-issuer
  secretName: {{ .Values.ui.tls.secretName }}
{{- end }}
```

### Task 32: Update UI Service for the HTTPS port

**Files:**
- Modify: `charts/magos/templates/ui/service.yaml`

- [ ] **Step 1: Add the conditional 443 port**

Inside the `ports:` block:

```yaml
{{- if .Values.ui.tls.enabled }}
- name: https
  port: 443
  targetPort: https
  protocol: TCP
{{- end }}
```

Keep the existing HTTP port so port-forward still works for in-cluster debugging.

### Task 33: chainsaw: ui-on-pod-tls

**Files:**
- Create: `test/chainsaw/tests/chart/chart-ui-on-pod-tls/chainsaw-test.yaml`

- [ ] **Step 1: Write the test**

```yaml
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: chart-ui-on-pod-tls
spec:
  description: |
    Asserts that ui.tls.enabled mounts the cert Secret on the UI pod
    and exposes a 443 port on the Service.
  steps:
  - try:
    - script:
        content: |
          helm template magos ../../charts/magos \
            --set ui.tls.enabled=true \
            > /tmp/render.yaml
          grep -q "containerPort: 443" /tmp/render.yaml || exit 1
          grep -q "name: tls" /tmp/render.yaml || exit 1
          grep -q "secretName: magos-ui-cert" /tmp/render.yaml || exit 1
```

- [ ] **Step 2: Run it**

Run: `./bin/chainsaw test test/chainsaw/tests/chart/chart-ui-on-pod-tls`
Expected: PASS.

### Task 34: Batch 5 commit checkpoint

- [ ] **Step 1: Lint and full chainsaw chart suite**

Run: `helm lint charts/magos --strict && make install && make test-chainsaw-chart`
Expected: PASS.

- [ ] **Step 2: Commit checkpoint**

Suggested message: `chart: add UI on-pod TLS via cert-manager`.

---

## Batch 6: PDB, topology, priorityClassName, probes toggle, cabundle

Cross-cutting Kargo conventions, applied after the templates and values are in their final shape.

### Task 35: PodDisruptionBudget for UI and API

**Files:**
- Create: `charts/magos/templates/api/pdb.yaml`
- Create: `charts/magos/templates/ui/pdb.yaml`

- [ ] **Step 1: Write `api/pdb.yaml`**

```yaml
{{- if and .Values.api.enabled .Values.api.podDisruptionBudget.enabled }}
{{- include "magos.validate" . -}}
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: {{ include "magos.fullname" . }}-api
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "magos.labels" . | nindent 4 }}
    {{- include "magos.api.labels" . | nindent 4 }}
spec:
  {{- if .Values.api.podDisruptionBudget.maxUnavailable }}
  maxUnavailable: {{ .Values.api.podDisruptionBudget.maxUnavailable }}
  {{- else }}
  minAvailable: {{ .Values.api.podDisruptionBudget.minAvailable }}
  {{- end }}
  selector:
    matchLabels:
      {{- include "magos.selectorLabels" . | nindent 6 }}
      {{- include "magos.api.labels" . | nindent 6 }}
{{- end }}
```

- [ ] **Step 2: Write `ui/pdb.yaml`**

Same shape with `ui` substituted for `api`.

### Task 36: topologySpreadConstraints + priorityClassName + topology in controllers loop

**Files:**
- Modify: `charts/magos/templates/api/deployment.yaml`
- Modify: `charts/magos/templates/ui/deployment.yaml`
- Modify: `charts/magos/templates/controller/deployments.yaml`

- [ ] **Step 1: Add `topologySpreadConstraints` block to API, UI, and each controller**

Inside `spec.template.spec`, after `tolerations:`:

```yaml
{{- with .Values.api.topologySpreadConstraints }}
topologySpreadConstraints:
  {{- toYaml . | nindent 8 }}
{{- end }}
```

Repeat with `.Values.ui.topologySpreadConstraints` for UI. In `controller/deployments.yaml`, use `$controller.topologySpreadConstraints`.

- [ ] **Step 2: priorityClassName already added in Batch 3 via `default .Values.global.priorityClassName`**

Confirm it is present in the API and UI deployments. If missing on the controllers loop, add inside the loop body:

```yaml
{{- with $controller.priorityClassName | default $root.Values.global.priorityClassName }}
priorityClassName: {{ . }}
{{- end }}
```

### Task 37: cabundle init container + projected volume

**Files:**
- Modify: `charts/magos/templates/api/deployment.yaml`
- Modify: `charts/magos/templates/controller/deployments.yaml`

- [ ] **Step 1: Add the init container and projected volume to the API deployment**

Just before the existing `terminationGracePeriodSeconds: 30` line, add:

```yaml
{{- if or .Values.api.cabundle.configMapName .Values.api.cabundle.secretName }}
initContainers:
- name: parse-cabundle
  image: {{ include "magos.api.image" . }}
  imagePullPolicy: {{ .Values.api.image.pullPolicy }}
  securityContext:
    runAsUser: 0
  command: ["/bin/sh", "-c"]
  args:
  - |
    for file in /tmp/source/*; do
      base=$(basename "$file" .crt)
      awk 'BEGIN {c=0;} /BEGIN CERT/{c++} { print > "/usr/local/share/ca-certificates/" base "." c ".crt"}' base="$base" < "$file"
    done
    /usr/sbin/update-ca-certificates
    find /etc/ssl/certs -type l -exec cp --remove-destination {} /etc/ssl/certs/ \;
    cp -r /etc/ssl/certs/* /tmp/target/
  volumeMounts:
  - { name: cabundle, mountPath: /tmp/source }
  - { name: certs,    mountPath: /tmp/target }
{{- end }}
```

And inside the container's `volumeMounts:`:

```yaml
{{- if or .Values.api.cabundle.configMapName .Values.api.cabundle.secretName }}
- name: certs
  mountPath: /etc/ssl/certs
{{- end }}
```

And add a top-level `volumes:` block (or extend the existing one) with:

```yaml
{{- if or .Values.api.cabundle.configMapName .Values.api.cabundle.secretName }}
- name: cabundle
  {{- if .Values.api.cabundle.secretName }}
  secret:
    secretName: {{ .Values.api.cabundle.secretName }}
  {{- else }}
  configMap:
    name: {{ .Values.api.cabundle.configMapName }}
  {{- end }}
- name: certs
  emptyDir: {}
{{- end }}
```

- [ ] **Step 2: Repeat the same three additions inside the controllers loop**

In `controller/deployments.yaml`, gate on `$controller.cabundle.configMapName` / `$controller.cabundle.secretName`. The init container image is `magos.image` (the controller binary), not the API image.

### Task 37b: Configmap/secret checksum annotations (deferred)

The spec calls for `configmap/checksum` and `secret/checksum` pod template annotations à la Kargo's API Deployment. Magos has no per-component ConfigMap or Secret today; the API picks up its config through env. The OIDC spec (`docs/superpowers/specs/2026-05-19-built-in-admin-and-oidc-auth-design.md`) adds an `api/secret.yaml` and an `api/configmap.yaml`. When that lands, append the two checksum annotations to the API pod template metadata at that point:

```yaml
annotations:
  configmap/checksum: {{ pick (include (print $.Template.BasePath "/api/configmap.yaml") . | fromYaml) "data" | toYaml | sha256sum }}
  secret/checksum: {{ pick (include (print $.Template.BasePath "/api/secret.yaml") . | fromYaml) "stringData" | toYaml | sha256sum }}
```

No task in this plan; leaving as a documented carryover.

### Task 38: Probes toggle audit

**Files:**
- Modify: `charts/magos/templates/api/deployment.yaml`
- Modify: `charts/magos/templates/ui/deployment.yaml`
- Modify: `charts/magos/templates/controller/deployments.yaml`

- [ ] **Step 1: Confirm liveness/readiness blocks are wrapped in `{{- if <component>.probes.enabled }}`**

Walk all three deployments. Each `livenessProbe:` / `readinessProbe:` pair must be inside `{{- if .Values.api.probes.enabled }}` (or `.Values.ui` or `$controller`). The API and UI already got this in Batch 3; verify the controllers loop has it too.

### Task 39: Batch 6 commit checkpoint

- [ ] **Step 1: Render and verify PDB toggles**

Run:

```
helm template magos charts/magos --set api.podDisruptionBudget.enabled=true | grep -c "kind: PodDisruptionBudget"
helm template magos charts/magos | grep -c "kind: PodDisruptionBudget"
```

Expected: `1` then `0`.

- [ ] **Step 2: Render and verify cabundle toggles**

Run:

```
helm template magos charts/magos --set api.cabundle.configMapName=my-ca | grep -c "parse-cabundle"
```

Expected: `1`.

- [ ] **Step 3: Full lint + chainsaw**

Run: `helm lint charts/magos --strict && make install && make test-chainsaw-chart`
Expected: PASS.

- [ ] **Step 4: Commit checkpoint**

Suggested message: `chart: add pdb, topology spread, priorityClassName, probes toggle, cabundle`.

---

## Batch 7: NOTES.txt, extraObjects, README annotations

### Task 40: extraObjects passthrough

**Files:**
- Create: `charts/magos/templates/extra-manifests.yaml`

- [ ] **Step 1: Write the template**

```yaml
{{- range .Values.extraObjects }}
---
{{- if kindIs "string" . }}
{{ tpl . $ }}
{{- else }}
{{ tpl (toYaml .) $ }}
{{- end }}
{{- end }}
```

- [ ] **Step 2: Verify it renders nothing when empty and renders an object when set**

Run:

```
helm template magos charts/magos --set 'extraObjects[0].apiVersion=v1' --set 'extraObjects[0].kind=ConfigMap' --set 'extraObjects[0].metadata.name=probe' --set 'extraObjects[0].data.x=y' | grep -A2 "name: probe"
```

Expected: the `probe` ConfigMap is in the render.

### Task 41: NOTES.txt

**Files:**
- Create: `charts/magos/templates/NOTES.txt`

- [ ] **Step 1: Write NOTES**

```gotemplate
Magos {{ .Chart.AppVersion }} is installed in the {{ .Release.Namespace }} namespace.

Components:
- API:                 {{ if .Values.api.enabled }}enabled{{ else }}disabled{{ end }}
- UI:                  {{ if .Values.ui.enabled }}enabled{{ else }}disabled{{ end }}
- Controllers:         {{ range $name, $c := .Values.controllers }}{{ if $c.enabled }}{{ $name }} {{ end }}{{ end }}
- Postgres mode:       {{ include "magos.postgresMode" . }}
- Logs storage mode:   {{ include "magos.logsStorageMode" . }}

{{- if .Values.ui.ingress.enabled }}

UI is reachable at: {{ include "magos.ui.baseURL" . }}
{{- else if .Values.ui.enabled }}

UI is reachable via port-forward:

  kubectl -n {{ .Release.Namespace }} port-forward svc/{{ include "magos.fullname" . }}-ui 8080:{{ .Values.ui.service.port }}

Then open http://localhost:8080
{{- end }}

{{- if or (and .Values.ui.tls.enabled .Values.ui.tls.selfSignedCert) (and .Values.ui.ingress.enabled .Values.ui.ingress.tls.enabled .Values.ui.ingress.tls.selfSignedCert) }}

NOTE: cert-manager CRDs must be installed in this cluster.
      The chart renders cert-manager.io/v1 Issuer and Certificate resources.
      If you do not have cert-manager set up, install it first or set
      ui.{tls,ingress.tls}.selfSignedCert=false and bring your own Secret.
{{- end }}

{{- if eq (include "magos.postgresMode" .) "embedded" }}
{{- if not .Values.postgres.embedded.auth.secret.name }}

NOTE: The embedded PostgreSQL password is auto-generated by the chart and
      preserved across upgrades. For production set
      postgres.embedded.auth.secret.name to a Secret you manage.
{{- end }}
{{- end }}

{{- if eq (include "magos.logsStorageMode" .) "embedded" }}
{{- if not .Values.logs.storage.embedded.secret.name }}

NOTE: The embedded RustFS access/secret keys are auto-generated by the
      chart and preserved across upgrades. For production set
      logs.storage.embedded.secret.name to a Secret you manage.
{{- end }}
{{- end }}
```

- [ ] **Step 2: Verify NOTES renders**

Run: `helm install --dry-run magos charts/magos --namespace magos-system 2>&1 | tail -40`
Expected: NOTES is printed.

### Task 42: README generation via helm-docs

**Files:**
- Modify: `charts/magos/values.yaml` (add `## @section` and `## @param` annotations)
- Modify: `Makefile` (add a `chart-docs` target)

- [ ] **Step 1: Annotate `values.yaml`**

Walk `values.yaml` top to bottom, prepend each value with a `## @param <path> <description>` comment (and each section with `## @section <Name>`). Follow the structure from Kargo's `charts/kargo/values.yaml` for tone and density (one line per field, plain prose).

- [ ] **Step 2: Add the make target**

Append to `Makefile`:

```
HELM_DOCS_VERSION ?= v1.14.2
HELM_DOCS := $(LOCALBIN)/helm-docs

$(HELM_DOCS): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install github.com/norwoodj/helm-docs/cmd/helm-docs@$(HELM_DOCS_VERSION)

.PHONY: chart-docs
chart-docs: $(HELM_DOCS) ## Generate charts/magos/README.md from values.yaml annotations.
	$(HELM_DOCS) --chart-search-root charts/magos
```

- [ ] **Step 3: Generate and commit `charts/magos/README.md`**

Run: `make chart-docs && head -50 charts/magos/README.md`
Expected: a structured README with one row per `@param`.

### Task 43: Batch 7 commit checkpoint

- [ ] **Step 1: Render + lint**

Run: `helm lint charts/magos --strict && helm template magos charts/magos > /dev/null`
Expected: PASS.

- [ ] **Step 2: Commit checkpoint**

Suggested message: `chart: add NOTES.txt, extraObjects, helm-docs README`.

---

## Batch 8: Installation docs rewrite

### Task 44: Rewrite `installation/index.mdx`

**Files:**
- Modify: `website/contents/docs/getting-started/installation/index.mdx`

- [ ] **Step 1: Replace the page body**

The new page covers:

1. Prerequisites including the conditional cert-manager requirement.
2. Default install command (unchanged shape).
3. Two example `values.yaml` blocks: defaults and production (external Postgres, external S3, Ingress with self-signed cert).
4. The `mode: embedded | external` toggle for Postgres and logs storage, with the `secret.name` BYO pattern.
5. The single front-door URL story (UI proxies API at `/apis/`).
6. A migration-from-0.1.x section listing the renamed paths.

Use the migration table from `docs/superpowers/specs/2026-05-20-chart-maturity-design.md` ("Migration from current values") as the source. Keep the existing `<Note type="danger" title="Under Development">` callout. Write in the project's existing docs voice (plain prose, no marketing language).

The production example values block:

```yaml
ui:
  host: magos.example.com
  ingress:
    enabled: true
    ingressClassName: nginx
    tls:
      enabled: true
      selfSignedCert: false
      secretName: magos-ui-cert

postgres:
  mode: external
  external:
    host: postgres.internal.example.com
    port: 5432
    database: magos
    username: magos
    secret:
      name: magos-postgres
      passwordKey: password
    sslMode: require

logs:
  storage:
    mode: external
    external:
      endpoint: https://s3.internal.example.com
      secret:
        name: magos-s3
        accessKeyKey: accessKey
        secretKeyKey: secretKey
```

The migration section is the table from the spec, verbatim.

- [ ] **Step 2: Lint the docs site if a docs CI script exists**

Run any `make docs-lint` / `npm run lint:docs` target if one exists in `website/`. If not, skip.

- [ ] **Step 3: Commit checkpoint**

Suggested message: `docs: rewrite installation page for chart maturity`.

---

## Final validation

### Task 45: End-to-end smoke

- [ ] **Step 1: Full chart suite green**

Run:

```
helm lint charts/magos --strict
make install
make test-chainsaw-chart
make test-chainsaw
```

Expected: every command exits 0.

- [ ] **Step 2: Spot-check NOTES**

Run: `helm install --dry-run magos charts/magos --namespace magos-system 2>&1 | tail -40`
Expected: NOTES prints with the front-door URL hint (port-forward path), the embedded credential notice for both Postgres and RustFS, no cert-manager warning (because no `selfSignedCert` toggle is set).

- [ ] **Step 3: Spot-check the deprecation guard**

Run:

```
helm template magos charts/magos --set podDefaults.foo=bar 2>&1 | grep -F "podDefaults"
helm template magos charts/magos --set postgres.auth.password=x 2>&1 | grep -F "postgres.{auth"
helm template magos charts/magos --set logs.storage.auth.accessKey=x 2>&1 | grep -F "logs.storage.{image"
```

Expected: each command prints the relevant `fail` message.

- [ ] **Step 4: Read the rendered Kargo-comparison once**

Render the chart with the production example values from the docs and skim the YAML to make sure it looks like something you would happily ship. If it does not, file the rough edges as follow-ups; do not block the merge on subjective polish.

- [ ] **Step 5: Done.**

The chart is now Kargo-shaped, BYO storage is a first-class value path, raw credentials no longer live in `values.yaml`, and the installation docs match the new surface.
