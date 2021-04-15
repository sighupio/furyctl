# LGTM (Looks Good To Me) (TODO)

- [ ] Giovanni Laieta
- [ ] Ramiro Algozino
- [ ] Luca Novara
- [ ] Luca Zecca

# Status+

_Draft_

# Description+

Integrate vShpere as provisioner of nodes in `furyctl`. Due to (TODO:
list limitations, ask Angel) we will only support the creation of
clusters (`furyctl cluster`) assuming an already working
infrastructure (`furyctl bootstrap`) that in this case must be done
manually.

# Problem+

Being able to create Fury clusters using vSphere node provider.

# Solution+

## Context+

... TODO

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

## Constraints+ (TODO)

- It's an enterprise feature so the repository should be kept private
  to SIGHUP organization.
- Must support VM images.
- Must support customization hook for VMs that can be used during
  installation.

## Risks+

- Requirements for VM images could be too strict (the customization
  hook can be used to reduce the risk starting from a general golden
  image).
- Compatibility between Kubernetes (of version X) and vSphere (of
  version Y).

## Considered Alternatives+

Not applicable, the only alternative is to do everything by hand.

## Deliverables+

- Provisioner repository (TODO: link)
- Furyctl that will be able to use the new provisioner.
- An Ubuntu golden image (TODO: link)
- An Oracle Linux 7.0 golden image (TODO: link)
- Documention (see below)

## Infrastructure Requirements+

- vSphere version ??? (TODO: Ask Angel)
- We will not consider the storage integration as part of the problem
  because vSphere it will not be compatible with Kubernetes ...(TODO:
  ask Angel)

## Testing+

- An end2end test for this provisioner will be integrated in the suite
  of end2end tests for other provisioners (TODO: link)

## Deploy/Rollout+

...

## Metrics

...

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

...

## What We Learned+

...

## Left For Later+

- Handle lifecycle of vShper nodes (TODO: ask Angel)
- Expose the ability to run customization script in `furyctl`
  configuration file so that it will be available for all
  provisioners.
