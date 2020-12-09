# LGTM (Looks Good To Me)

- [x] Gabriele Lana
- [x] Jacopo Nardiello
- [ ] Gabriele Lana
- [ ] Luca Novara
- [ ] Philippe Scorsolini

> If you think someone else can review this document, feel free to add them into the list.

# Description+

The main goal of this document is to define the **MVP** of the self-service cluster creation feature for the `furyctl`
binary. Currently `furyctl` is underused, it is used to vendor the artifacts specified in a `Furyctl.yml` and
also to download the right `Furyctl.yml` plus `kustomization.yml` files while running the `furyctl init`
command *(for the distribution)*.

It is time to extend its functionalities to provide a way to self-provision Kubernetes clusters.

# Problem+

The main reason to start this feature is to make it simpler to deploy a production-grade cluster from a SIGHUP 
well-known command-line interface (CLI) tool: `furyctl`.

It is not only about deploying (one shoot) Kubernetes Clusters, it has to be maintained during the time, so any
kind of action has to be done via `furyctl`.

Even being open source, SIGHUP Fury stack is not easy to use by someone on the internet *(aka community)*.
This situation limits the target of possible clients as no one can try Fury Clusters before considering hiring us.

# Solution+

## Context+

We already did a POC in a separate binary. The idea is to reuse as much as possible our SIGHUP current stack
(terraform, go...) to create a subcommand in the `furyctl` CLI.

## Implementation+

We have to implement the `cluster` subcommand under the `furyctl` binary to be able to manage the complete lifecycle
of an EKS cluster using the current [SIGHUP EKS Cloud installer](https://github.com/sighupio/fury-eks-installer).

The implementation will:

- (Optional) Create a VPC with a VPN Host. The command will be something like: `furyctl bootstrap {init,update,destroy} --config bootstrap.yml`
  - The VPN server has to provide auto recovery in case an outage happens. TL;DR use `furyagent` to init
  - Then, to create a production-grade cluster, the operator has to connect to the VPN server.
- Create a cluster in a user-defined network. As the first implementation is an EKS private cluster, a pre-requirement will be to stay connected in the VPN created by the bootstrap stage.
  - The provisioner has to warn the operator if it needs to be connected to a VPN or deploy the cluster using a bastion host. (It has to have connectivity to the end cluster)


### Bootstrap

The bootstrap has to initialize a network and deploy a VPN host in the public subnet as we are going to deploy only private clusters
> Nice link: https://learn.hashicorp.com/tutorials/terraform/eks

The VPN host has to use our `furyagent` functionalities to:
- Init the configuration (both VPN and ssh users).
- Configure the VPN software.
- Configure the ssh system user.


A warning has to be raised if the terraform state configuration is a file.

### Cluster

The cluster has to be provided through the bastion host or using a previously configured VPN. One of both alternatives
it's required because some clusters providers require to have connectivity to the API server (private, not public-facing)

A warning has to be raised if the terraform state configuration is a file.

## Constraints+

- The owner of the task has limited experience developing golang code.
- It has to be integrated within `furyctl` and use `furyagent`.

## Risks+

The possibility of failing during the implementation is there:

- We could not manage the complete lifecycle
- We could end up with a tool that is not used by anyone in the company/community


## Considered Alternatives+

We found multiple alternatives; some of them got a try:

- [cluster-api](https://github.com/kubernetes-sigs/cluster-api): We thought about using it. We found some stoppers here:
    - It is super tricky, and the learning curve is too much
    - In the cluster-api docs we can read an ample warning:
        - Cluster API is still in the prototype stage while we get feedback on the API types themselves. All the code here is to experiment with the API and demo its abilities, in order to drive more technical feedback to the API design. Because of this, all the codebases is rapidly changing.
        - Source: https://github.com/kubernetes-sigs/cluster-api#what-is-the-cluster-api
    - If we see other competitors, like RedHat OCP4 installer, we can see they are using both options, cluster-api and terraform bundled in the CLI.
- [Terranova](https://github.com/johandry/terranova): looks like an excellent project, but there were a couple of problems with it:
    - Providers versions have to be statically defined; in addition to it, not all providers’ versions are currently compatible.
    - It seems to be maintained by a single person.
- [Terraform](https://github.com/hashicorp/terraform): Using the terraform code natively could be a good idea. For sure more complicated than using Terranova (Terranova was born because using terraform golang code is complex)
Angel’s skills are not good enough to develop this CLI starting from the terraform code.
- [Terraform-exec](https://github.com/hashicorp/terraform-exec): it is a project maintained by hashicorp, making easy execute terraform code from golang. It is just a wrapper around a terraform binary. 
As you can see, this seems to be the easier path with our current environment.


## Testing+

It has to automate test:

- unit-testing
- integration tests
- e2e tests

Then manual tests have to be done on real providers.

## Deploy / Rollout+

The deployment of new releases has to be triggered automatically via drone + GitHub releases.
It has to be available via brew or download it via the GitHub release download link.

## Metrics

As we experimented with mixpanel, we can continue adding metrics there. We can monitor (if possible):

- Number of Downloads of the `furyctl` binary
  - Platforms
  - Version
- Number of provisioner clusters
  - Provisioner
- Number of updated clusters
- Number of destroyed clusters.

## Documentation

- kubernetesfury.com has to be updated
  - cli section
  - create a new section under installers
- community docs in the repository.
  - tl;dr with a working example

## Future Opportunities

This feature will enable multiple streams:

- The community will be able to test Fury. This could potentially end up with some ping from enterprise to get support
on it.
- Current clients will be able to spin up new Clusters without the delivery team. Then we can bill them by the
CPU/node if they require support on these clusters.
