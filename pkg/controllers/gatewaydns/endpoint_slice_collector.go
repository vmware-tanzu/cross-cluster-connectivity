// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package gatewaydns

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	discoveryv1beta1 "k8s.io/api/discovery/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	clusterv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
)

type EndpointSliceCollector struct {
	Log            logr.Logger
	ClientProvider clientProvider
}

func (e *EndpointSliceCollector) CreateEndpointSlicesForClusters(ctx context.Context,
	gatewayDNS connectivityv1alpha1.GatewayDNS,
	clusters []clusterv1alpha3.Cluster) ([]discoveryv1beta1.EndpointSlice, error) {

	gatewayDNSSpecService := newNamespacedNameFromString(gatewayDNS.Spec.Service)

	var endpointSlices []discoveryv1beta1.EndpointSlice
	for _, cluster := range clusters {
		service, err := e.getLoadBalancerServiceForCluster(ctx, gatewayDNSSpecService, cluster)
		if err != nil {
			return nil, err
		}
		if service != nil {
			e.Log.Info("Get Load Balancer Service: ", "cluster", cluster.ClusterName, "service", service)
			endpointSlices = append(endpointSlices, convertServiceToEndpointSlice(service, cluster.ObjectMeta.Name, gatewayDNS.Namespace))
		}
	}
	return endpointSlices, nil
}

func (e *EndpointSliceCollector) getLoadBalancerServiceForCluster(ctx context.Context,
	serviceNamespacedName types.NamespacedName,
	cluster clusterv1alpha3.Cluster) (*corev1.Service, error) {

	clusterClient, err := e.ClientProvider.GetClient(ctx, types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      cluster.Name,
	})
	if err != nil {
		return nil, err
	}

	var service corev1.Service
	err = clusterClient.Get(ctx, serviceNamespacedName, &service)
	if err != nil {
		return nil, err
	}

	e.Log.Info("Get Service: ", "cluster", cluster.Name, "serviceNamespacedName", serviceNamespacedName, "service", service)
	if isLoadBalancerWithExternalIP(service) {
		return &service, nil
	}
	return nil, nil
}

func newNamespacedNameFromString(s string) types.NamespacedName {
	namespacedName := types.NamespacedName{}
	result := strings.Split(s, string(types.Separator))
	if len(result) == 2 {
		namespacedName.Namespace = result[0]
		namespacedName.Name = result[1]
	}
	return namespacedName
}

func isLoadBalancerWithExternalIP(service corev1.Service) bool {
	if service.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return false
	}
	if len(service.Status.LoadBalancer.Ingress) == 0 {
		return false
	}
	return true
}

func convertServiceToEndpointSlice(service *corev1.Service, clusterName string, gatewayDNSNamespace string) discoveryv1beta1.EndpointSlice {
	// TODO: xcc.test TLD should be a configuration option
	hostname := fmt.Sprintf("*.gateway.%s.%s.clusters.xcc.test", clusterName, gatewayDNSNamespace)
	name := fmt.Sprintf("%s-%s-gateway", gatewayDNSNamespace, clusterName)
	addresses := []string{}

	for _, ingress := range service.Status.LoadBalancer.Ingress {
		addresses = append(addresses, ingress.IP)
	}

	return discoveryv1beta1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "capi-dns",
			Annotations: map[string]string{
				connectivityv1alpha1.DNSHostnameAnnotation: hostname,
			},
		},
		AddressType: discoveryv1beta1.AddressTypeIPv4,
		Endpoints: []discoveryv1beta1.Endpoint{
			{
				Addresses: addresses,
			},
		},
	}
}
