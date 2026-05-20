# chart: mature the Helm chart along Kargo conventions

Branch: `f/chart-improvements`
Spec: `docs/superpowers/specs/2026-05-20-chart-maturity-design.md`
Plan: `docs/superpowers/plans/2026-05-20-chart-maturity.md`

## Summary

Rewrites `charts/magos/` along the conventions Akuity's Kargo chart uses. Templates are now per-component folders, values gained a `global` block and a clear `embedded` vs `external` toggle for both Postgres and the S3 log store, raw credentials no longer live in `values.yaml`, and the chart now ships an Ingress with cert-manager based TLS, PodDisruptionBudget, topology spread, priorityClassName, probe toggles, CA bundle support, `extraObjects`, NOTES.txt, and a generated chart README. This is a breaking values rewrite (chart is still 0.x). The chart hard-fails the install when any removed key is set, with a pointer to the migration table.

## Why

The chart worked but was shaped like a v0 spike. Three concrete pressures drove the rewrite:

1. **BYO Postgres and BYO S3 should be obvious value paths.** The previous shape buried external mode under `postgres.external.*` while embedded settings lived at `postgres.{auth,service,...}`. The new `postgres.{mode, embedded.*, external.*}` and `logs.storage.{mode, embedded.*, external.*}` split makes the toggle the first thing a reader sees.
2. **Credentials should not live as raw `values.yaml` fields.** `postgres.auth.password` and `logs.storage.auth.{accessKey,secretKey}` are gone. Credentials come from a chart-managed Secret (embedded default install) or a user-supplied Secret named through `secret.name` (production). Same shape Kargo uses for `api.secret.name`.
3. **The chart had no front door.** No Ingress, no TLS, no cert option. Added a single Ingress aimed at the UI Service (the UI's nginx already proxies `/apis/*` to the API in-cluster, so one host covers both) with cert-manager based self-signed TLS by default and BYO Secret as the opt-out.

## What changed

### Template layout

Templates moved from a flat `charts/magos/templates/` into per-component folders that mirror Kargo's layout:

```
charts/magos/templates/
  _helpers.tpl
  _validate.tpl
  crds.yaml
  extra-manifests.yaml
  NOTES.txt
  common/cert-issuer.yaml
  api/{deployment,service,service-account,cluster-role,cluster-role-binding,pdb}.yaml
  ui/{deployment,service,ingress,ingress-cert,cert,pdb}.yaml
  controller/{deployments,service-accounts,cluster-role,cluster-role-binding,leader-election-role,leader-election-role-binding,metrics-service}.yaml
  job/{service-account,cluster-role,cluster-role-binding}.yaml
  postgres/{statefulset,service,secret}.yaml
  rustfs/{deployment,pvc,service,secret}.yaml
  kyverno/validatingpolicy-crd.yaml
```

The empty `templates/garage/` leftover is gone.

### Values shape

Top-level changes (breaking):

- `replicaCount` becomes `replicas` everywhere.
- `podDefaults.*` removed; members move to `global.{labels,annotations,podLabels,podAnnotations,env,envFrom,nodeSelector,tolerations,affinity,priorityClassName,securityContext,containerSecurityContext}`.
- `rbac.create` splits into `rbac.installClusterRoles` and `rbac.installClusterRoleBindings`.
- `jobServiceAccount.*` moves under `job.serviceAccount.*`.
- `postgres.{auth,image,service,persistence,sslMode,resources,podSecurityContext,containerSecurityContext}` collapse into `postgres.embedded.*`.
- `logs.storage.{image,auth,service,persistence,resources}` collapse into `logs.storage.embedded.*`.
- `postgres.embedded.auth.password` removed entirely; chart auto-generates and preserves via `lookup`, or set `postgres.embedded.auth.secret.name` for BYO.
- `logs.storage.embedded.auth.{accessKey,secretKey}` removed entirely; same pattern with `logs.storage.embedded.secret.name`.
- `existingSecret` / `passwordKey` / `accessKeyKey` / `secretKeyKey` everywhere become a nested `secret: {name, passwordKey}` (Postgres) or `secret: {name, accessKeyKey, secretKeyKey}` (RustFS) block. Matches Kargo's `api.secret.name` shape.
- Every component (`api`, `ui`, each controller) gains its own `labels`, `annotations`, `podLabels`, `podAnnotations`, `nodeSelector`, `tolerations`, `affinity`, `priorityClassName`, `securityContext`, `containerSecurityContext`, `env`, `envFrom`, `topologySpreadConstraints`, `podDisruptionBudget`, `probes.enabled`, `cabundle`. All default to empty and merge with `global.*` through helpers.
- Top-level `extraObjects: []` for arbitrary user manifests.

Full migration table is in the installation docs page.

### New helpers (`_helpers.tpl`)

- Image helpers: `magos.image`, `magos.jobImage`, `magos.api.image`, `magos.ui.image`.
- Per-component label helpers: `magos.api.labels`, `magos.ui.labels`, `magos.controller.labels`, `magos.postgres.labels`, `magos.rustfs.labels`, `magos.job.labels`.
- Annotation merge helper: `magos.annotations` (root + component, mirrors Kargo's helper).
- TLS helpers: `magos.useTLS`, `magos.baseURL`, `magos.ui.baseURL`.
- GOMAXPROCS source-field helper: `magos.selectCpuResourceField`.
- Updated storage-wiring helpers: `magos.postgresEnv`, `magos.logstoreEnv`, `magos.postgresSecretName`, `magos.rustfsSecretName`, etc. now read from the new `embedded.*` / `external.secret.*` paths.

### Ingress and TLS

One Ingress (`ui/ingress.yaml`), one host, single front door:

- `ui.ingress.enabled` gates the resource.
- `ui.host` is the DNS name.
- `ui.ingress.tls.enabled` + `selfSignedCert` + `secretName` follow the Kargo pattern. `selfSignedCert: true` renders a cert-manager.io/v1 Certificate against `magos-selfsigned-cert-issuer` (a namespaced Issuer in `templates/common/cert-issuer.yaml`). `selfSignedCert: false` requires the user to provide the Secret.
- `ui.tls.enabled` is the on-pod TLS path for clusters without an Ingress controller; the UI nginx listens on 443 in addition to 8080, and the chart mounts the cert Secret at `/etc/magos/tls/`. New `ui/cert.yaml` mirrors `ui/ingress-cert.yaml` for this path.
- cert-manager is not a hard dependency. When every `selfSignedCert` toggle is false, no cert-manager resources render. NOTES.txt warns when a toggle is on and the cluster might not have cert-manager.

### UI Dockerfile and entrypoint

The UI's nginx config grew an optional `server { listen 443 ssl; ... }` block. The Dockerfile's previous inline `CMD ["sh", "-c", "envsubst ... | nginx"]` is replaced with a small `ui/docker-entrypoint.sh` that strips the 443 block via `awk` when `MAGOS_TLS_ENABLED` is not `true`, then runs `envsubst`, then execs nginx. The chart sets `MAGOS_TLS_ENABLED` from `ui.tls.enabled`.

### Embedded Secret rendering

`postgres/secret.yaml` and `rustfs/secret.yaml` are now gated by `(not .Values.<...>.secret.name)`. When the user supplies their own Secret, the chart renders nothing in those files. When they do not, the chart generates random credentials with `randAlphaNum` on first install and preserves them across upgrades via `lookup`. Same trick the chart used before, minus the raw value override paths.

### Cross-cutting Kargo conventions

- `api/pdb.yaml` and `ui/pdb.yaml`: `policy/v1 PodDisruptionBudget` gated by `<component>.podDisruptionBudget.enabled`. Prefers `maxUnavailable` over `minAvailable` when both are set.
- `topologySpreadConstraints` on API, UI, and each controller.
- `priorityClassName` on every deployment, falling back to `global.priorityClassName`.
- `probes.enabled` toggle wraps every `livenessProbe` / `readinessProbe` pair on every deployment.
- `cabundle.{configMapName,secretName}` on API and each controller. When set, a `parse-cabundle` init container splits the multi-cert PEM, runs `update-ca-certificates`, and copies the result into a `certs` emptyDir mounted at `/etc/ssl/certs` in the main container. The init container script starts with `mkdir -p /usr/local/share/ca-certificates` so it works on Alpine (Magos images) and Debian alike.
- `extra-manifests.yaml` ranges over `extraObjects` and renders each entry (string or object), supporting `tpl` expansion against the chart context.

### Deprecation guard (`_validate.tpl`)

A new `_helpers`-style partial that `fail`s the install with a specific message when any deprecated value path is present. Covers: `podDefaults`, `rbac.create`, `jobServiceAccount`, `ui.replicaCount`, `api.replicaCount`, `controllers.<name>.replicaCount`, legacy flat `postgres.{auth,image,service,persistence,sslMode,resources,podSecurityContext,containerSecurityContext}`, `postgres.external.{existingSecret,passwordKey}`, legacy flat `logs.storage.{image,auth,service,persistence,resources}`, `logs.storage.external.{existingSecret,accessKeyKey,secretKeyKey}`. The partial is included from every top-level template via `{{- include "magos.validate" . -}}`.

### NOTES.txt

Prints the install location, the front-door URL when Ingress is enabled (or a `kubectl port-forward` example when it is not), warns when a `selfSignedCert` toggle is true (cert-manager required), and warns when embedded mode runs without `secret.name` (credentials are chart-managed).

### Generated chart README

`charts/magos/README.md` is now generated by `@bitnami/readme-generator-for-helm` from `## @param` annotations in `values.yaml`. Wired via `make chart-docs`, which is now a dependency of `make generate` so the README stays in sync. The generator is configured by `hack/helm-docs/readme-generator-config.json` and invoked through `hack/helm-docs/helm-docs.sh`. Same setup Kargo uses.

### Dev workflow

`hack/local-values.yaml` no longer carries raw credentials. Two new Secret manifests, `hack/dev-postgres-secret.yaml` and `hack/dev-rustfs-secret.yaml`, hold the dev credentials. The Makefile's `install-local-chart` target applies both before `helm install`, and the `run` target reads the env from the dev Secret names. Keeps `make run` deterministic without reintroducing raw credential paths into the chart.

### Installation docs

`website/contents/docs/getting-started/installation/index.mdx` rewritten (1171 words) to cover the cert-manager prerequisite, the `mode: embedded | external` toggle, the single front-door URL story, a production-shaped example values block, and a full migration table from the 0.1.x shape.

## Test plan

- `helm lint charts/magos --strict` exits 0.
- `helm template magos charts/magos` produces 8 Deployments (5 controllers + API + UI + RustFS), 1 StatefulSet (Postgres), 5 Services, 5 CRDs, 3 ClusterRoles, 3 ClusterRoleBindings, 7 ServiceAccounts, 2 Secrets (Postgres + RustFS chart-managed), 1 PVC, 1 Role, 1 RoleBinding.
- `helm template ... --set postgres.mode=external --set postgres.external.host=h --set postgres.external.secret.name=s` renders no Postgres StatefulSet.
- `helm template ... --set logs.storage.mode=external --set logs.storage.external.endpoint=e --set logs.storage.external.secret.name=s` renders no RustFS resources.
- `helm template ... --set ui.ingress.enabled=true --set ui.host=...` renders Ingress + Issuer + Certificate.
- `helm template ... --set ui.ingress.enabled=true --set ui.ingress.tls.selfSignedCert=false --set ui.ingress.tls.secretName=...` renders Ingress only, no cert-manager resources.
- `helm template ... --set ui.tls.enabled=true` mounts the cert Secret on the UI pod and exposes a 443 port.
- `helm template ... --set api.podDisruptionBudget.enabled=true --set ui.podDisruptionBudget.enabled=true` renders two PodDisruptionBudgets.
- `helm template ... --set api.cabundle.configMapName=my-ca` renders the cabundle init container on the API.
- `helm template ... --set podDefaults.foo=bar` hard-fails with the migration pointer.
- `helm install --dry-run ... --namespace magos-system` prints NOTES.txt with the port-forward hint and the per-store credential warnings.
- `make chart-docs` regenerates `charts/magos/README.md` without diff drift.

New chainsaw tests under `test/chainsaw/tests/chart/`:

- `chart-default-install`
- `chart-postgres-external`
- `chart-logs-external`
- `chart-ingress-self-signed`
- `chart-ingress-byo-secret`
- `chart-ui-on-pod-tls`

Existing chainsaw chart tests (`policy-crd-and-rbac`) still pass.

## Out of scope (deferred)

- Pod template `configmap/checksum` and `secret/checksum` annotations. Magos has no per-component ConfigMap or Secret today. The OIDC spec (`docs/superpowers/specs/2026-05-19-built-in-admin-and-oidc-auth-design.md`) adds `api/configmap.yaml` and `api/secret.yaml`; the checksum annotations land with that PR.
- ServiceMonitor / Prometheus operator integration. `metricsService` toggle is still present; the ServiceMonitor resource is left for a follow-up.
- Exposing `MAGOS_LOGS_S3_REGION` and `MAGOS_LOGS_S3_BUCKET` at the chart layer. Both are hardcoded in `internal/logstore/logstore.go`; making them configurable is a paired Go change tracked separately.

## Migration for existing 0.1.x installs

The chart will hard-fail with a specific message when any deprecated key is set. The error message names the new path. Full mapping lives in `docs/superpowers/specs/2026-05-20-chart-maturity-design.md` and `website/contents/docs/getting-started/installation/index.mdx` ("Upgrading from 0.1.x").

The dev installation (anyone running `make run`) needs to apply the two new dev Secrets first; the Makefile does that automatically as part of `make install-local-chart`.
