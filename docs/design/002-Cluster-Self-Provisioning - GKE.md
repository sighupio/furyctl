# LGTM (Looks Good To Me)

- [ ] Gabriele Lana
- [ ] Jacopo Nardiello
- [ ] Luca Novara
- [ ] Niccolo' Raspa

> If you think someone else can review this document, feel free to add them to the list.

# Description+

This document's primary goal is to define the second phase of the self-service cluster creation feature for the
`furyctl` binary. Currently, `furyctl` is underused; it can only deploy AWS/EKS clusters along with the
requirements to deploy it (bastion/VPN, VPC, and so on).

It is time to extend its functionalities to provide a way to self-provision Kubernetes clusters on Google Cloud / GKE.

# Problem+

As mentioned above, the main problem to solve with this CVI is to enable other cloud providers into this binary. 
The main goal is to continue enabling new cloud providers to deploy Kubernetes clusters.

The next cloud provider to enable in this binary should be Google Cloud and GKE.

It is not only about deploying (one shoot) Kubernetes Clusters; it has to be maintained during the time, so any
kind of action has to be done via `furyctl`.

Even being open source, SIGHUP Fury stack is not easy to use by someone on the internet *(aka community)*.
This situation limits possible clients' target as no one can try Fury Clusters before considering hiring us.

# Solution+

## Context+

[The initial design of the cluster self-provisioning feature](001-Cluster-Self-Provisioning.md)
enables the easy extension of it with new provisioners. 

## Implementation+

We have to extend the `cluster` and `bootstrap` subcommand under the `furyctl` binary to be able to manage the complete
lifecycle of a GKE cluster using the current [SIGHUP GKE Cloud installer](https://github.com/sighupio/fury-gke-installer).

The implementation will:

- (Optional) Create a Network with a VPN Host. The command will be something like: `furyctl bootstrap {init,apply,destroy} --config bootstrap.yml`
  - The VPN server has to provide auto recovery in case an outage happens. TL;DR use `furyagent` to init.
  - Then, to create a production-grade cluster, the operator has to connect to the VPN server.
- Create a cluster in a user-defined network. As the first implementation is a GKE private cluster, a pre-requirement will stay connected in the VPN created by the bootstrap stage.
  - The provisioner has to warn the operator if it needs to be connected to a VPN or deploy the cluster using a bastion host. (It has to have connectivity to the end cluster)


### Bootstrap

The bootstrap has to initialize a network and deploy a VPN host in the public subnet as we are going to deploy only private clusters

The VPN host has to use our `furyagent` functionalities to:
- Init the configuration (both VPN and ssh users).
- Configure the VPN software.
- Configure the ssh system user.

Raise a warning if the terraform state configuration is a file.

### Cluster

The cluster has to be provided through the bastion host or using a previously configured VPN. One of both alternatives
it's required because some clusters providers need to have connectivity to the API server (private, not public-facing)

Raise a warning if the terraform state configuration is a file.

## Constraints+

- Have to be integrated within `furyctl` and use `furyagent`.

## Risks+

The possibility of failing during the implementation is there:

- We could not manage the complete lifecycle.
- We could end up with a tool not used by anyone in the company/community.


## Considered Alternatives+

> [The initial design already addressed other alternatives](001-Cluster-Self-Provisioning.md)

We selected google cloud as the next provisioner in `furyctl` instead of azure because the AKS installer has some manual
interactions. It makes it challenging to automate the completed workflow.


## Testing+

It has to automate test:

- unit-testing
- integration-tests
- e2e-tests

Then manual tests have to be done on real providers.

## Deploy / Rollout+

The deployment of new releases has to be triggered automatically via drone + GitHub releases.
It has to be available via brew or download via the GitHub release download link.

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
  - CLI section
  - create a new section under installers
- community docs in the repository.
  - tl;dr with a working example

## Future Opportunities

This feature will enable multiple streams:

- The community will be able to test Fury. This could potentially end up with some ping from enterprise to get support
on it.
- Current clients will be able to spin up new Clusters without the delivery team. Then we can bill them by the
CPU/node if they require support on these clusters.
