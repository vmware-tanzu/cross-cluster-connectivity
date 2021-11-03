// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package gatewaydns_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/gatewaydns"
	corev1 "k8s.io/api/core/v1"
	discoveryv1beta1 "k8s.io/api/discovery/v1beta1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("ClusterGateway", func() {
	var clusterGateways []gatewaydns.ClusterGateway

	BeforeEach(func() {
		clusterGateways = []gatewaydns.ClusterGateway{
			{
				ClusterNamespacedName: types.NamespacedName{
					Name:      "cluster-name-foo",
					Namespace: "cluster-namespace-foo",
				},
				ControllerNamespace: "xcc-dns",
				DomainSuffix:        "xcc.test",
				GatewayDNSNamespacedName: types.NamespacedName{
					Name:      "gateway-dns-name",
					Namespace: "gateway-dns-namespace",
				},
				Gateway: &corev1.Service{
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{{IP: "1.1.0.1"}},
						},
					},
				},
			},
			{
				ClusterNamespacedName: types.NamespacedName{
					Name:      "cluster-name-bar",
					Namespace: "cluster-namespace-bar",
				},
				ControllerNamespace: "xcc-dns",
				DomainSuffix:        "xcc.test",
				GatewayDNSNamespacedName: types.NamespacedName{
					Name:      "gateway-dns-name",
					Namespace: "gateway-dns-namespace",
				},
				Gateway: &corev1.Service{
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{{IP: "1.1.0.2"}},
						},
					},
				},
			},
			{
				ClusterNamespacedName: types.NamespacedName{
					Name:      "cluster-name-baz",
					Namespace: "cluster-namespace-baz",
				},
				ControllerNamespace: "xcc-dns",
				DomainSuffix:        "xcc.test",
				GatewayDNSNamespacedName: types.NamespacedName{
					Name:      "gateway-dns-name",
					Namespace: "gateway-dns-namespace",
				},
				Gateway: &corev1.Service{
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{{IP: "1.1.0.3"}},
						},
					},
				},
			},
		}
	})

	It("Transforms Cluster Gateways into Endpoint Slices", func() {
		endpointSlice := clusterGateways[0].ToEndpointSlice()
		Expect(endpointSlice.Name).To(Equal("cluster-namespace-foo-cluster-name-foo-gateway"))
		Expect(endpointSlice.Namespace).To(Equal("xcc-dns"))
		Expect(endpointSlice.Annotations[connectivityv1alpha1.DNSHostnameAnnotation]).To(Equal("*.gateway.cluster-name-foo.cluster-namespace-foo.clusters.xcc.test"))
		Expect(endpointSlice.Annotations[connectivityv1alpha1.GatewayDNSRefAnnotation]).To(Equal("gateway-dns-namespace/gateway-dns-name"))
		Expect(endpointSlice.AddressType).To(Equal(discoveryv1beta1.AddressTypeIPv4))
		Expect(endpointSlice.Endpoints).To(HaveLen(1))
		Expect(endpointSlice.Endpoints[0].Addresses).To(ConsistOf("1.1.0.1"))
		Expect(endpointSlice.Labels["kubernetes.io/service-name"]).To(Equal("cluster-namespace-foo-cluster-name-foo-gateway"))

		endpointSlice = clusterGateways[1].ToEndpointSlice()
		Expect(endpointSlice.Name).To(Equal("cluster-namespace-bar-cluster-name-bar-gateway"))
		Expect(endpointSlice.Namespace).To(Equal("xcc-dns"))
		Expect(endpointSlice.Annotations[connectivityv1alpha1.DNSHostnameAnnotation]).To(Equal("*.gateway.cluster-name-bar.cluster-namespace-bar.clusters.xcc.test"))
		Expect(endpointSlice.Annotations[connectivityv1alpha1.GatewayDNSRefAnnotation]).To(Equal("gateway-dns-namespace/gateway-dns-name"))
		Expect(endpointSlice.AddressType).To(Equal(discoveryv1beta1.AddressTypeIPv4))
		Expect(endpointSlice.Endpoints).To(HaveLen(1))
		Expect(endpointSlice.Endpoints[0].Addresses).To(ConsistOf("1.1.0.2"))
		Expect(endpointSlice.Labels["kubernetes.io/service-name"]).To(Equal("cluster-namespace-bar-cluster-name-bar-gateway"))

		endpointSlice = clusterGateways[2].ToEndpointSlice()
		Expect(endpointSlice.Name).To(Equal("cluster-namespace-baz-cluster-name-baz-gateway"))
		Expect(endpointSlice.Namespace).To(Equal("xcc-dns"))
		Expect(endpointSlice.Annotations[connectivityv1alpha1.DNSHostnameAnnotation]).To(Equal("*.gateway.cluster-name-baz.cluster-namespace-baz.clusters.xcc.test"))
		Expect(endpointSlice.Annotations[connectivityv1alpha1.GatewayDNSRefAnnotation]).To(Equal("gateway-dns-namespace/gateway-dns-name"))
		Expect(endpointSlice.AddressType).To(Equal(discoveryv1beta1.AddressTypeIPv4))
		Expect(endpointSlice.Endpoints).To(HaveLen(1))
		Expect(endpointSlice.Endpoints[0].Addresses).To(ConsistOf("1.1.0.3"))
		Expect(endpointSlice.Labels["kubernetes.io/service-name"]).To(Equal("cluster-namespace-baz-cluster-name-baz-gateway"))
	})
})
