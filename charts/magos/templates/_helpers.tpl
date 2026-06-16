{{/*
Expand the name of the chart.
*/}}
{{- define "magos.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "magos.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "magos.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

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

{{/*
Common labels
*/}}
{{- define "magos.labels" -}}
helm.sh/chart: {{ include "magos.chart" . }}
{{ include "magos.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "magos.selectorLabels" -}}
app.kubernetes.io/name: {{ include "magos.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

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

{{/*
Returns a non-empty value when at least one controller is enabled.
*/}}
{{- define "magos.controllersEnabled" -}}
{{- range $controller := .Values.controllers -}}
{{- if $controller.enabled -}}true{{- end -}}
{{- end -}}
{{- end }}

{{/*
Create the name of the service account to use for a controller
*/}}
{{- define "magos.controllerServiceAccountName" -}}
{{- if .controller.serviceAccount.create }}
{{- default (printf "%s-%s" (include "magos.fullname" .root) .name) .controller.serviceAccount.name }}
{{- else }}
{{- default "default" .controller.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the service account to use for the API
*/}}
{{- define "magos.apiServiceAccountName" -}}
{{- if .Values.api.serviceAccount.create }}
{{- default (printf "%s-api" (include "magos.fullname" .)) .Values.api.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.api.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "magos.authSecretName" -}}
{{- default (printf "%s-auth" (include "magos.fullname" .)) .Values.auth.secret.name -}}
{{- end -}}


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
