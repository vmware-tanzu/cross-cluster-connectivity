# 5. Enable Service Discovery via DNS

Date: 2020-08-26
Author: Christian Ang, Jamie Monserrate, Edwin Xie

## Status

Proposal

## Context

When a user exports a shared service the following steps occur:
* A selector-less `Service` with a `ClusterIP` is created on the workload
  clusters for the shared service in the `cross-cluster-connectivity` namespace.
* An `Endpoint` is created for each node IP on the workload clusters for the
  shared service in the `cross-cluster-connectivity` namespace.

To discover and make requests to the shared service, a consumer of the service
would have to use the Kubernetes API to discover the `ClusterIP` of the
`Service` in the `cross-cluster-connectivity` namespace. This would also require
the consumer to inject the FQDN of the `Service` into the SNI (Server Name
Indicator) when they make a request. This wouldn't be a great user experience
for the consumer.

## Decision

There are a few options to provide Service Discovery for a consumer that provide
an easier user experience. All of them revolve around providing Service
Discovery via DNS.

### Use the existing Kubernetes DNS for Services.

A `Service` is already created by the connectivity-binder controller,
which Kubernetes is creating a DNS entry for. With a small modification to how
`Services` are named at the moment to be just the hostname instead of the entire
FQDN, the API could require the `HTTPProxy` to use the specific Kubernetes DNS
format. For example, the FQDN given to `HTTPProxy` could be set to
`foo.cross-cluster-connectivity.svc.cluster.local`. This would cause an `Service`
to be created on the importing cluster with the name `foo` in the namespace
`cross-cluster-connectivity`, allowing a user on the importing cluster to use the
DNS name of `foo.cross-cluster-connectivity.svc.cluster.local`.

There are downsides to this approach:
* There are use-cases where a user would want to use a custom FQDN.
* This would be a leaky abstraction because we are leaking the
  `cross-cluster-connectivity` namespace. The fact that a `Service` is created
  for an exported Service is an implementation detail of the system.
* Reusing the `cluster.local` domain for cross cluster services feels like an
  odd design choice.

### Use an external DNS provider.

An external DNS provider could be used to manage the DNS records for the shared
services.

There are tradeoffs to this approach as well:
* Existing design choices have been made to avoid centralized control planes
  like an external DNS provider.
  * Some organizations have organization wide DNS systems and they may not trust
    a series of clusters to modify this DNS system dynamically. Some
    organizations would require a person to manually submit a ticket to modify a
    DNS entry or zone.
  * The design of our product is akin to a "decentralized DNS system", with
    registry clients explicitly opting in to receiving records from a registry
    server on a cluster. Implementing this product with "centralized DNS" is
    orthogonal to the product's goal.

### Use a single DNS entry to point to a reverse proxy.

Another alternative to routing via DNS entries, is to route via a reverse proxy
and have fewer, static DNS entries that point to the reverse proxy IP.

There are tradeoffs to this approach:
* This will require extra hops to get to a shared service.
* The reverse proxy would probably require SNI to route to a shared service.
  This makes TCP/UDP services more difficult to implement.
* The flipside of using SNI to route is that there would be less IPs being used
  by the system.
* A reverse proxy can allow a user to configure traffic shaping e.g weighted
  routing, rate limiting.

### Plug-in our own DNS system into the existing Kubernetes DNS system.

The last option is to delegate a zone in the Kubernetes DNS system to our own
DNS server in each cluster. Both kube-dns and core-dns have methods of
configuring forwarding DNS requests to another server.

There are tradeoffs to this approach:
* The configuration for Kube-DNS or Core-DNS is a configmap. This causes some
  complexity because we can change it, but we may be overriding a change or
  another process/user may overwrite our change.
* This approach is generally more flexible than the other options e.g can use a
  custom FQDN and is decentralized.

### Actual Decision

We plan to plug-in our own DNS system into the existing Kubernetes DNS system.
This will be our own CoreDNS server.

There will be a new deployment that is a CoreDNS server and a Kubernetes
controller. The controller will watch for shared services that need to be
resolved by the DNS server, it can store the FQDNs and ClusterIP in memory for
the CoreDNS server to use as a lookup table when it receives a DNS request.
Another potential solution is to use a CNAME record to the Service in cluster
DNS name instead of an A record to the ClusterIP, but to avoid the complexity of
figuring out the FQDN of the `Service`, we will just use the ClusterIP.

The controller will watch for `Services` to discover what shared services it
needs to resolve. It will filter for `Services` with the
`connectivity.tanzu.vmware.com/fqdn` annotation. The value of this annotation
and the `ClusterIP` of the `Service` will be stored in memory.

When a request is made to the DNS server, our custom CoreDNS plugin in the
CoreDNS server will handle the request by using the in-memory lookup table
generated by the Kubernetes controller part of the process.

A potential solution to the installation problem is to use
[coredns-operator](https://github.com/kubernetes-sigs/cluster-addons/tree/master/coredns),
which can control the Corefile in the case of using CoreDNS as the core
Kubernetes DNS system. This allows us to delegate a specific domain to our DNS
server to resolve.

## Consequences

- A user of a shared service can discover and make requests to a shared service
  by using DNS.
