# Contributing

Thanks for taking the time to contribute to the project. These guidelines will
help you get started with the Cross-cluster Connectivity project.

## Running the tests

### Prerequisites

* [Golang](https://golang.org/) 1.13+
* [Ginkgo](https://github.com/onsi/ginkgo)

The majority of our tests use the [Ginkgo](https://github.com/onsi/ginkgo) BDD
framework to write our tests. Any new tests written should also be written using
[Ginkgo](https://github.com/onsi/ginkgo) and
[Gomega](https://github.com/onsi/gomega).

### Running the unit tests

To run the unit tests for the Cross-cluster Connectivity components, run the
following command:

```
make test-unit
```

## Building

When you want to build new images of the components, with local changes, to
deploy to a Kubernetes cluster, you can run:

```
make build-images
```

These images can then be deployed into a set of
[Kind](https://kind.sigs.k8s.io/) clusters by running:

```
make e2e-up
```

At the end of this you will have a local end-to-end development environment.
There will be a `management` cluster with [Cluster
API](https://cluster-api.sigs.k8s.io/) deployed on it and two workload clusters
called `cluster-a` and `cluster-b`.

### Running the end-to-end tests

These tests verify the end-to-end functionality of the Cross-cluster
Connectivity components. The test creates an example exported service on
`cluster-a` and tests connectivity to it from `cluster-b`.

With the clusters created by the `e2e-up` above, run the following command:

```
make test-cluster-api-dns
```

### Connecting to the clusters

After running `e2e-up`, there will be three kubeconfig files created in the repo
root: `management.kubeconfig`, `cluster-a.kubeconfig`, `cluster-b.kubeconfig`.
For example, to get the pods on `cluster-a` you can run:

```
kubectl --kubeconfig ./cluster-a.kubeconfig get pods -A
```

### Tearing down end-to-end development environment

Run the following to tear down the Kind clusters created by `e2e-up`:

```
make e2e-down
```

### Add license to source files

If you are introducing any new source files, then you must add the
[license](https://github.com/vmware-tanzu/cross-cluster-connectivity/blob/main/hack/license.txt)
to the top of every file. This can be done automatically using:

```
make addlicense
```

## Contributor Workflow

This section describes the process for contributing a bug fix or new feature.

### Before you submit a pull request

Please raise an issue before working on any code. This helps us discuss any
proposed changes to the project before you spend your valuable time working on
it.

### Guidelines for submitting a pull request

* Have a short subject on the first line and a body. The body can be empty.
* Use the imperative mood (ie "If applied, this commit will (subject)" should
  make sense).
* Target the `main` branch in your pull request. If `main` has moved on, you may need to rebase
  and resolve any conflicts before we can merge.
* Add any test cases where they make sense.
* Update any document as applicable.
* Add license to any new source files
* Sign the CLA

## CLA

We welcome contributions from everyone but we can only accept them if you sign
our Contributor License Agreement (CLA). If you would like to contribute and you
have not signed it, our CLA-bot will walk you through the process when you open
a Pull Request. For questions about the CLA process, see the
[FAQ](https://cla.vmware.com/faq) or submit a question through the GitHub issue
tracker.
