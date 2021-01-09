// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package gatewaydns

import (
	"context"
	"fmt"
	"reflect"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	discoveryv1beta1 "k8s.io/api/discovery/v1beta1"
	"k8s.io/apimachinery/pkg/types"

	clusterv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type EndpointSliceReconciler struct {
	ClientProvider clientProvider
	Namespace      string
}

func (e *EndpointSliceReconciler) ConvergeEndpointSlicesToClusters(ctx context.Context,
	clusters []clusterv1alpha3.Cluster, gatewayDNSNamespacedName types.NamespacedName, desiredEndpointSlices []discoveryv1beta1.EndpointSlice) error {

	for _, cluster := range clusters {
		clusterClient, err := e.ClientProvider.GetClient(ctx, types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      cluster.Name,
		})
		if err != nil {
			return err
		}

		err = e.convergeCluster(ctx, gatewayDNSNamespacedName, clusterClient, desiredEndpointSlices)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *EndpointSliceReconciler) convergeCluster(ctx context.Context, gatewayDNSNamespacedName types.NamespacedName, clusterClient client.Client, desiredEndpointSlices []discoveryv1beta1.EndpointSlice) error {
	clusterDiff, err := e.diffCluster(ctx, gatewayDNSNamespacedName, clusterClient, desiredEndpointSlices)
	if err != nil {
		return err
	}

	for _, endpointSlice := range clusterDiff.missing {
		err = clusterClient.Create(ctx, &endpointSlice)
		if err != nil {
			if k8serrors.IsAlreadyExists(err) {
				var existingEndpointSlice discoveryv1beta1.EndpointSlice
				namespacedName := types.NamespacedName{
					Namespace: endpointSlice.Namespace,
					Name:      endpointSlice.Name,
				}
				err = clusterClient.Get(ctx, namespacedName, &existingEndpointSlice)
				if err != nil {
					return err
				}
				existingEndpointSlice = merge(endpointSlice, existingEndpointSlice)
				err = clusterClient.Update(ctx, &existingEndpointSlice)
				if err != nil {
					return err
				}
				continue
			}
			return err
		}
	}

	for _, endpointSlice := range clusterDiff.changed {
		err = clusterClient.Update(ctx, &endpointSlice)
		if err != nil {
			return err
		}
	}

	for _, endpointSlice := range clusterDiff.undesired {
		err = clusterClient.Delete(ctx, &endpointSlice)
		if err != nil {
			return err
		}
	}

	//TODO: log end state

	return nil
}

type ClusterDiff struct {
	undesired []discoveryv1beta1.EndpointSlice
	missing   []discoveryv1beta1.EndpointSlice
	changed   []discoveryv1beta1.EndpointSlice
}

func (e *EndpointSliceReconciler) diffCluster(ctx context.Context, gatewayDNSNamespacedName types.NamespacedName, clusterClient client.Client, desiredEndpointSlices []discoveryv1beta1.EndpointSlice) (ClusterDiff, error) {
	existingEndpointSliceList := &discoveryv1beta1.EndpointSliceList{}
	err := clusterClient.List(ctx, existingEndpointSliceList, client.InNamespace(e.Namespace))
	if err != nil {
		return ClusterDiff{}, err
	}

	existingEndpointSliceMap := make(map[string]discoveryv1beta1.EndpointSlice)
	for _, item := range existingEndpointSliceList.Items {
		if _, ok := item.Annotations[connectivityv1alpha1.DNSHostnameAnnotation]; !ok {
			continue
		}

		existingGatewayDNSNamespacedName, ok := item.Annotations[connectivityv1alpha1.GatewayDNSRefAnnotation]
		if !ok || existingGatewayDNSNamespacedName != gatewayDNSNamespacedName.String() {
			continue
		}

		existingEndpointSliceMap[endpointSliceKey(item)] = item
	}

	desiredEndpointSliceMap := make(map[string]discoveryv1beta1.EndpointSlice)
	for _, item := range desiredEndpointSlices {
		desiredEndpointSliceMap[endpointSliceKey(item)] = item
	}

	clusterDiff := ClusterDiff{}
	for _, item := range desiredEndpointSliceMap {
		if existingItem, ok := existingEndpointSliceMap[endpointSliceKey(item)]; ok {
			if !compareEndpointSlices(item, existingItem) {
				existingItem = merge(item, existingItem)
				clusterDiff.changed = append(clusterDiff.changed, existingItem)
			}
		} else {
			clusterDiff.missing = append(clusterDiff.missing, item)
		}
	}

	for _, item := range existingEndpointSliceMap {
		if _, ok := desiredEndpointSliceMap[endpointSliceKey(item)]; !ok {
			clusterDiff.undesired = append(clusterDiff.undesired, item)
		}
	}

	return clusterDiff, nil
}

func endpointSliceKey(endpointSlice discoveryv1beta1.EndpointSlice) string {
	return fmt.Sprintf("%s/%s", endpointSlice.Namespace, endpointSlice.Name)
}

func merge(source, dest discoveryv1beta1.EndpointSlice) discoveryv1beta1.EndpointSlice {
	if dest.Annotations == nil {
		dest.Annotations = map[string]string{}
	}
	dest.Annotations[connectivityv1alpha1.DNSHostnameAnnotation] = source.Annotations[connectivityv1alpha1.DNSHostnameAnnotation]
	dest.Annotations[connectivityv1alpha1.GatewayDNSRefAnnotation] = source.Annotations[connectivityv1alpha1.GatewayDNSRefAnnotation]
	dest.AddressType = source.AddressType
	dest.Endpoints = source.Endpoints
	dest.Ports = source.Ports
	return dest
}

func compareEndpointSlices(a, b discoveryv1beta1.EndpointSlice) bool {
	return a.Annotations[connectivityv1alpha1.DNSHostnameAnnotation] == b.Annotations[connectivityv1alpha1.DNSHostnameAnnotation] &&
		a.AddressType == b.AddressType &&
		reflect.DeepEqual(a.Endpoints, b.Endpoints) &&
		reflect.DeepEqual(a.Ports, b.Ports)
}
