# Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

---
apiVersion: kfd.sighup.io/v1alpha2
kind: KFDDistribution
metadata:
  name: __CLUSTER_NAME__
spec:
  distributionVersion: v1.27.5
  distribution:
    kubeconfig: __KUBECONFIG__
    modules:
      auth:
        provider:
          type: none
      ingress:
        baseDomain: internal.example.dev
        nginx:
          type: none
          tls:
            provider: none
      logging:
        type: none
      networking:
        type: calico
      policy:
        type: none
      dr:
        type: on-premises
        velero: {}
