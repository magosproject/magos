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

{{/*
Create the bundled RustFS resource names.
*/}}
{{- define "magos.rustfsName" -}}
{{- printf "%s-rustfs" (include "magos.fullname" .) -}}
{{- end }}

{{- define "magos.rustfsSecretName" -}}
{{- if eq (include "magos.logsStorageMode" .) "external" -}}
{{- required "logs.storage.external.existingSecret is required when logs.storage.mode=external" .Values.logs.storage.external.existingSecret -}}
{{- else -}}
{{- include "magos.rustfsName" . -}}
{{- end -}}
{{- end }}

{{- define "magos.rustfsAccessKeyKey" -}}
{{- if eq (include "magos.logsStorageMode" .) "external" -}}
{{- default "accessKey" .Values.logs.storage.external.accessKeyKey -}}
{{- else -}}
accessKey
{{- end -}}
{{- end }}

{{- define "magos.rustfsSecretKeyKey" -}}
{{- if eq (include "magos.logsStorageMode" .) "external" -}}
{{- default "secretKey" .Values.logs.storage.external.secretKeyKey -}}
{{- else -}}
secretKey
{{- end -}}
{{- end }}

{{/*
Create the bundled PostgreSQL resource names
*/}}
{{- define "magos.postgresName" -}}
{{- printf "%s-postgres" (include "magos.fullname" .) -}}
{{- end }}

{{- define "magos.postgresHeadlessName" -}}
{{- printf "%s-headless" (include "magos.postgresName" .) -}}
{{- end }}

{{- define "magos.postgresSecretName" -}}
{{- default (include "magos.postgresName" .) .Values.postgres.auth.existingSecret -}}
{{- end }}

{{- define "magos.postgresPasswordKey" -}}
{{- if eq (include "magos.postgresMode" .) "external" -}}
{{- default "password" .Values.postgres.external.passwordKey -}}
{{- else -}}
{{- default "password" .Values.postgres.auth.passwordKey -}}
{{- end -}}
{{- end }}

{{/*
Resolve the selected storage/database wiring modes.
*/}}
{{- define "magos.logsStorageMode" -}}
{{- $mode := default "embedded" .Values.logs.storage.mode -}}
{{- if and (ne $mode "embedded") (ne $mode "external") -}}
{{- fail (printf "logs.storage.mode must be either embedded or external, got %q" $mode) -}}
{{- end -}}
{{- $mode -}}
{{- end }}

{{- define "magos.postgresMode" -}}
{{- $mode := default "embedded" .Values.postgres.mode -}}
{{- if and (ne $mode "embedded") (ne $mode "external") -}}
{{- fail (printf "postgres.mode must be either embedded or external, got %q" $mode) -}}
{{- end -}}
{{- $mode -}}
{{- end }}

{{/*
Environment variables for the run-summary database. These are wired either from
the bundled PostgreSQL deployment or from an external PostgreSQL instance.
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
      name: {{ required "postgres.external.existingSecret is required when postgres.mode=external" .Values.postgres.external.existingSecret }}
      key: {{ include "magos.postgresPasswordKey" . }}
- name: MAGOS_POSTGRES_SSLMODE
  value: {{ .Values.postgres.external.sslMode | quote }}
{{- else }}
- name: MAGOS_POSTGRES_HOST
  value: {{ include "magos.postgresName" . | quote }}
- name: MAGOS_POSTGRES_PORT
  value: {{ .Values.postgres.service.port | quote }}
- name: MAGOS_POSTGRES_DATABASE
  value: {{ .Values.postgres.auth.database | quote }}
- name: MAGOS_POSTGRES_USER
  value: {{ .Values.postgres.auth.username | quote }}
- name: MAGOS_POSTGRES_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ include "magos.postgresSecretName" . }}
      key: {{ include "magos.postgresPasswordKey" . }}
- name: MAGOS_POSTGRES_SSLMODE
  value: {{ .Values.postgres.sslMode | quote }}
{{- end }}
{{- end }}

{{/*
Environment variables for the log store. These are wired either from the
bundled RustFS deployment or from an external S3-compatible backend.
*/}}
{{- define "magos.logstoreEnv" -}}
{{- if eq (include "magos.logsStorageMode" .) "external" }}
- name: MAGOS_LOGS_S3_ENDPOINT
  value: {{ required "logs.storage.external.endpoint is required when logs.storage.mode=external" .Values.logs.storage.external.endpoint | quote }}
{{- else }}
- name: MAGOS_LOGS_S3_ENDPOINT
  value: {{ printf "http://%s:%v" (include "magos.rustfsName" .) .Values.logs.storage.service.port | quote }}
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
{{- end }}
