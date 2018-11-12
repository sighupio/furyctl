# Furyctl

## Install
Get the right binary for you in the [latest release](https://git.incubator.sh/sighup/furyctl/tags)

## Usage

```
furyctl
├── install
├── agent
├── configure
│   ├── node
│   ├── etcd
│   └── master
├── backup
│   ├── etcd
│   └── master
└── restore
    ├── etcd
    └── master
```

Write a [`Furyfile.yml`](Furyfile.yml) in the root of your project and then simply run `furyctl`



## Contributing
We still use `go mod` as golang package manager. Once you have that installed you can run `go mod vendor` and `go build` or `go install` should run without problems

# Storage
There is going to be one and only one bucket per cluster. Having multiple clusters saved up in a single bucket can be dangerous

I'll try to describe the structure in the bucket as best as I can imagine

```
S3 bucket
├── etcd
│   ├── node-1
│   │   └── 2018-10-20-23-01-etcd.db
│   ├── node-2
│   └── node-3
├── cluster-backup
│   ├── full-20181002120049
│   │   ├── full-20181002120049.tar.gz
│   │   ├── full-20181002120049-logs.gz
│   │   └── ark-backup.json
│   ├── full-20181003120049
│   │   ├── full-20181003120049.tar.gz
│   │   ├── full-20181003120049-logs.gz
│   │   └── ark-backup.json
│   └── full-20181004120049
│       ├── full-20181004120049.tar.gz
│       ├── full-20181004120049-logs.gz
│       └── ark-backup.json
├── nodes
│   ├── discovery.txt
│   └── token.txt
├── users
│   ├── giacomo.conf
│   ├── jacopo.conf
│   ├── luca.conf
│   ├── philippe.conf
│   └── berat.conf
├── configurations
│   ├── kustomization.yaml
│   ├── audit.yaml
│   ├── nodeSelector.yaml
│   └── kubeadm.yml
└── pki
    ├── etcd
    │   ├── ca.crt
    │   └── ca.key
    ├── master
    │   ├── sa.key
    │   ├── sa.pub
    │   ├── front-proxy-ca.crt
    │   ├── front-proxy-ca.key
    │   ├── ca.crt
    │   └── ca.key
    └── vpn
        ├── ca.crt
        └── ca.key

```

For ARK volume backup using restic backup is necessary a different bucket then this one.

