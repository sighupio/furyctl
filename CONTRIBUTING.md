# Contributing

## Prerequisites

Required go version >= 1.11

To install dependencies and run this package export env `GO111MODULE=on`.
It is required because the usage of `go mod` is opt-in at the time of writing.

## Download dependencies

Execute `go mod download`

## Adding provisioners

The self-service feature of `furyctl` has been designed to make it easily extensible. If you are in charge of adding
a new provisioner for bootstrapping or create a new cluster, read the following guide.

The guide is almost identical between bootstrap and cluster. This guide focus on the `bootstrap` provisioners.
The main differences between bootstrap and cluster provisioners:

- **`data/provisioner`** directory to hold terraform projects:
  - **bootstrap**: `data/provisioners/bootstrap/{provisioner-name}`
  - **cluster**: `data/provisioners/cluster/{provisioner-name}`
- **configuration parser** at `internal/configuration/config.go` has different methods:
  - **bootstrap**: `bootstrapParser`
  - **cluster**: `clusterParser`.
- **provisioner builder** at `internal/provisioners/provisioners.go` has different methods:
  - **bootstrap**: `getBootstrapProvisioner`.
  - **cluster**: `getClusterProvisioner`.
- The `configuration struct` goes
  - bootstrap at: `internal/bootstrap/configuration/{provisioner-name}.go`.
  - cluster: `internal/cluster/configuration/{provisioner-name}.go`.
- The `provisioner implementation` goes
  - **bootstrap** at `internal/bootstrap/provisioners/{provisioner-name}/provisioner.go`
  - **cluster** at `internal/cluster/provisioners/{provisioner-name}/provisioner.go`

### Terraform project

First of all, `furyctl` provisioners are based on terraform code. So you have to have an already
working terraform project.
Place the terraform project in the following `data/provisioners/bootstrap/{provisioner-name}` path.

**Let's use `dummy` as the `provisioner-name`.**

### Configuration parser

Then, create the golang structures that will be used to hold the input variables.
Create the `internal/bootstrap/configuration/dummy.go` file with a `struct` definition:

```golang
package configuration

// Dummy reprensents the input variables of a dummy provisioner.
// THIS IS AN EXAMPLE. Create a struct definition that fits the input variables of your terraform project.
type Dummy struct {
	NetworkCIDR         string   `yaml:"networkCIDR"`
	PublicSubnetsCIDRs  []string `yaml:"publicSubnetsCIDRs"`
	PrivateSubnetsCIDRs []string `yaml:"privateSubnetsCIDRs"`
}
```

Then, add the `Dummy` configuration to the configuration parser in the `internal/configuration/config.go` file
*(new swith option)*:

```diff
--- a/internal/configuration/config.go
+++ b/internal/configuration/config.go
@@ -122,6 +122,15 @@ func bootstrapParser(config *Configuration) (err error) {
                }
                config.Spec = awsSpec
                return nil
+       case provisioner == "dummy":
+               dummySpec := bootstrapcfg.Dummy{}
+               err = yaml.Unmarshal(specBytes, &dummySpec)
+               if err != nil {
+                       log.Errorf("error parsing configuration file: %v", err)
+                       return err
+               }
+               config.Spec = dummySpec
+               return nil
        default:
                log.Error("Error parsing the configuration file. Provisioner not found")
                return errors.New("Bootstrap provisioner not found")
```

The configuration structures that we created is part of the entire configuration file.
It is the structure that gives value to the `spec` base configuration attribute.

### Provisioner interface implementation

Then, create a new directory/package inside `internal/bootstrap/provisioners/` named `dummy`.
Create a golang file: `internal/bootstrap/provisioners/dummy/provisioner.go` then implement the provisioner interface
available at `internal/provisioners/provisioners.go`:

```golang
// Provisioner represents a kubernetes terraform provisioner
type Provisioner interface {
	InitMessage() string // Prints this message after init phase finishes
	UpdateMessage() string // Prints this message after update phase finishes
	DestroyMessage() string // Prints this message after destroy phase finishes

	SetTerraformExecutor(tf *tfexec.Terraform) // Configures a terraform executor once initialized
	TerraformExecutor() (tf *tfexec.Terraform) // Returns the provisioner terraform executor
	TerraformFiles() []string // Returns the list of terraform files that makes the provisioner.

	Enterprise() bool // Indicates if it requires a valid token. Contact sales@sighup.io

	Plan() error // Execute the terraform plan command
	Update() error // Execute the terraform apply command
	Destroy() error // Execute the terraform destroy command

	Box() *packr.Box // Returns a `box` with the binary data of the terraform files.
}
```

*Take a look at the AWS implementation to understand how it works*: `internal/bootstrap/provisioners/aws/provisioner.go`

Once implemented, include it in the available provisioner switch at `internal/provisioners/provisioners.go`

```diff
index 8c5a6d8..a3d6ecf 100644
--- a/internal/provisioners/provisioners.go
+++ b/internal/provisioners/provisioners.go
@@ -11,6 +11,7 @@ import (
        "github.com/gobuffalo/packr/v2"
        "github.com/hashicorp/terraform-exec/tfexec"
        "github.com/sighupio/furyctl/internal/bootstrap/provisioners/aws"
+       "github.com/sighupio/furyctl/internal/bootstrap/provisioners/dummy"
        "github.com/sighupio/furyctl/internal/cluster/provisioners/eks"
        "github.com/sighupio/furyctl/internal/configuration"
        log "github.com/sirupsen/logrus"
@@ -61,6 +62,8 @@ func getBootstrapProvisioner(config configuration.Configuration) (Provisioner, e
        switch {
        case config.Provisioner == "aws":
                return aws.New(&config), nil
+       case config.Provisioner == "dummy":
+               return dummy.New(&config), nil
        default:
                log.Error("Provisioner not found")
                return nil, errors.New("Provisioner not found")
```

Then, build the `furyctl` binary: `make build`, then create a configuration file: `dummy-bootstrap.yml`

```yaml
kind: Bootstrap # Important!
metadata:
  name: my-dummy-demo
provisioner: dummy # Important! This is the provider we have created.
spec: # This is managed by the configuration struct
  networkCIDR: "10.0.0.0/16"
  publicSubnetsCIDRs:
    - "10.0.1.0/24"
    - "10.0.2.0/24"
    - "10.0.3.0/24"
  privateSubnetsCIDRs:
    - "10.0.101.0/24"
    - "10.0.102.0/24"
    - "10.0.103.0/24"
```

All is set to start using the new provisioner:

```bash
$ furyctl bootstrap init -c dummy-bootstrap.yml
```

Enjoy!
