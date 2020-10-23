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

When you want to build new images of the components to deploy to a Kubernetes
cluster, you can run:

```
make build-images
```

This will take any local changes and build new docker images. These images can
then be deployed into a Kind cluster by running:

```
make e2e-up
```

### Running the end-to-end tests

These tests verify the end-to-end functionality of the Cross-cluster
Connectivity components. The test creates an example exported service on the
shared-services cluster and tests connectivity to it from the workload cluster.

With the clusters created by the instructions above, run the following command:

```
make test-connectivity
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
* Pull requests target develop. If develop has moved on, you may need to rebase
  before we can merge.
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
