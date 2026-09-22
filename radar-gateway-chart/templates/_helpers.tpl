{{- define "radar-gateway.name" -}}
{{- default .Chart.Name .Values.global.name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "radar-gateway.fullname" -}}
{{- include "radar-gateway.name" . }}
{{- end }}

{{- define "radar-gateway.labels" -}}
app.kubernetes.io/name: {{ include "radar-gateway.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
