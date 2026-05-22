{{- define "cloudflare-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "cloudflare-operator.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}

{{- define "cloudflare-operator.namespace" -}}
{{- .Values.namespace }}
{{- end }}

{{- define "cloudflare-operator.serviceAccountName" -}}
{{- .Values.serviceAccountName }}
{{- end }}

{{- define "cloudflare-operator.labels" -}}
app.kubernetes.io/name: {{ include "cloudflare-operator.name" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}
