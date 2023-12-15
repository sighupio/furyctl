# Copyright (c) 2017-present SIGHUP s.r.l All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

---
apiVersion: kfd.sighup.io/v1alpha2
kind: OnPremises
metadata:
  name: __CLUSTER_NAME__
spec:
  distributionVersion: v1.26.3
  kubernetes:
    pkiFolder: ./infra/secrets/pki
    ssh:
      username: ubuntu
      keyPath: ./infra/secrets/ssh-private-key.pem
    dnsZone: __CLUSTER_NAME__.internal
    controlPlaneAddress: __CONTROL_PLANE_IP__:6443
    podCidr: 172.16.128.0/17
    svcCidr: 172.16.0.0/17
    loadBalancers:
      enabled: false
      hosts:
        - name: haproxy1
          ip: 0.0.0.0
      keepalived:
        enabled: false
        interface: eth1
        ip: 192.168.1.201/24
        virtualRouterId: "201"
        passphrase: "123aaaccc321"
      stats:
        username: admin
        password: password
    masters:
      hosts:
        - name: master1
          ip: __CONTROL_PLANE_IP__
    nodes:
      - name: worker
        hosts:
          - name: worker1
            ip: __NODE_1_IP__
          - name: worker2
            ip: __NODE_2_IP__
          - name: worker3
            ip: __NODE_3_IP__
  distribution:
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
        certManager:
          clusterIssuer:
            name: letsencrypt-fury
            email: engineering+fury-distribution@sighup.io
            type: http01
      logging:
        type: loki
      networking:
        type: calico
      policy:
        type: gatekeeper
      dr:
        type: on-premises
        velero: {}
