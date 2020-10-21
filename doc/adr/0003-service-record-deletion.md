# 3. Service Record Deletion

Date: 2020-08-21
Author: Christian Ang, Edwin Xie, Jamie Monserrate

## Status

Accepted

## Context

The connectivity-publisher controller watches for `HTTPProxy` resources
and if an `HTTPProxy` resource is created with the
`connectivity.tanzu.vmware.com/export` label then the controller creates a
`ServiceRecord` resource within the `cross-cluster-connectivity` namespace.

The `HTTPProxy` has an `FQDN` field on the `spec` and the `FQDN` is used as the
name of the `ServiceRecord` created by the connectivity-publisher.

In an ideal world the `ServiceRecord` resource would have an `OwnerReference` to
the `HTTPProxy` resource and that would take care of deletion. However, this
isn't possible because they are in different namespaces and an `OwnerReference`
cannot be added across different namespaces.

## Decision

There are three pieces to this puzzle:
* When a user removes the `connectivity.tanzu.vmware.com/export` label
  `HTTPProxy`, the connectivity-publisher can perform a reconcile on the
  updated `HTTPProxy` object, see there is no
  `connectivity.tanzu.vmware.com/export` label and delete the `ServiceRecord`
  named with the FQDN.
* When the user deletes the `HTTPProxy` resource entirely, the
  connectivity-publisher can perform a reconcile, see the `HTTPProxy`
  resource has been deleted, get the FQDN from the cached `HTTPProxy` resource,
  and delete the `ServiceRecord` named with that FQDN.
* There are also scenarios where a `ServiceRecord` can be orphaned. The primary
  one being, if the controller is down and the user deletes the `HTTPProxy`
  resource how would the controller reconcile the deletion after it starts back
  up.
  To handle this, the connectivity-publisher will watch the
  `ServiceRecord` resources. On reconcile of the `ServiceRecord` resource it
  will check if the `ServiceRecord` has a parent `HTTPProxy`. If it has been
  orphaned, it will be deleted.

### Out-of-Scope

We are considering these two cases out of scope because we are only considering
the Harbor use-case.

* The user changes the FQDN and orphans a `ServiceRecord`.
* There are `ServiceRecords` that were created by controllers/users outside the
  control of connectivity-publisher.

### Open Questions

The following are open questions that we don't think need to be answered right
now for the first implementation, but will require a decision in the future and
will probably modify the decision made within this ADR.

* Can two HTTPProxy's have the same FQDN?
  * If yes this would potentially create a single `ServiceRecord` that has
    endpoints for both `HTTPProxies`. This would change how deletions would be
    handled.
* Do we want to support `ServiceRecords` being created by controllers other than
  the connectivity-publisher?

## Consequences

* Deleting `ServiceRecords` works.
* There are some unhandled `ServiceRecord` deletion cases e.g. changing the FQDN
  on a `HTTPProxy`
