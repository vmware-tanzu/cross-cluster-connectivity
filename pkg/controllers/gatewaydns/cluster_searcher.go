// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package gatewaydns

import (
	"context"

	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
)

type ClusterSearcher struct {
	client.Client
}

func (cs *ClusterSearcher) ListMatchingClusters(ctx context.Context,
	gatewayDNS connectivityv1alpha1.GatewayDNS) ([]clusterv1alpha3.Cluster, error) {

	selector, err := metav1.LabelSelectorAsSelector(&gatewayDNS.Spec.ClusterSelector)
	if err != nil {
		return nil, err
	}

	var matchingClusters clusterv1alpha3.ClusterList
	err = cs.Client.List(ctx,
		&matchingClusters,
		client.MatchingLabelsSelector{Selector: selector},
		client.InNamespace(gatewayDNS.Namespace),
	)
	if err != nil {
		return nil, err
	}
	return matchingClusters.Items, nil
}
