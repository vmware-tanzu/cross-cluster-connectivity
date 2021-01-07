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
)

var _ = Describe("Endpoint Slice Creator", func() {
	var (
		clusterGateways []gatewaydns.ClusterGateway
	)

	BeforeEach(func() {
		clusterGateways = []gatewaydns.ClusterGateway{
			{
				ClusterName: "foo",
				Gateway: corev1.Service{
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								corev1.LoadBalancerIngress{
									IP: "1.1.0.1",
								},
							},
						},
					},
				},
			},
			{
				ClusterName: "bar",
				Gateway: corev1.Service{
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								corev1.LoadBalancerIngress{
									IP: "1.1.0.2",
								},
							},
						},
					},
				},
			},
			{
				ClusterName: "baz",
				Gateway: corev1.Service{
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								corev1.LoadBalancerIngress{
									IP: "1.1.0.3",
								},
							},
						},
					},
				},
			},
		}

	})

	It("Transforms Cluster Gateways into Endpoint Slices", func() {
		endpointSlices := gatewaydns.ConvertGatewaysToEndpointSlices(clusterGateways, "gateway-dns-namespace")
		Expect(endpointSlices).To(HaveLen(3))
		Expect(endpointSlices[0].Name).To(Equal("gateway-dns-namespace-foo-gateway"))
		Expect(endpointSlices[0].Namespace).To(Equal("capi-dns"))
		Expect(endpointSlices[0].Annotations[connectivityv1alpha1.DNSHostnameAnnotation]).To(Equal("*.gateway.foo.gateway-dns-namespace.clusters.xcc.test"))
		Expect(endpointSlices[0].AddressType).To(Equal(discoveryv1beta1.AddressTypeIPv4))
		Expect(endpointSlices[0].Endpoints).To(HaveLen(1))
		Expect(endpointSlices[0].Endpoints[0].Addresses).To(HaveLen(1))
		Expect(endpointSlices[0].Endpoints[0].Addresses[0]).To(Equal("1.1.0.1"))

		Expect(endpointSlices[1].Name).To(Equal("gateway-dns-namespace-bar-gateway"))
		Expect(endpointSlices[1].Namespace).To(Equal("capi-dns"))
		Expect(endpointSlices[1].Annotations[connectivityv1alpha1.DNSHostnameAnnotation]).To(Equal("*.gateway.bar.gateway-dns-namespace.clusters.xcc.test"))
		Expect(endpointSlices[1].AddressType).To(Equal(discoveryv1beta1.AddressTypeIPv4))
		Expect(endpointSlices[1].Endpoints).To(HaveLen(1))
		Expect(endpointSlices[1].Endpoints[0].Addresses).To(HaveLen(1))
		Expect(endpointSlices[1].Endpoints[0].Addresses[0]).To(Equal("1.1.0.2"))

		Expect(endpointSlices[2].Name).To(Equal("gateway-dns-namespace-baz-gateway"))
		Expect(endpointSlices[2].Namespace).To(Equal("capi-dns"))
		Expect(endpointSlices[2].Annotations[connectivityv1alpha1.DNSHostnameAnnotation]).To(Equal("*.gateway.baz.gateway-dns-namespace.clusters.xcc.test"))
		Expect(endpointSlices[2].AddressType).To(Equal(discoveryv1beta1.AddressTypeIPv4))
		Expect(endpointSlices[2].Endpoints).To(HaveLen(1))
		Expect(endpointSlices[2].Endpoints[0].Addresses).To(HaveLen(1))
		Expect(endpointSlices[2].Endpoints[0].Addresses[0]).To(Equal("1.1.0.3"))
	})
})
