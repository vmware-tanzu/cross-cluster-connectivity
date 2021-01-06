// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package gatewaydns

import (
	"context"

	discoveryv1beta1 "k8s.io/api/discovery/v1beta1"
	"k8s.io/apimachinery/pkg/types"

	clusterv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
)

type EndpointSliceReconciler struct {
	ClientProvider clientProvider
}

func (e *EndpointSliceReconciler) writeEndpointSlicesToClusters(ctx context.Context,
	clusters []clusterv1alpha3.Cluster, endpointSlices []discoveryv1beta1.EndpointSlice) error {

	for _, cluster := range clusters {
		clusterClient, err := e.ClientProvider.GetClient(ctx, types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      cluster.Name,
		})
		if err != nil {
			return err
		}

		for _, endpointSlice := range endpointSlices {
			err := clusterClient.Create(ctx, &endpointSlice)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
