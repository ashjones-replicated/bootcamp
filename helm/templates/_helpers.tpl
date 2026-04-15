{{/*
Expand the name of the chart.
*/}}
{{- define "bootcamp.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "bootcamp.fullname" -}}
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
Common labels
*/}}
{{- define "bootcamp.labels" -}}
helm.sh/chart: {{ include "bootcamp.name" . }}-{{ .Chart.Version }}
{{ include "bootcamp.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "bootcamp.selectorLabels" -}}
app.kubernetes.io/name: {{ include "bootcamp.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
CloudNativePG cluster name
*/}}
{{- define "bootcamp.pgClusterName" -}}
{{- include "bootcamp.fullname" . }}-pg
{{- end }}

{{/*
Upload service resource name
*/}}
{{- define "bootcamp.uploadName" -}}
{{- include "bootcamp.fullname" . }}-upload
{{- end }}

{{/*
Upload service selector labels
*/}}
{{- define "bootcamp.uploadSelectorLabels" -}}
app.kubernetes.io/name: {{ include "bootcamp.name" . }}-upload
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Return an image reference, optionally routed through the Replicated proxy.
Args: dict "root" $ "image" "registry/repo:tag"
When global.replicated.proxy.registryDomain is set, prefixes the image with
  <domain>/proxy/bootcamp/
Otherwise returns the image reference unchanged.
*/}}
{{- define "bootcamp.proxyImage" -}}
{{- $domain := .root.Values.global.replicated.proxy.registryDomain -}}
{{- if $domain -}}
{{- printf "%s/proxy/bootcamp/%s" $domain .image -}}
{{- else -}}
{{- .image -}}
{{- end -}}
{{- end }}
