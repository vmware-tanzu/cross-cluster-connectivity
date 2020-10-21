# 4. Services Garbage Collection

Date: 2020-08-25
Author: Christian Ang, Edwin Xie, Jamie Monserrate

## Status

Accepted

## Context

The connectivity-binder watches for `ServiceRecord` resources on the
workload cluster and creates a `Service` resource and an `Endpoints` resource
locally on the workload cluster in the `cross-cluster-connectivity` namespace.

The name of the `ServiceRecord` resource is the `FQDN` of the imported service
followed by the name of the cluster from which this service was imported. The
`ServiceRecord` also has an `FQDN` field on the `spec` which identifies this
imported service as part of a collection of distributed services that can be
accessed by the same fully qualified domain name. The connectivity-binder
will create a single `Service` and `Endpoints` resource to represent every
`ServiceRecord` in this collection.

When an imported `ServiceRecord` is deleted because of a change in the registry,
the following behavior is desired:

1. If the deleted `ServiceRecord` is the only imported service remaining for its
   FQDN, then delete the `Service` and `Endpoints` resources associated with
   this FQDN.
2. If the deleted `ServiceRecord` is not the only imported service remaining for
   its FQDN, then the `Endpoints` resource should be updated to reflect the
   change in imported services that comprise the FQDN.

## Decision

To implement the first desired behavior, the connectivity-binder will add
to both the `Service` and `Endpoints` resources an `OwnerReference` for each
`ServiceRecord` associated with the FQDN for the collection. As a result, when
the last `ServiceRecord` remaining for a particular FQDN is removed, Kubernetes
will automatically garbage-collect the associated `Service` and `Endpoints`
resource.

To implement the second desired behavior, the connectivity-binder will
handle the delete logic by treating the deletion of a `ServiceRecord` as an
`Update` operation. In a normal `Update` operation, when a modification of a
`ServiceRecord` occurs, the connectivity-binder will retrieve all
`ServiceRecords` for its FQDN and recompute the endpoints for the `Service`.
In the `Delete` case, the connectivity-binder can reuse this existing
logic to recompute the endpoints and update the `OwnerReferences` on the
`Service` and `Endpoints` to match the remaining `ServiceRecords`. In the case
that there are no remaining `ServiceRecords` for the FQDN, the
connectivity-binder will skip handling of the update entirely to allow
the Kubernetes garbage collection to take place naturally.

## Consequences

* When the only ServiceRecord for a given FQDN is deleted on the workload
  cluster, garbage collection of associated `Service` and `Endpoints` on the
  workload cluster works.
* When one of many ServiceRecords for a given FQDN is deleted on the workload
  cluster, the associated `Service` and `Endpoints` are updated and connectivity
  for the FQDN remains possible.

