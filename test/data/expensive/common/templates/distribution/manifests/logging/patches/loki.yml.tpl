# Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

{{- $package := index .spec.distribution.modules.logging "loki" -}}

{{- if and (eq .spec.distribution.modules.logging.type "loki") (hasKeyAny $package "resources") }}
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: loki-distributed-ingester
  namespace: logging
spec:
  template:
    spec:
      containers:
      - name: ingester
        resources:
          {{ $package.resources | toYaml | indent 10 | trim }}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: loki-distributed-compactor
  namespace: logging
spec:
  template:
    spec:
      containers:
      - name: compactor
        resources:
          {{ $package.resources | toYaml | indent 10 | trim }}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: loki-distributed-distributor
  namespace: logging
spec:
  template:
    spec:
      containers:
      - name: distributor
        resources:
          {{ $package.resources | toYaml | indent 10 | trim }}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: loki-distributed-gateway
  namespace: logging
spec:
  template:
    spec:
      containers:
      - name: nginx
        resources:
          {{ $package.resources | toYaml | indent 10 | trim }}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: loki-distributed-query-frontend
  namespace: logging
spec:
  template:
    spec:
      containers:
      - name: query-frontend
        resources:
          {{ $package.resources | toYaml | indent 10 | trim }}
{{- end }}
