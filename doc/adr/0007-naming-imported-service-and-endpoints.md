# 7. Naming of the Imported Service and Service Endpoints

Date: 2020-09-14
Author: Christian Ang, Edwin Xie

## Status

Accepted

## Context

The Service and Service Endpoint created from an imported ServiceRecord uses the
FQDN as the name. This enables a Service to use endpoints from multiple
ServiceRecords (i.e multiple clusters).

A Service name is limited to 63 characters due to the DNS spec limiting a
subdomain to 63 characters. A Service name is required to conform to this
limit so that the Service name can be used as a DNS name in-cluster.

## Decision

The Service Name and Service Endpoint Name will use the FQDN that is truncated
to 54 characters with an 8 character hash appended to the end i.e
`<fqdn truncated to 54 characters>-<8 character hash of fqdn>`.

In order to provide hashes with a reasonably low chance of collision, we have
decided to use fnv-1a to create 32-bit hashes of the FQDN.

## Consequence

- The FQDN is no longer limited by the Service/Endpoint name.
