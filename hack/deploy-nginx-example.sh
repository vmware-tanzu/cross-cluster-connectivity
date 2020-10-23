#!/bin/bash
# Copyright (c) 2020 VMware, Inc. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0


set -euxo pipefail

kubectl --kubeconfig ./shared-services.kubeconfig apply -f https://github.com/jetstack/cert-manager/releases/download/v0.11.0/cert-manager.yaml
kubectl --kubeconfig ./shared-services.kubeconfig wait --for=condition=Available --timeout=300s apiservice v1beta1.webhook.cert-manager.io
kubectl --kubeconfig ./shared-services.kubeconfig apply -f ./manifests/example/certs.yaml
kubectl --kubeconfig ./shared-services.kubeconfig apply -f ./manifests/example/nginx.yaml
kubectl --kubeconfig ./shared-services.kubeconfig apply -f ./manifests/example/exported_http_proxy.yaml

kubectl --kubeconfig ./workloads.kubeconfig run -it --rm --restart=Never \
  --image=curlimages/curl curl -- \
  curl -v -k "https://nginx.xcc.test"
