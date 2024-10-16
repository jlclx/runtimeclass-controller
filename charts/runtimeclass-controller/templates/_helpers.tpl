{{- define "runtimeclass-controller.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "runtimeclass-controller.fullname" -}}
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

{{- define "runtimeclass-controller.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "runtimeclass-controller.labels" -}}
helm.sh/chart: {{ include "runtimeclass-controller.chart" . }}
{{ include "runtimeclass-controller.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "runtimeclass-controller.selectorLabels" -}}
app.kubernetes.io/name: {{ include "runtimeclass-controller.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "runtimeclass-controller.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "runtimeclass-controller.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "runtimeclass-controller.certificateName" -}}
{{- if .Values.webhook.tls.certificateSecret }}
{{- .Values.webhook.tls.certificateSecret }}
{{- else }}
{{- include "runtimeclass-controller.fullname" . }}-tls
{{- end }}
{{- end }}