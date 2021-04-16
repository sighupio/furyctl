# LGTM (Looks Good To Me) (TODO)

- [ ] Giovanni Laieta
- [ ] Ramiro Algozino
- [ ] Luca Novara
- [ ] Luca Zecca

# Status+

_Draft_

# Description+

Integrate vShpere as provisioner of clusters in `furyctl`.
Due to budget (time) contraints we will only support the creation of
clusters (`furyctl cluster`) assuming an already working
infrastructure (`furyctl bootstrap`) that in this case must be done
manually.

# Problem+

Being able to create Fury clusters using vSphere node provider.

# Solution+

## Context+

We currently use a set of roles to deploy infrastructure on top of any on-premise
servers. We can use these resources to deploy the cluster on top of VM created with
terraform. These ansible roles do not allow us to manage the day two operations as we
are currently managing it with terraform on the cloud-installers. So we are going to
start supporting the creation of clusters on vpshere along some basic day two
operations like adding or scaling node pools.

## Implementation+

### Plan

- Create a workable Oracle Linux image.
- Take the work done (Terraform, Ansible, ...) on vShpere lab with
  Ubuntu instances and make it work for Oracle Linux instances.
- Make the above work general and configurable (ex. number of worker
  nodes, labeling, etc...).
- Make it run with `furyctl cluster` command.
- Add end2end tests.
- Write documentation.

## Constraints+

- It's an enterprise feature so the provisioner should be kept private
  to SIGHUP organization.
- Must support two kinds of VM images: Ubuntu 20 and Oracle Linux 7.9.
- Must support customization hook for VMs that can be used during
  installation.
  - We decided to enable the user to provide an input variable per node
  to specify a local script path. It will be executed doring first boot.
- As mentioned above, the implementation will allow to safely create
clusters while the day two operations must be performed as usual.

## Risks+

- Requirements for VM images could be too strict (the customization
  hook can be used to reduce the risk starting from a general golden
  image).
  - We can provide the customer the recipe to bake a VM template for
  Ubuntu 20 and Oracle Linux 7.9
- Compatibility between Kubernetes (of version X) and vSphere (of
  version Y).
- vSphere version is a risk. We only have one invironment were to test
the development.


## Considered Alternatives+

Not applicable, the only alternative is to do everything by hand.

## Deliverables+

- [Provisioner repository](https://github.com/sighupio/furyctl-provisioners)
- Furyctl that will be able to use the new provisioner.
- An Ubuntu golden image recipe.
- An Oracle Linux 7.0 golden image recipe.
- Documention (see below)

## Infrastructure Requirements+

- vSphere version 6.5 ([terraform requirement](https://github.com/hashicorp/terraform-provider-vsphere))
  - vSphere version 6.7 required to install the CSI: https://github.com/kubernetes-sigs/vsphere-csi-driver
  - vSphere version 6.7 required to install the CPI: https://cloud-provider-vsphere.sigs.k8s.io/tutorials/kubernetes-on-vsphere-with-kubeadm.html
- The vSphere integration has to be performed after cluster creation.
- We will not consider the storage integration as part of the problem
  because vSphere it will not be compatible with Kubernetes...

## Testing+

- An end2end test for this provisioner will be integrated in the suite
  of end2end tests for other provisioners.

## Deploy/Rollout+

- The deployment is going to be managed as usual with an automated pipeline.

## Metrics

- Add a new id to the current metrics to identify vSphere provisions.
- Manage the same way we are managing the other provisioners.

## Documentation+

- Documentation for the customers (on constraints, how to use it)
- Documentation for the technical end user (add to Fury documentation,
  don't forget the compatibility matrix with vShpere versions)
- Usage demo
- Sales proposition/values?

## Security

...

## Privacy

...

## Scaling

...

# After+

We have to consider adding day two operations like:
- Certificate Renewal
- Cluster Upgrade

## What We Learned+

...

## Left For Later+

- Handle lifecycle of vShper nodes (TODO: ask Angel)
- Expose the ability to run customization script in `furyctl`
  configuration file so that it will be available for all
  provisioners.
