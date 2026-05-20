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
