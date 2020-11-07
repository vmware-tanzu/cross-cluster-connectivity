# Installation

These instructions guide you through setting up an example two-cluster
deployment of Cross-cluster Connectivity on Kubernetes. You can adapt these
instructions to cover other deployment scenarios.

## Prerequisites

* [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
* [make](https://www.gnu.org/software/make/)
* [docker](https://www.docker.com/)
* Verify that you have set up two Kubernetes clusters in the same network. In
  other words, the Kubernetes nodes in each of the clusters should be able to
  access each other over this network.

## Installing

1. First, we will `git clone` and `cd` into the `cross-cluster-connectivity-api`
   repository.
   ```bash
   $ git clone https://github.com/vmware-tanzu/cross-cluster-connectivity-api.git
   $ cd cross-cluster-connectivity-api
   ```

1. Since Cross-cluster Connectivity is still in the alpha stage, the images will
   have to be built and uploaded to a Docker registry for development purposes.
   Build the docker images with the `IMAGE_REGISTRY` environment set to where
   the images will be located. For example:
   ```bash
   $ IMAGE_REGISTRY=ghcr.io/vmware-tanzu/cross-cluster-connectivity make build-images
   ```

1. Upload these images to the Docker registry you have chosen:
   ```bash
   $ IMAGE_REGISTRY=ghcr.io/vmware-tanzu/cross-cluster-connectivity make push-images
   ```

1. Update the deployment manifests to use the correct image registry. You will
   have to modify the `image` value in each of the following files with your
   registry:
   ```
   ./manifests/connectivity-binder/deployment.yaml
   ./manifests/connectivity-dns/deployment.yaml
   ./manifests/connectivity-publisher/deployment.yaml
   ./manifests/connectivity-registry/deployment.yaml
   ```

### Install Cross-cluster Connectivity on the Cluster exporting Services

Contour must be installed on the Cluster exporting Services in addition to the
`connectivity-publisher` and `connectivity-registry` components from
Cross-cluster Connectivity.

#### Setup Steps

1. Follow instructions [here](https://projectcontour.io/getting-started/) to
   install Contour.

1. Create a namespace called `cross-cluster-connectivity`. This namespace will
   be used by all the Cross-cluster Connectivity components.
  ```bash
  kubectl create namespace cross-cluster-connectivity
  ```

#### Generate Certificates for the `connectivity-registry` components

To enable certificate management for the `connectivity-registry` component the
instructions will also install `cert-manager`, but this could be replaced by
another certificate management solution, if desired.

1. Follow instructions
   [here](https://cert-manager.io/docs/installation/kubernetes) to install
   cert-manager.

1. Add a cert-manager `Issuer` to generate a certificate for the registry server
   in the following command.
   - To quickly get started, you can apply our manifest to generate a
     self-signed `Issuer` that can generate a certificate for the registry
     server.
   ```bash
   $ kubectl apply -f ./manifests/connectivity-registry-certs/ca_certificate.yaml
   ```
   - You can also use your own CA certificate to generate certificates for the
   registry server. Create a secret named `registry-ca` in the
   `cross-cluster-connectivity` namespace as per these
   [instructions](https://cert-manager.io/docs/configuration/ca/).

1. Change the `REGISTRY_DOMAIN` in the
   `./manifests/connectivity-registry-certs/registry_certificate.yaml` to a
   domain of your choosing. This is the domain where the registry server will be
   published within the cluster. For example, the Kind cluster setup uses
   `shared-services-registry.xcc.test`.

1. Create connectivity-registry certificate.
   ```bash
   $ kubectl apply -f ./manifests/connectivity-registry-certs/registry_certificate.yaml
   ```

#### Install Cross-cluster Connectivity components

1. Install the CRDs.
   ```bash
   $ kubectl apply -f ./manifests/crds
   ```

1. Install connectivity-publisher.
   ```bash
   $ kubectl apply -f ./manifests/connectivity-publisher
   ```

1. Install connectivity-registry.
   ```bash
   $ kubectl apply -f ./manifests/connectivity-registry
   ```

### Install Cross-cluster Connectivity on the Cluster consuming Services

#### Setup Steps

1. Create a namespace called `cross-cluster-connectivity`. This namespace will
   be used by all the Cross-cluster Connectivity components.
  ```bash
  $ kubectl create namespace cross-cluster-connectivity
  ```

#### Generate Certificates for the `connectivity-registry` components

To enable certificate management for the `connectivity-registry` component the
instructions will also install `cert-manager`, but this could be replaced by
another certificate management solution, if desired.

1. Follow instructions
   [here](https://cert-manager.io/docs/installation/kubernetes) to install
   cert-manager.

1. Add a cert-manager `Issuer` to generate a certificate for the registry server
   in the following command.
   - To quickly get started, you can apply our manifest to generate a
     self-signed `Issuer` that can generate a certificate for the registry
     server.
   ```bash
   $ kubectl apply -f ./manifests/connectivity-registry-certs/ca_certificate.yaml
   ```
   - You can also use your own CA certificate to generate certificates for the
   registry server. Create a secret named `registry-ca` in the
   `cross-cluster-connectivity` namespace as per these
   [instructions](https://cert-manager.io/docs/configuration/ca/).

1. Change the `REGISTRY_DOMAIN` in the
   `./manifests/connectivity-registry-certs/registry_certificate.yaml` to a
   domain of your choosing. This is the domain where the registry server will be
   published within the cluster. For example, the Kind cluster setup uses
   `shared-services-registry.xcc.test`.

1. Create connectivity-registry certificate.
   ```bash
   $ kubectl apply -f ./manifests/connectivity-registry-certs/registry_certificate.yaml
   ```

#### Install Cross-cluster Connectivity components

1. Install the CRDs.
   ```bash
   $ kubectl apply -f ./manifests/crds
   ```

1. Install connectivity-registry.
   ```bash
   $ kubectl apply -f ./manifests/connectivity-registry
   ```

1. Install connectivity-binder.
   ```bash
   $ kubectl apply -f ./manifests/connectivity-binder
   ```

1. Install connectivity-dns.
   ```bash
   $ kubectl apply -f ./manifests/connectivity-dns
   ```

#### Peer the registry to the Cluster exporting Services

To consume Services from a Service registry, a `RemoteRegistry` resource must be
created. An example template is located in
`./manifests/example/remoteregistry/remoteregistry.yaml`.

The `REMOTE_REGISTRY_IP` in the `remoteregistry.yaml` must be replaced with the
address of the remote registry. By default, the `connectivity-registry` creates
a `NodePort` Service on port `30001`. The IP addresses of the Nodes and port
`30001` can be provided for connectivity to the Service Registry. Alternatively,
you can setup a Load Balancer to connect to the `connectivity-registry` if that
is preferable.

The `REGISTRY_CA_CERTIFICATE` must be replaced by the CA certificate of the
remote registry. It should also be base64 encoded when provided to the remote
registry resource.

The `REGISTRY_DOMAIN` must be replaced by the `REGISTRY_DOMAIN` you have chosen
above. For example, the Kind cluster setup uses
`shared-services-registry.xcc.test`.

Once everything is replaced, run:
```bash
$ kubectl apply -f ./manifests/example/remoteregistry/remoteregistry.yaml
```

#### Forward DNS zone to connectivity-dns server

Depending on what your cluster is using for in-cluster DNS, either Core DNS or
kube-dns. Follow the relevant instructions for your DNS server.

For more in-depth reading on configuring DNS in Kubernetes, please read
[this](https://kubernetes.io/docs/tasks/administer-cluster/dns-custom-nameservers/#configuration-of-stub-domain-and-upstream-nameserver-using-coredns)

#### Core DNS

Add a server block into the Corefile configmap of the in-cluster Core DNS. By
default, this is usually located at `coredns` in the `kube-system` namespace.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: coredns
  namespace: kube-system
data:
  Corefile: |
  ...

  SERVICES_DOMAIN {
      forward . CONNECTIVITY_DNS_SERVICE_CLUSTER_IP
      reload
  }
```

Replace `SERVICES_DOMAIN` with the domain you have chosen for cross-cluster
services.

Replace `CONNECTIVITY_DNS_SERVICE_CLUSTER_IP` with the `ClusterIP` of the
`connectivity-dns` Service. This can be retrieved by running:

```bash
$ kubectl get service -n cross-cluster-connectivity connectivity-dns
```

#### kube-dns

Add a `stub_domain` into the `kube-dns` configmap.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kube-dns
  namespace: kube-system
data:
  stubDomains: |
    {"SERVICES_DOMAIN" : ["CONNECTIVITY_DNS_SERVICE_CLUSTER_IP"]}
```

Replace `SERVICES_DOMAIN` with the domain you have chosen for cross-cluster
services.

Replace `CONNECTIVITY_DNS_SERVICE_CLUSTER_IP` with the `ClusterIP` of the
`connectivity-dns` Service. This can be retrieved by running:

```bash
$ kubectl get service -n cross-cluster-connectivity connectivity-dns
```
