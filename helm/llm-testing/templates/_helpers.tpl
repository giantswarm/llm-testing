{{/*
Expand the name of the chart.
*/}}
{{- define "llm-testing.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "llm-testing.fullname" -}}
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
Common labels.
*/}}
{{- define "llm-testing.labels" -}}
helm.sh/chart: {{ include "llm-testing.name" . }}-{{ .Chart.Version | replace "+" "_" }}
{{ include "llm-testing.selectorLabels" . }}
app.kubernetes.io/version: {{ .Values.image.tag | default .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
application.giantswarm.io/team: {{ index .Chart.Annotations "application.giantswarm.io/team" | quote }}
{{- end }}

{{/*
Selector labels.
*/}}
{{- define "llm-testing.selectorLabels" -}}
app.kubernetes.io/name: {{ include "llm-testing.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Service account name.
*/}}
{{- define "llm-testing.serviceAccountName" -}}
{{- if .Values.serviceAccount.name }}
{{- .Values.serviceAccount.name }}
{{- else }}
{{- include "llm-testing.fullname" . }}
{{- end }}
{{- end }}

{{/*
KServe namespace.
*/}}
{{- define "llm-testing.kserveNamespace" -}}
{{- if .Values.kserve.namespace }}
{{- .Values.kserve.namespace }}
{{- else }}
{{- .Release.Namespace }}
{{- end }}
{{- end }}
