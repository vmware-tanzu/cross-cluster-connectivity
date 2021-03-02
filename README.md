# Cross-cluster Connectivity

Multi-cluster DNS for [Cluster API](https://cluster-api.sigs.k8s.io/)

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

## Walkthrough

This walkthrough assumes:
- A [management
  cluster](https://cluster-api.sigs.k8s.io/reference/glossary.html#management-cluster)
  exists, running the Cluster API.
- Two [workload
  clusters](https://cluster-api.sigs.k8s.io/user/quick-start.html#create-your-first-workload-cluster)
  exist, with support for services type LoadBalancer. For the sake of this doc, assume
  `cluster-a` and `cluster-b` exist and both of these Clusters belong to the
  `dev-team` namespace on the management cluster.
- The images referenced in these files are not publicly available. The images
  can be built using `make build-images`. Until the team finds a home for
  images, you'll need to push them to your own registry and update the
  deployments accordingly.
- If you're using `kind`, you'll need to BYO your own load balancer.
  Consider using [MetalLB](https://metallb.universe.tf).

### Install Multi-cluster DNS on management cluster

1. Install `GatewayDNS` CRD on the management cluster
   ```bash
   kubectl --kubeconfig management.kubeconfig \
      apply -f manifests/crds/connectivity.tanzu.vmware.com_gatewaydns.yaml
   ```
1. Install `xcc-dns-controller` on the management cluster
   ```bash
   kubectl  --kubeconfig management.kubeconfig \
      apply -f manifests/xcc-dns-controller/deployment.yaml
   ```

   Note: by default, this manifest configures the controller to generate
   cross-cluster DNS records with the suffix `xcc.test`.
   To use a different suffix, customize your deployment yaml, e.g.
   ```bash
   cat manifests/xcc-dns-controller/deployment.yaml \
      | sed 's/xcc\.test/multi-cluster.example.com/g' \
      | kubectl --kubeconfig management.kubeconfig apply -f -
   ```

### Install Multi-cluster DNS on *each* workload cluster

1. Deploy `dns-server` controller on both workload clusters
   ```bash
   kubectl --kubeconfig cluster-a.kubeconfig \
      apply -f manifests/dns-server/
   ```
1. Configure your cluster's root DNS server to forward queries for the `xcc.test` zone to
   the xcc `dns-server`. This can be done by running the `dns-config-patcher`
   job.
   ```bash
   kubectl --kubeconfig cluster-a.kubeconfig \
      apply -f manifests/dns-config-patcher/deployment.yaml
   ```
   Note: to use a DNS zone other than `xcc.test`, customize your deployment yaml, e.g.
   ```bash
   cat manifests/dns-config-patcher/deployment.yaml \
      | sed 's/xcc\.test/multi-cluster.example.com/g' \
      | kubectl --kubeconfig cluster-a.kubeconfig apply -f -
   ```

Repeat the steps above for `cluster-b`.

### Deploy a load balanced service to `cluster-a`

1. Install Contour
   ```bash
   kubectl --kubeconfig cluster-a.kubeconfig \
      apply -f manifests/contour/
   ```
1. Deploy a workload (kuard)
   ```bash
   kubectl --kubeconfig cluster-a.kubeconfig \
      apply -f manifests/example/kuard.yaml
   ```

### Lastly, wire up cross cluster connectivity

1. On the management cluster, label the Clusters that have services that shall
   be discoverable by other clusters. The `GatewayDNS` record created later will
   use this label as a `ClusterSelector`.  In this example, the clusters are
   using the label `hasContour=true`.
   ```bash
   kubectl --kubeconfig management.kubeconfig \
      -n dev-team label cluster cluster-a hasContour=true --overwrite
   kubectl --kubeconfig management.kubeconfig \
      -n dev-team label cluster cluster-b hasContour=true --overwrite
   ```
1. On the management cluster, create a GatewayDNS Record.
   ```bash
   kubectl --kubeconfig management.kubeconfig \
      -n dev-team apply -f manifests/example/dev-team-gateway-dns.yaml
   ```

   The GatewayDNS's spec has a `clusterSelector` that tells the
   `xcc-dns-controller` which clusters shall be watched for services. The
   controller will look for a service with the namespace/name of `service`. In
   this example, Contour runs a service in the `projectcontour/envoy`
   namespace/name. The `resolutionType` in this example's service is type
   `loadBalancer`.
   ```yaml
   ---
   apiVersion: connectivity.tanzu.vmware.com/v1alpha1
   kind: GatewayDNS
   metadata:
     name: dev-team-gateway-dns
     namespace: dev-team
   spec:
     clusterSelector:
       matchLabels:
         hasContour: "true"
     service: projectcontour/envoy
     resolutionType: loadBalancer
   ```

### Test DNS resolution from `cluster-b` to `cluster-a`

At this point the kuard application deployed to `cluster-a` should be
addressable from `cluster-b`.
   ```bash
   kubectl --kubeconfig cluster-b.kubeconfig \
      run kuard-test -i --rm --image=curlimages/curl \
      --restart=Never -- curl -v --connect-timeout 3 \
      http://kuard.gateway.cluster-a.dev-team.clusters.xcc.test
   ```

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
