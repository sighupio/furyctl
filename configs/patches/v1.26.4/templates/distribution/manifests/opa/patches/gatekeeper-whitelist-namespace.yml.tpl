# Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

# THIS FILE HAS BEEN PATCHED BY FURYCTL TO ENSURE BACKWARDS COMPATIBILITY.
# IT IS NOT THE ORIGINAL FILE FOUND IN THE DISTRIBUTION REPOSITORY.

---
{{- if .spec.distribution.modules.policy.gatekeeper.additionalExcludedNamespaces }}
{{- range .spec.distribution.modules.policy.gatekeeper.additionalExcludedNamespaces }}
- op: "add"
  path: "/spec/match/0/excludedNamespaces/-"
  value: {{ . }}
{{- end }}
{{- end }}
- op: "add"
  path: "/spec/match/0/excludedNamespaces/-"
  value: "tracing"
