# 6. Naming of the Imported Service Record

Date: 2020-09-11
Author: Christian Ang, Edwin Xie

## Status

Accepted

## Context

Previously, when an imported ServiceRecord was created by our controllers,
it was given a name that reflected both the FQDN that the ServiceRecord
used and the remote registry from which the ServiceRecord was imported:
`<fqdn>.<remote-registry-name>`.

For example, if a registry on cluster A exported a ServiceRecord named
`foo.bar.com`, it would be named something like `foo.bar.com.cluster-a`.

However, the remote registry name affects the length of the FQDN with this
format because the remote registry name can be of arbitrary length and a
Kubernetes name is limited to 253 characters.

In order to ensure cross-cluster connectivity in cases when the name of the
ServiceRecord's FQDN or remote registry name is too long, it's necessary to
come up with some sort of scheme to shorten the length of the ServiceRecord
name.

## Decision

Hash the remote registry name when generating the ServiceRecord name, i.e the
ServiceRecord will now use the following:
`<FQDN>-<8-char hash of remote registry name>`.

In order to provide hashes with a reasonably low chance of collision, we have
decided to use fnv-1a to create 32-bit hashes of the remote registry name.

In order to ensure that it is still possible to determine from which remote
registry the ServiceRecord was imported, we also added an additional label
to the ServiceRecord with the full remote registry name under the key
`"connectivity.tanzu.vmware.com/remote-registry"`.

## Consequence

- The FQDN limit for a shared service is 253-9 characters, 244 characters.
