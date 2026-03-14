{{- define "web.name" -}}
{{- default .Chart.Name .Values.fullNameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}
