# Migration path

Having a project with a terraform executor defined prior to != 0.15.4

## I am in 0.12.X

> The following provisioners could be affected. It depends on your `executor.version` or `executor.path` definition.
> aws bootstrap
> eks cluster


If you already have deployed a `{cluster, bootrap}` using furyctl version `< 0.6` and you used an executor version
0.12.X like this one:

```yaml
kind: Bootstrap
metadata:
  name: demo
provisioner: aws
executor:
  version: 0.12.29
  state:
    backend: s3
    config:
      bucket: fury-testing
      key: furyctl-upgrade-test
      region: eu-west-1
spec:
  networkCIDR: 10.0.0.0/16
  publicSubnetsCIDRs:
  - 10.0.20.0/24
  - 10.0.30.0/24
  privateSubnetsCIDRs:
  - 10.0.182.0/24
  - 10.0.192.0/24
  vpn:
    instances: 1
    instanceType: t3.micro
    operatorName: sighup
    subnetCIDR: 172.16.0.0/16
    sshUsers:
    - angelbarrera92
    operatorCIDRs:
    - 54.27.48.48/32
```

You have to manually migrate the terraform project to 0.13.X by downloading a terraform 0.13.X version. Then:

```bash
$ cd bootstrap/
$ terraenv terraform install 0.13.7
Downloading terraform 0.13.7 from https://releases.hashicorp.com/terraform/0.13.7/terraform_0.13.7_darwin_amd64.zip
terraform version is set to 0.13.7
$ terraform version
Terraform v0.13.7

Your version of Terraform is out of date! The latest version
is 0.15.4. You can update by downloading from https://www.terraform.io/downloads.html
$ terraform 0.13upgrade -yes

Upgrade complete!

Use your version control system to review the proposed changes, make any
necessary adjustments, and then commit.
$ cd ..
```

At this point, you have to modify the `bootstrap.yml` file to change the `executor.version`:

```yaml
kind: Bootstrap
metadata:
  name: demo
provisioner: aws
executor:
  version: 0.13.7 # Place here the latest 0.13 Version
  state:
    backend: s3
    config:
      bucket: fury-testing
      key: furyctl-upgrade-test
      region: eu-west-1
spec:
  ...
```

Then run:

```bash
$ furyctl bootstrap apply --config bootstrap.yml --reconfigure
```

**`WARNING`** Don't forget the `--reconfigure` flag.

After the command finishes, **download the new `furyctl` version (0.6.0).**
This release deprecates the `executor.version` and `executor.path`.
Then modify again the `bootstrap.yml` file in order to remove the `executor.version`:

```yaml
kind: Bootstrap
metadata:
  name: demo
provisioner: aws
executor:
  state:
    backend: s3
    config:
      bucket: fury-testing
      key: furyctl-upgrade-test
      region: eu-west-1
spec:
  ...
```

Finally, run:

```bash
$ furyctl bootstrap init --reset --config bootstrap.yml --reconfigure
# IN CASE YOU ARE USING THE CLUSTER vSphere PROVISIONER, READ THE NOTE AT THE END OF THIS DOCUMENT (*)
$ furyctl bootstrap apply --config bootstrap.yml
```


## I am in 0.13.X or 0.14.X

> The following provisioners could be affected. It depends on your `executor.version` or `executor.path` definition.
> aws bootstrap
> eks cluster
> gcp bootstrap
> gke cluster


If you already have deployed a `{cluster, bootrap}` using furyctl version `< 0.6` and you used an executor version
0.1{3,4}.X like this one:

```yaml
kind: Bootstrap
metadata:
  name: demo
provisioner: gcp
executor:
  version: 0.13.7
  state:
    backend: s3
    config:
      bucket: fury-testing
      key: furyctl-upgrade-test
      region: eu-west-1
spec:
  publicSubnetsCIDRs:  
    - 10.0.1.0/24
  privateSubnetsCIDRs: 
    - 10.0.101.0/24
  clusterNetwork:
    subnetworkCIDR: 10.1.0.0/16
    podSubnetworkCIDR: 10.2.0.0/16
    serviceSubnetworkCIDR: 10.3.0.0/16
  vpn:
    instances: 1
    subnetCIDR: 192.168.200.0/24
    sshUsers:
      - angelbarrera92
```

**Download the new `furyctl` version (0.6.0).**
This release deprecates the `executor.version` and `executor.path`.
Then modify again the `bootstrap.yml` file in order to remove the `executor.version`:

```yaml
kind: Bootstrap
metadata:
  name: demo
provisioner: gcp
executor:
  state:
    backend: s3
    config:
      bucket: fury-testing
      key: furyctl-upgrade-test
      region: eu-west-1
spec:
  ...
```

Finally, run:

```bash
$ furyctl bootstrap init --reset --config bootstrap.yml --reconfigure
# IN CASE YOU ARE USING THE CLUSTER vSphere PROVISIONER, READ THE NOTE AT THE END OF THIS DOCUMENT (*)
$ furyctl bootstrap apply --config bootstrap.yml
```

**`WARNING`** Don't forget the `--reconfigure` flag.


# IMPORTANT Notes

- **(*)**: By running `furyctl cluster init --reset --config cluster.yml --reconfigure` with the vSphere provisioner,
it recreates the PKI of the cluster. Make sure you backup it *(or you have everything versioned in git)* before
run this command. After run `init` and before run `apply`, restore the PKI to don't break the cluster.
