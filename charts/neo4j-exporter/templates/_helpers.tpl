{{/*
Expand the name of the chart.
*/}}
{{- define "neo4j-exporter.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Fully qualified app name.
*/}}
{{- define "neo4j-exporter.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Chart name + version, used in labels.
*/}}
{{- define "neo4j-exporter.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels applied to every resource.
*/}}
{{- define "neo4j-exporter.labels" -}}
helm.sh/chart: {{ include "neo4j-exporter.chart" . }}
{{ include "neo4j-exporter.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: neo4j-exporter
{{- with .Values.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end -}}

{{/*
Selector labels — stable across upgrades, do NOT include version.
*/}}
{{- define "neo4j-exporter.selectorLabels" -}}
app.kubernetes.io/name: {{ include "neo4j-exporter.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Common annotations applied to every resource.
*/}}
{{- define "neo4j-exporter.annotations" -}}
{{- with .Values.commonAnnotations -}}
{{ toYaml . }}
{{- end -}}
{{- end -}}

{{/*
ServiceAccount name to use.
*/}}
{{- define "neo4j-exporter.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "neo4j-exporter.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{/*
Secret name resolution.
- If auth.existingSecret is set, use that.
- Otherwise, the chart-rendered Secret has the same name as the fullname.
*/}}
{{- define "neo4j-exporter.secretName" -}}
{{- if .Values.auth.existingSecret -}}
{{- .Values.auth.existingSecret -}}
{{- else -}}
{{- include "neo4j-exporter.fullname" . -}}
{{- end -}}
{{- end -}}

{{/*
Image reference. Uses .Chart.AppVersion when image.tag is empty.
*/}}
{{- define "neo4j-exporter.image" -}}
{{- $tag := default .Chart.AppVersion .Values.image.tag -}}
{{- printf "%s:%s" .Values.image.repository $tag -}}
{{- end -}}

{{/*
Decide whether to render a PodDisruptionBudget.
- explicit true  → render
- explicit false → skip
- null (default) → render iff replicaCount > 1 OR autoscaling is enabled
*/}}
{{- define "neo4j-exporter.pdbEnabled" -}}
{{- $explicit := .Values.podDisruptionBudget.enabled -}}
{{- if eq (kindOf $explicit) "bool" -}}
{{- $explicit -}}
{{- else -}}
{{- if or (gt (int .Values.replicaCount) 1) .Values.autoscaling.enabled -}}true{{- else -}}false{{- end -}}
{{- end -}}
{{- end -}}
