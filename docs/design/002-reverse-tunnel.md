# LGTM (Looks Good To Me)

- [ ] Ángel Barrera Sánchez
- [ ] Giovanni Laieta
- [ ] Luca Novara
- [ ] ???

# Status+

_Draft_: the paragraphs that are prefixed or suffixed with `...`
should be considered scratchpads for ideas that need further thinking
and structure.

# Problem

Every Fury customer have different security policies (some are
stricter than others) and different implementation of those
policies.

Since we need the ability to access potentially every node
of every cluster, for each client we need to come up with something
that will let us do that in a way that is compliant with the client
policies.

Having different ways to access different clusters for different
clients leads to:

- Time wasted by the operation team.
- Time wasted organizing the work of the operation team.
- Cognitive overhead in time sensitive situations: we need to fix
  incidents as soon as possible, we cannot waste time thinking how to
  connect to the customer's nodes.
- Lack of scalability and diversification: better to put the same
  people on the same clients because they already know how to access
  their nodes.
- Perceived immaturity from the customer: not having this matter
  solved in a fully automated way can bring questions from the
  customer side.

# Description

A uniform and convenient way to access all the customer's nodes.

# Solution

## Constraints

- The solution must require the minimum amount of trust to the
  customer side, i.e. our requirements must be compliant with as much
  security policies as possible.
- The solution must require the minimum amount of effort to the
  customer side.
- Only certain people from SIGHUP should be able to access a certain
  customer's node and we need to be able to choose them. We need to
  have the ability to add or remove people in an easy and centralized
  way.
- Only certain people from SIGHUP should be able to access a certain
  customer's node in certain time windows and the customer should be
  able to choose them.
- If a node is reachable we need to be able to connect to it.

## Context

See the _Problem_ section, that's basically the history

## Implementation

We need to have a daemon running on every customer's node that we want
to reach. We will call it `fury-tunnel-agent`.

The main goal of this daemon is to open a reverse SSH tunnel towards
one or more nodes maintained by and accessible by SIGHUP. We will call
them `bastions`.

On every `bastion` will run a service on port `443` (HTTPS port). We
will call this service `fury-home`.

The main goal of this service will be to help and organize the work of
all the `fury-tunnel-agent`s.

When a `fury-tunnel-agent` starts it will ask `fury-base` to book a
port on a `bastion` to be used as port to create the reverse SSH
tunnel.

Then it will create the reverse SSH tunnel towards the bastion using
the `443` port (`fury-home` or reverse proxy should be able to tell if
the connection is coming from a SSH client and if so to forward it to
the SSH server).

The `fury-tunnel-agent` must strive to keep the reverse tunnel
connected all the time.

The `fury-tunnel-agent` must ask periodically to `fury-home` who are
the people authorized to access the node, the response must be a list
of public keys that will populate an `authorized_keys` files that will
let anyone that owns the associated private keys to access the node
via the SSH tunnel.

An HTML user interface can be provided by every node via
`fury-tunnel-agent` to let the customer select from a list of people
who will be able to access the current node for how much time,
`fury-tunnel-agent` will then forward the request to `fury-home` that
will send later the appropriate public keys.

The `fury-home` will provide a set of API able to configure who should
have access to what nodes of what customer in which time-frame.

A `fury-tunnel-cli` executable will be available to the SIGHUP
employees. This executable will let them to list the available nodes
of all the customers and to connect to one of them given the
appropriate credentials.

... Provide sequence diagrams for node bootstrap scenario

... Provide sequence diagrams for node bootstrap scenario

### Daemon: fury-tunnel-agent

...

### Service: fury-home

...

### Executable: fury-tunnel-cli

...

### Bastion

...

### Fury Integration

... Write this with Ángel

... Two kinds of installation, `fury-tunnel-agent` on every customer's
nodes, `fury-tunnel-agent` only on some customer's bastion
nodes. Maybe this needs some further discussion?

### Plan

- ...
- ...
- ...

## Personas+

Who are the users of this feature / product?

## Risks+

How can this fail, what kind of risks we face?

## Considered Alternatives

... Should be filled by Giovanni?

## Infrastructure Requirements+

What do you need to run it?

## Testing+

How are you going to make sure that it works?

## Deploy / Rollout+

See _Fury Integration_ section

## Metrics

How can we measure the technical qualities of this solution?

## Documentation

What kind of documentation we need to expect?

## Security

Security concerns we need to address and how.

## Privacy

Privacy concerns we need to address and how.

## Scaling

How it's going to scale?

## Future Opportunities

What kind of opportunity will open / create in the future?
