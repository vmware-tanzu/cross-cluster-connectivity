// SPDX-License-Identifier: Apache-2.0

package gatewaydns

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
)

type ClusterSearcher struct {
	client.Client
}

func (cs *ClusterSearcher) ListMatchingClusters(ctx context.Context,
	labelSelector metav1.LabelSelector) ([]clusterv1alpha3.Cluster, error) {

	selector, err := metav1.LabelSelectorAsSelector(&labelSelector)
	if err != nil {
		return nil, err
	}

	var matchingClusters clusterv1alpha3.ClusterList
	err = cs.Client.List(ctx, &matchingClusters, client.MatchingLabelsSelector{Selector: selector})
	if err != nil {
		return nil, err
	}
	return matchingClusters.Items, nil
}
