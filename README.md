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
1. Install `capi-dns-controller` on the management cluster
   ```bash
   kubectl  --kubeconfig management.kubeconfig \
      apply -f manifests/capi-dns-controller/deployment.yaml
   ```

### Install Multi-cluster DNS on *each* workload cluster

1. Deploy `dns-server` controller on both workload clusters
   ```bash
   kubectl --kubeconfig cluster-a.kubeconfig \
      apply -f manifests/dns-server/
   ```
1. Get IP address assigned to the DNS server service
   ```bash
   kubectl --kubeconfig cluster-a.kubeconfig \
      get service -n capi-dns dns-server -o=jsonpath='{.spec.clusterIP}'
   ```
1. Patch CoreDNS to forward `xcc.test` zone to the `dns-server`.
   Note: the needed change is to add forwarding configuration for the
   `xcc.test` zone. Preserve your Corefile's other configurations.
   ```bash
   kubectl --kubeconfig cluster-a.kubeconfig \
      -n kube-system patch configmap coredns \
      --type=strategic --patch="$(cat <<EOF
   data:
     Corefile: |
       .:53 {
           errors
           health {
              lameduck 5s
           }
           ready
           kubernetes cluster.local in-addr.arpa ip6.arpa {
              pods insecure
              fallthrough in-addr.arpa ip6.arpa
              ttl 30
           }
           prometheus :9153
           forward . /etc/resolv.conf
           cache 30
           loop
           reload
           loadbalance
       }
       xcc.test {
           forward . <REPLACE_WITH_IP_OBTAINED_IN_PRIOR_STEP>
           reload
       }
   EOF
    )"
    ```
1. Perform the same steps in this section for `cluster-b`

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
   `capi-dns-controller` which clusters shall be watched for services. The
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
