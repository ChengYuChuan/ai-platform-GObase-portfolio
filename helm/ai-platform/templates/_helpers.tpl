{{/*
Expand the name of the chart.
*/}}
{{- define "ai-platform.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "ai-platform.fullname" -}}
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
{{- define "ai-platform.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "ai-platform.labels" -}}
helm.sh/chart: {{ include "ai-platform.chart" . }}
{{ include "ai-platform.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "ai-platform.selectorLabels" -}}
app.kubernetes.io/name: {{ include "ai-platform.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "ai-platform.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "ai-platform.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Gateway labels
*/}}
{{- define "ai-platform.gateway.labels" -}}
{{ include "ai-platform.labels" . }}
app.kubernetes.io/component: gateway
{{- end }}

{{- define "ai-platform.gateway.selectorLabels" -}}
{{ include "ai-platform.selectorLabels" . }}
app.kubernetes.io/component: gateway
{{- end }}

{{/*
RAG Service labels
*/}}
{{- define "ai-platform.ragService.labels" -}}
{{ include "ai-platform.labels" . }}
app.kubernetes.io/component: rag-service
{{- end }}

{{- define "ai-platform.ragService.selectorLabels" -}}
{{ include "ai-platform.selectorLabels" . }}
app.kubernetes.io/component: rag-service
{{- end }}

{{/*
Dashboard labels
*/}}
{{- define "ai-platform.dashboard.labels" -}}
{{ include "ai-platform.labels" . }}
app.kubernetes.io/component: dashboard
{{- end }}

{{- define "ai-platform.dashboard.selectorLabels" -}}
{{ include "ai-platform.selectorLabels" . }}
app.kubernetes.io/component: dashboard
{{- end }}

{{/*
Qdrant labels
*/}}
{{- define "ai-platform.qdrant.labels" -}}
{{ include "ai-platform.labels" . }}
app.kubernetes.io/component: qdrant
{{- end }}

{{- define "ai-platform.qdrant.selectorLabels" -}}
{{ include "ai-platform.selectorLabels" . }}
app.kubernetes.io/component: qdrant
{{- end }}
