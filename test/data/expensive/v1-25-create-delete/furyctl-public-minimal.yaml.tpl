# This file represents a minimal config for a public EKS cluster with 1.25.9 version
# and the least amount of modules enabled.
---
apiVersion: kfd.sighup.io/v1alpha2
kind: EKSCluster
metadata:
  name: furytest-__ID__
spec:
  distributionVersion: v1.25.9
  toolsConfiguration:
    terraform:
      state:
        s3:
          bucketName: furytest-__ID__
          keyPrefix: swe-dev/furytest-__ID__
          region: eu-west-1
  region: eu-west-1
  tags:
    env: "swe-dev"
    k8s: "furytest-__ID__"
    githubOrg: "sighupio"
    githubRepo: "product-management"
    githubIssue: "193"
  infrastructure:
    vpc:
      network:
        cidr: 10.10.0.0/16
        subnetsCidrs:
          private:
            - 10.10.0.0/20
            - 10.10.16.0/20
            - 10.10.32.0/20
            - 10.10.48.0/20
          public:
            - 10.10.192.0/24
            - 10.10.193.0/24
            - 10.10.194.0/24
  kubernetes:
    apiServer:
      privateAccess: false
      publicAccess: true
      privateAccessCidrs: ["0.0.0.0/0"]
      publicAccessCidrs: ["0.0.0.0/0"]
    nodeAllowedSshPublicKey: "{file:///Users/omissis/Work/Sighup/Repositories/fury/tests/product-management-193/tests/id_ed25519.pub}"
    nodePoolsLaunchKind: "launch_templates"
    logRetentionDays: 1
    nodePools:
      - name: workers
        size:
          min: 1
          max: 2
        instance:
          type: t3.xlarge
          spot: true
          volumeSize: 50
        tags:
          k8s.io/cluster-autoscaler/node-template/label/nodepool: "workers"
          k8s.io/cluster-autoscaler/node-template/label/node.kubernetes.io/role: "workers"
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
            email: engineering@sighup.io
            type: http01
        dns:
          public:
            name: "example.dev"
            create: true
          private:
            name: "internal.example.dev"
            create: true
      logging:
        type: none
      policy:
        type: none
      dr:
        type: none
