# 2. Kind, Concourse, and Btrfs

Date: 2020-08-05
Author: Christian Ang, Edwin Xie

## Status

Accepted

## Context

Our current CI/CD tool of choice is [Concourse](https://concourse-ci.org/),
which by default is deployed with Worker VMs using a
[btrfs](https://btrfs.wiki.kernel.org/index.php/Main_Page) filesystem type.

Our current testing strategy utilizes [Kind](https://kind.sigs.k8s.io/) clusters
that can be quickly spun up on a local development machine (or in this case a
remote worker VM).

* Concourse runs every task in a container on the Worker VMs. Kind uses Docker
  to create Kubernetes clusters. This means we have to run Docker within a
  container which requires us to do some Docker-in-Docker magic. We use
  [karlkfi/concourse-dcind](https://github.com/karlkfi/concourse-dcind) to
  perform this magic.
* Concourse worker VMs with a btrfs filesystem mounts a [loop
  device](https://man7.org/linux/man-pages/man4/loop.4.html) to the
  [container](https://github.com/concourse/baggageclaim/blob/60bc64b5559fb6008afdcabdeb00f9dd1bbf896e/fs/btrfs.go#L31-L65),
  but it doesn't create a file for the loop device in `/dev/` (e.g
  `/dev/loop0`). This causes a Kind cluster to fail to start because the Kubelet
  has
  [code](https://github.com/google/cadvisor/blob/10bce1cd55ba0d3510852a4ad7dbb057139097cb/fs/fs.go#L500)
  to `stat` the `/dev/loop0` device, which doesn't exist. There seems to be two
  solutions to this:
   * Set the `LocalStorageCapacityIsolation` feature flag on Kubelet to false.
     The Kubelet only seems to go down this code path for this specific feature
     flag (Relevant [Kubernetes Issue
     #80633](https://github.com/kubernetes/kubernetes/issues/80633) comment).
     Kind allows modifications to Kubelet via [Kubeadm Config
     Patches](https://kind.sigs.k8s.io/docs/user/configuration/#kubeadm-config-patches).
   * [mknod](https://man7.org/linux/man-pages/man1/mknod.1.html) the
     `/dev/loop0` device in the container, which allows the Kubelet to
     successfully `stat` the device and start. Loop devices are [global
     resources](https://lwn.net/Articles/819625/) and a loop device created with
     `mknod` within the container will be the same device as the one on the
     host. This does require a privileged container on Concourse to create this
     file, but that was also required for Docker-in-Docker.

## Decision

We are using `mknod` to create the loop device that is necessary for Kind to
start successfully e.g `mknod /dev/loop0 b 7 0`.

## Consequences

- Our end-to-end test is now successfully running in Concourse without having to
  provision an overlayfs worker.
