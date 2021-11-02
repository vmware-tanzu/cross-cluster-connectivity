// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package gatewaydns

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
)

type ClusterGateway struct {
	ClusterNamespacedName    types.NamespacedName
	Gateway                  *corev1.Service
	Unreachable              bool
	DomainSuffix             string
	ControllerNamespace      string // xcc-test by default, where xcc-dns-controller and dns-server are deployed
	GatewayDNSNamespacedName types.NamespacedName
}

func (cg ClusterGateway) ToEndpointSlice() discoveryv1.EndpointSlice {
	hostname := fmt.Sprintf("*.gateway.%s.%s.clusters.%s",
		cg.ClusterNamespacedName.Name,
		cg.ClusterNamespacedName.Namespace,
		cg.DomainSuffix,
	)
	addresses := []string{}
	for _, ingress := range cg.Gateway.Status.LoadBalancer.Ingress {
		addresses = append(addresses, ingress.IP)
	}

	return discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cg.endpointSliceName(),
			Namespace: cg.ControllerNamespace,
			Annotations: map[string]string{
				connectivityv1alpha1.DNSHostnameAnnotation:   hostname,
				connectivityv1alpha1.GatewayDNSRefAnnotation: cg.GatewayDNSNamespacedName.String(),
			},
			Labels: map[string]string{
				"kubernetes.io/service-name": cg.endpointSliceName(),
			},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: addresses,
			},
		},
	}
}

func (cg ClusterGateway) endpointSliceName() string {
	return fmt.Sprintf("%s-%s-gateway", cg.ClusterNamespacedName.Namespace, cg.ClusterNamespacedName.Name)
}

func (cg ClusterGateway) EndpointSliceKey() string {
	return fmt.Sprintf("%s/%s", cg.ControllerNamespace, cg.endpointSliceName())
}

func EndpointSliceKey(endpointSlice discoveryv1.EndpointSlice) string {
	return fmt.Sprintf("%s/%s", endpointSlice.Namespace, endpointSlice.Name)
}
