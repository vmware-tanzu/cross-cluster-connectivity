# Cross-cluster Connectivity

Cross-cluster Connectivity enables a Service in one cluster to be discovered and
used by a client pod in a different cluster, while putting minimal requirements
on the clusters or their networking environment.

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

## Limitations

* Cluster nodes need to be IP routable, for now
* No multi-tenancy support, yet
* No IPv6 support, yet

## Getting Started

These instructions will setup the project on a [Kind](https://kind.sigs.k8s.io/)
cluster on your local machine for development and testing purposes.

For instructions on installing Cross-cluster Connectivity on other Kubernetes
clusters, see [INSTALLATION.md](./INSTALLATION.md).

### Prerequisites

* [kind](https://kind.sigs.k8s.io/) 0.7.0+
* [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
* [clusterctl](https://cluster-api.sigs.k8s.io/clusterctl/overview.html)
* [kustomize](https://kustomize.io/)
* [docker](https://www.docker.com/)
* [jq](https://stedolan.github.io/jq/) 1.3+
* [make](https://www.gnu.org/software/make/)
* [ytt](https://get-ytt.io/)
* Docker requires at least 4GB of memory.

### Installing on Kind

1. First, we will `git clone` and `cd` into the `cross-cluster-connectivity-api`
   repository.
   ```bash
   git clone https://github.com/vmware-tanzu/cross-cluster-connectivity-api.git
   cd cross-cluster-connectivity-api
   ```

1. Build the docker images.
   ```bash
   $ make build-images
   ```

1. Next is spinning up the environment locally on `kind`.
   ```bash
   $ make e2e-up
   ```

   After this script is complete, you will see 3 `kind` clusters:
   1. The management cluster created by [Cluster
      API](https://cluster-api.sigs.k8s.io/) that manages `kind` clusters.
   1. A cluster that hosts shared services. This cluster can be accessed using
      the kubeconfig file named `shared-services.kubeconfig`.
   1. A cluster that hosts workloads, which can access the shared services. This
      cluster can be accessed using the kubeconfig file named
      `workloads.kubeconfig`.

### Deploy an example shared service

There is a set of manifests within `./manifests/example/nginx` to serve as an
example of how to deploy and export an Nginx deployment to the workload cluster.

1. Apply the `./manifests/example/nginx/certs.yaml` to create self-signed certificates
   for Nginx. This manifest depends on [cert-manager](https://cert-manager.io),
   which is deployed for you when using `make e2e-up`.
   ```bash
   $ kubectl --kubeconfig ./shared-services.kubeconfig apply -f ./manifests/example/nginx/certs.yaml
   ```

1. Apply the `./manfiests/example/nginx.yaml` to create the Nginx deployment and
   Nginx Kubernetes service.
   ```bash
   $ kubectl --kubeconfig ./shared-services.kubeconfig apply -f ./manifests/example/nginx/nginx.yaml
   ```

1. Apply the `./manifests/example/nginx/exported_http_proxy.yaml` to create a Contour
   HTTPProxy resource that has been labeled with
   `connectivity.tanzu.vmware.com/export` to declare it as an exported service.
   ```bash
   $ kubectl --kubeconfig ./shared-services.kubeconfig apply -f ./manifests/example/nginx/exported_http_proxy.yaml
   ```

1. Test connectivity to the shared service from the workload cluster.
   ```bash
   $ kubectl --kubeconfig ./workloads.kubeconfig run -it --rm --restart=Never \
      --image=curlimages/curl curl -- curl -v -k "https://nginx.xcc.test"
   ```

### Tear Down Environment

To clean up the clusters created by the `make e2e-up` command:
```bash
make e2e-down
```

## Built With

* [Hamlet](https://github.com/vmware/hamlet) - The API that enables federation
  between clusters
* [CoreDNS](https://coredns.io/) - Used to build DNS Server

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
