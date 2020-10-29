#!/bin/bash
# Copyright (c) 2020 VMware, Inc. All Rights Reserved.
# SPDX-License-Identifier: Apache-2.0


set -euxo pipefail

kubectl --kubeconfig ./shared-services.kubeconfig apply -f ./manifests/example/nginx/certs.yaml
kubectl --kubeconfig ./shared-services.kubeconfig apply -f ./manifests/example/nginx/nginx.yaml
kubectl --kubeconfig ./shared-services.kubeconfig apply -f ./manifests/example/nginx/exported_http_proxy.yaml

kubectl --kubeconfig ./workloads.kubeconfig run -it --rm --restart=Never \
  --image=curlimages/curl curl -- \
  curl -v -k "https://nginx.xcc.test"
