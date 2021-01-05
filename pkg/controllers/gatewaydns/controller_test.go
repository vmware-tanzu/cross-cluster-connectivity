// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package gatewaydns_test

import (
	"context"

	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/gatewaydns"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/gatewaydns/gatewaydnsfakes"
	corev1 "k8s.io/api/core/v1"
	discoveryv1beta1 "k8s.io/api/discovery/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Reconcile", func() {
	var (
		managementClient client.Client
		clusterClient    client.Client
		clientProvider   *gatewaydnsfakes.FakeClientProvider

		gatewayDNSReconciler *gatewaydns.GatewayDNSReconciler

		gatewayDNS *connectivityv1alpha1.GatewayDNS
		cluster    *clusterv1alpha3.Cluster

		req ctrl.Request
	)

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		_ = clientgoscheme.AddToScheme(scheme)
		_ = connectivityv1alpha1.AddToScheme(scheme)
		_ = clusterv1alpha3.AddToScheme(scheme)
		_ = discoveryv1beta1.AddToScheme(scheme)

		managementClient = fake.NewClientBuilder().WithScheme(scheme).Build()
		clusterClient = fake.NewClientBuilder().WithScheme(scheme).Build()

		clientProvider = &gatewaydnsfakes.FakeClientProvider{}
		clientProvider.GetClientReturns(clusterClient, nil)

		ctrl.SetLogger(zap.New(
			zap.UseDevMode(true),
			zap.WriteTo(GinkgoWriter),
		))

		gatewayDNSReconciler = &gatewaydns.GatewayDNSReconciler{
			Client:         managementClient,
			Log:            ctrl.Log.WithName("controllers").WithName("GatewayDNS"),
			Scheme:         managementClient.Scheme(),
			ClientProvider: clientProvider,
		}

		gatewayDNS = &connectivityv1alpha1.GatewayDNS{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-gateway-dns",
				Namespace: "some-namespace",
			},
			Spec: connectivityv1alpha1.GatewayDNSSpec{
				ClusterSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"some-label": "true",
					},
				},
				Service:        "some-service-namespace/some-gateway-service",
				ResolutionType: connectivityv1alpha1.ResolutionTypeLoadBalancer,
			},
		}

		err := managementClient.Create(context.Background(), gatewayDNS)
		Expect(err).NotTo(HaveOccurred())

		cluster = &clusterv1alpha3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-cluster",
				Namespace: "some-namespace",
				Labels: map[string]string{
					"some-label": "true",
				},
			},
		}

		err = managementClient.Create(context.Background(), cluster)
		Expect(err).NotTo(HaveOccurred())

		req.Name = gatewayDNS.Name
		req.Namespace = gatewayDNS.Namespace
	})

	Context("when a gateway dns resource matches a cluster", func() {
		BeforeEach(func() {
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-gateway-service",
					Namespace: "some-service-namespace",
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
				},
				Status: corev1.ServiceStatus{
					LoadBalancer: corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{
							corev1.LoadBalancerIngress{
								IP: "1.2.3.4",
							},
						},
					},
				},
			}

			err := clusterClient.Create(context.Background(), service)
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates an endpoint slice", func() {
			_, err := gatewayDNSReconciler.Reconcile(context.Background(), req)
			Expect(err).NotTo(HaveOccurred())

			endpointSlice := &discoveryv1beta1.EndpointSlice{}
			err = clusterClient.Get(context.Background(), types.NamespacedName{
				Namespace: "capi-dns",
				Name:      "some-namespace-some-cluster-gateway",
			}, endpointSlice)
			Expect(err).NotTo(HaveOccurred())

			Expect(endpointSlice.ObjectMeta.Name).To(Equal("some-namespace-some-cluster-gateway"))
			Expect(endpointSlice.ObjectMeta.Namespace).To(Equal("capi-dns"))
			Expect(endpointSlice.ObjectMeta.Annotations[connectivityv1alpha1.DNSHostnameAnnotation]).To(Equal("*.gateway.some-cluster.some-namespace.clusters.xcc.test"))
			Expect(endpointSlice.AddressType).To(Equal(discoveryv1beta1.AddressTypeIPv4))
			Expect(endpointSlice.Endpoints[0].Addresses).To(Equal([]string{"1.2.3.4"}))
		})
	})

	Context("when a service has no external IPs", func() {
		BeforeEach(func() {
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-gateway-service",
					Namespace: "some-service-namespace",
				},
				Spec: corev1.ServiceSpec{
					Type:        corev1.ServiceTypeLoadBalancer,
					ExternalIPs: []string{},
				},
			}

			err := clusterClient.Create(context.Background(), service)
			Expect(err).NotTo(HaveOccurred())
		})

		It("does not create an endpoint slice", func() {
			_, err := gatewayDNSReconciler.Reconcile(context.Background(), req)
			Expect(err).NotTo(HaveOccurred())

			endpointSlices := &discoveryv1beta1.EndpointSliceList{}
			err = clusterClient.List(context.Background(), endpointSlices)
			Expect(err).NotTo(HaveOccurred())
			Expect(endpointSlices.Items).To(HaveLen(0))
		})
	})
})
