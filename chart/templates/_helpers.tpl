{{- define "kuberex.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | quote }}
{{ include "kuberex.selector" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "kuberex.selector" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "kuberex.serviceAccountName" -}}
{{ .Values.serviceAccount.name | default .Release.Name }}
{{- end }}

{{- /* kuberex.accountSecretVolume names the volume of Secret (index 1) for account (index 0); a hash keeps any pair a valid, distinct volume name. */ -}}
{{- define "kuberex.accountSecretVolume" -}}
{{ printf "secret-%s" (printf "%s/%s" (index . 0) (index . 1) | sha256sum | trunc 12) }}
{{- end }}
