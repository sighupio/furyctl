{{- $dnsZone := .spec.kubernetes.dnsZone -}}

global
    daemon
    maxconn 20480

defaults
    mode tcp
    timeout connect 5s
    timeout client 50s
    timeout server 50s

listen  stats
    bind *:1936
    mode            http
    log             global

    maxconn 10

    timeout client      100s
    timeout server      100s
    timeout connect     100s
    timeout queue       100s

    stats enable
    stats uri /stats
    stats hide-version
    stats refresh 30s
    stats show-node
    stats auth {{ .spec.kubernetes.loadBalancers.stats.username }}:{{ .spec.kubernetes.loadBalancers.stats.password }}

frontend control-plane
    mode tcp
    bind *:6443 alpn h2,http/1.1
    default_backend masters

backend masters
    option httpchk GET /healthz
    balance roundrobin
    {{- range $h := .spec.kubernetes.masters.hosts }}
    server {{ $h.name }}.{{ $dnsZone }} {{ $h.ip }}:6443 maxconn 256 check check-ssl ca-file /etc/ssl/certs/kubernetes.crt
    {{- end }}

{{- if index .spec.kubernetes.loadBalancers "additionalConfig" }}
{{ .spec.kubernetes.loadBalancers.additionalConfig }}
{{- end }}
