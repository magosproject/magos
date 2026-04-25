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
S3 endpoint for the log store.
Defaults to the in-cluster RustFS service when rustfs.enabled is true.
*/}}
{{- define "magos.logstoreEndpoint" -}}
{{- if .Values.logs.s3.endpoint -}}
{{- .Values.logs.s3.endpoint -}}
{{- else if .Values.rustfs.enabled -}}
{{- printf "http://%s-rustfs:%v" (include "magos.fullname" .) .Values.rustfs.service.port -}}
{{- else -}}
{{- fail "logs.s3.endpoint must be set when rustfs.enabled is false" -}}
{{- end -}}
{{- end }}

{{/*
Name of the Secret that holds the S3 credentials for the log store.
Defaults to the bundled RustFS secret when rustfs.enabled is true.
*/}}
{{- define "magos.logstoreSecretName" -}}
{{- if .Values.logs.s3.accessKeySecretRef.name -}}
{{- .Values.logs.s3.accessKeySecretRef.name -}}
{{- else if .Values.rustfs.enabled -}}
{{- printf "%s-rustfs" (include "magos.fullname" .) -}}
{{- else -}}
{{- fail "logs.s3.accessKeySecretRef.name must be set when rustfs.enabled is false" -}}
{{- end -}}
{{- end }}

{{/*
Environment variables for the log store. Renders MAGOS_LOGS_ENABLED unconditionally;
all S3 settings are only rendered when logs.enabled is true.
*/}}
{{- define "magos.logstoreEnv" -}}
- name: MAGOS_LOGS_ENABLED
  value: {{ .Values.logs.enabled | quote }}
{{- if .Values.logs.enabled }}
- name: MAGOS_LOGS_S3_BUCKET
  value: {{ .Values.logs.s3.bucket | quote }}
- name: MAGOS_LOGS_S3_REGION
  value: {{ .Values.logs.s3.region | quote }}
- name: MAGOS_LOGS_S3_ENDPOINT
  value: {{ include "magos.logstoreEndpoint" . | quote }}
- name: MAGOS_LOGS_S3_FORCE_PATH_STYLE
  value: {{ .Values.logs.s3.forcePathStyle | quote }}
- name: MAGOS_LOGS_S3_INSECURE_SKIP_TLS_VERIFY
  value: {{ .Values.logs.s3.insecureSkipTLSVerify | quote }}
- name: MAGOS_LOGS_S3_ACCESS_KEY_ID
  valueFrom:
    secretKeyRef:
      name: {{ include "magos.logstoreSecretName" . }}
      key: {{ .Values.logs.s3.accessKeySecretRef.key }}
- name: MAGOS_LOGS_S3_SECRET_ACCESS_KEY
  valueFrom:
    secretKeyRef:
      name: {{ include "magos.logstoreSecretName" . }}
      key: {{ .Values.logs.s3.secretKeySecretRef.key }}
{{- end }}
{{- end }}
