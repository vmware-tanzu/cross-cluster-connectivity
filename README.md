# Cross-cluster Connectivity

Multi-cluster DNS for [Cluster API](https://cluster-api.sigs.k8s.io/)


## Status

This project is in the middle of a reboot, to simplify the operational model
and utilize a Cluster API control plane.  We're keeping the old code on the `v1`
branch.  New development is happening in PRs against `main`.

A high-level tracking issue is [#38](https://github.com/vmware-tanzu/cross-cluster-connectivity/issues/38).


## What?

This project aims to enables a Kubernetes service in one cluster to be
discovered and used by a client pod in a different cluster, while putting
minimal requirements on the clusters or their networking environment.


## Why?

There are various ways to share Kubernetes Services across clusters. Each has
its limitations, often due to assumptions made about the infrastructure network,
DNS, or tenancy model of the clusters.

This project aims to provide cross-cluster service discovery and connectivity
even when:

1. Service type:LoadBalancer is unavailable, or prohibitively expensive if used
   per-service
2. Pod and ClusterIP CIDR blocks are identical on all clusters
3. Users do not have permissions to edit public DNS records
4. Clusters do not follow the rules of [Namespace
   Sameness](https://groups.google.com/forum/#!msg/kubernetes-sig-multicluster/jfDAMxFWlOg/9Z9O0mVpAgAJ)



## Contributing

Please read [CONTRIBUTING.md](./CONTRIBUTING.md) for details on the process for
running tests, making changes, and submitting issues and pull requests to the
project.

## Code of Conduct

Please familiarize yourself with the [Code of Conduct](./CODE_OF_CONDUCT.md)
before contributing. This code of conduct applies to the **Cross-cluster
Connectivity** community at large (Slack, mailing lists, Twitter, etc.).

## License

This project is licensed under the Apache-2.0 License - see the
[LICENSE](LICENSE) file for details.
