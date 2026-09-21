{{- define "radar-agent.name" -}}
{{- default .Chart.Name .Values.global.name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- define "radar-agent.fullname" -}}
{{- include "radar-agent.name" . -}}
{{- end -}}
{{- define "radar-agent.labels" -}}
app.kubernetes.io/name: {{ include "radar-agent.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}
