{{/*
Expand the name of the chart.
*/}}
{{- define "kafka-backup-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "kafka-backup-operator.fullname" -}}
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
{{- define "kafka-backup-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "kafka-backup-operator.labels" -}}
helm.sh/chart: {{ include "kafka-backup-operator.chart" . }}
{{ include "kafka-backup-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "kafka-backup-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kafka-backup-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "kafka-backup-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "kafka-backup-operator.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the cluster role to use
*/}}
{{- define "kafka-backup-operator.clusterRoleName" -}}
{{- include "kafka-backup-operator.fullname" . }}-manager-role
{{- end }}

{{/*
Create the name of the cluster role binding to use
*/}}
{{- define "kafka-backup-operator.clusterRoleBindingName" -}}
{{- include "kafka-backup-operator.fullname" . }}-manager-rolebinding
{{- end }}

{{/*
Create the name of the metrics service
*/}}
{{- define "kafka-backup-operator.metricsServiceName" -}}
{{- include "kafka-backup-operator.fullname" . }}-metrics
{{- end }}

{{/*
Create the name of the webhook service
*/}}
{{- define "kafka-backup-operator.webhookServiceName" -}}
{{- include "kafka-backup-operator.fullname" . }}-webhook-service
{{- end }}