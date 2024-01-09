# Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# THIS FILE HAS BEEN PATCHED BY FURYCTL TO ENSURE BACKWARDS COMPATIBILITY.
# IT IS NOT THE ORIGINAL FILE FOUND IN THE DISTRIBUTION REPOSITORY.

---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
{{- if eq .spec.distribution.modules.policy.type "gatekeeper" }}
  - {{ print "../" .spec.distribution.common.relativeVendorPath "/modules/opa/katalog/gatekeeper/core" }}
  - {{ print "../" .spec.distribution.common.relativeVendorPath "/modules/opa/katalog/gatekeeper/gpm" }}
  - {{ print "../" .spec.distribution.common.relativeVendorPath "/modules/opa/katalog/gatekeeper/rules" }}
  - {{ print "../" .spec.distribution.common.relativeVendorPath "/modules/opa/katalog/gatekeeper/monitoring" }}
{{- if ne .spec.distribution.modules.ingress.nginx.type "none" }}
  - resources/ingress-infra.yml
{{- end }}
{{- end }}
{{- if eq .spec.distribution.modules.policy.type "kyverno" }}
  - {{ print "../" .spec.distribution.common.relativeVendorPath "/modules/opa/katalog/kyverno" }}
{{- end }}

patchesStrategicMerge:
  - patches/infra-nodes.yml
{{- if .spec.distribution.modules.policy.kyverno.additionalExcludedNamespaces }}
  - patches/kyverno-whitelist-namespace.yml
{{- end }}

patchesJson6902:
  - target:
      group: config.gatekeeper.sh
      version: v1alpha1
      kind: Config
      name: config
    path: patches/gatekeeper-whitelist-namespace.yml
