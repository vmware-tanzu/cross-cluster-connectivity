// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package gatewaydns_test

import (
	"context"
	"errors"

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
		managementClient            client.Client
		gatewayClusterClient        client.Client
		workloadClusterClient       client.Client
		otherNamespaceClusterClient client.Client
		clusterClients              map[string]client.Client

		scheme *runtime.Scheme

		clientProvider *gatewaydnsfakes.FakeClientProvider

		gatewayDNSReconciler *gatewaydns.GatewayDNSReconciler

		gatewayDNS            *connectivityv1alpha1.GatewayDNS
		gatewayCluster        *clusterv1alpha3.Cluster
		workloadCluster       *clusterv1alpha3.Cluster
		otherNamespaceCluster *clusterv1alpha3.Cluster

		req ctrl.Request
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		_ = clientgoscheme.AddToScheme(scheme)
		_ = connectivityv1alpha1.AddToScheme(scheme)
		_ = clusterv1alpha3.AddToScheme(scheme)
		_ = discoveryv1beta1.AddToScheme(scheme)

		managementClient = fake.NewClientBuilder().WithScheme(scheme).Build()
		gatewayClusterClient = fake.NewClientBuilder().WithScheme(scheme).Build()
		workloadClusterClient = fake.NewClientBuilder().WithScheme(scheme).Build()
		otherNamespaceClusterClient = fake.NewClientBuilder().WithScheme(scheme).Build()

		clusterClients = make(map[string]client.Client)
		clusterClients["some-namespace/some-gateway-cluster"] = gatewayClusterClient
		clusterClients["some-namespace/some-workload-cluster"] = workloadClusterClient
		clusterClients["some-other-namespace/some-other-cluster"] = otherNamespaceClusterClient

		clientProvider = &gatewaydnsfakes.FakeClientProvider{}
		clientProvider.GetClientStub = func(ctx context.Context, namespacedName types.NamespacedName) (client.Client, error) {
			clusterClient, ok := clusterClients[namespacedName.String()]
			if !ok {
				return nil, errors.New("unexpected namespaced name")
			}
			return clusterClient, nil
		}

		ctrl.SetLogger(zap.New(
			zap.UseDevMode(true),
			zap.WriteTo(GinkgoWriter),
		))

		log := ctrl.Log.WithName("controllers").WithName("GatewayDNS")

		gatewayDNSReconciler = &gatewaydns.GatewayDNSReconciler{
			Client:          managementClient,
			Log:             log,
			Scheme:          managementClient.Scheme(),
			ClientProvider:  clientProvider,
			Namespace:       "capi-dns",
			DomainSuffix:    "xcc.test",
			ClusterSearcher: &gatewaydns.ClusterSearcher{Client: managementClient},
			EndpointSliceReconciler: &gatewaydns.EndpointSliceReconciler{
				Log:            log,
				ClientProvider: clientProvider,
			},
			ClusterGatewayCollector: &gatewaydns.ClusterGatewayCollector{
				Log:            log,
				ClientProvider: clientProvider,
			},
		}

		gatewayDNS = &connectivityv1alpha1.GatewayDNS{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-gateway-dns",
				Namespace: "some-namespace",
			},
			Spec: connectivityv1alpha1.GatewayDNSSpec{
				ClusterSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"cluster-with-gateway": "true",
					},
				},
				Service:        "some-service-namespace/some-gateway-service",
				ResolutionType: connectivityv1alpha1.ResolutionTypeLoadBalancer,
			},
		}

		err := managementClient.Create(context.Background(), gatewayDNS)
		Expect(err).NotTo(HaveOccurred())

		gatewayCluster = &clusterv1alpha3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-gateway-cluster",
				Namespace: "some-namespace",
				Labels: map[string]string{
					"cluster-with-gateway": "true",
				},
			},
		}

		err = managementClient.Create(context.Background(), gatewayCluster)
		Expect(err).NotTo(HaveOccurred())

		workloadCluster = &clusterv1alpha3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-workload-cluster",
				Namespace: "some-namespace",
			},
		}

		err = managementClient.Create(context.Background(), workloadCluster)
		Expect(err).NotTo(HaveOccurred())

		otherNamespaceCluster = &clusterv1alpha3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-other-cluster",
				Namespace: "some-other-namespace",
				Labels: map[string]string{
					"cluster-with-gateway": "true",
				},
			},
		}
		err = managementClient.Create(context.Background(), otherNamespaceCluster)
		Expect(err).NotTo(HaveOccurred())

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

		err = gatewayClusterClient.Create(context.Background(), service)
		Expect(err).NotTo(HaveOccurred())

		service = &corev1.Service{
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
							IP: "1.2.3.5",
						},
					},
				},
			},
		}

		err = otherNamespaceClusterClient.Create(context.Background(), service)
		Expect(err).NotTo(HaveOccurred())

		req.Name = gatewayDNS.Name
		req.Namespace = gatewayDNS.Namespace
	})

	Context("when a gateway dns resource matches a cluster", func() {
		It("creates an endpoint slice on all clusters in the gateway dns's namespace", func() {
			_, err := gatewayDNSReconciler.Reconcile(context.Background(), req)
			Expect(err).NotTo(HaveOccurred())

			endpointSlice := discoveryv1beta1.EndpointSlice{}
			err = gatewayClusterClient.Get(context.Background(), types.NamespacedName{
				Namespace: "capi-dns",
				Name:      "some-namespace-some-gateway-cluster-gateway",
			}, &endpointSlice)
			Expect(err).NotTo(HaveOccurred())

			Expect(endpointSlice.ObjectMeta.Name).To(Equal("some-namespace-some-gateway-cluster-gateway"))
			Expect(endpointSlice.ObjectMeta.Namespace).To(Equal("capi-dns"))
			Expect(endpointSlice.ObjectMeta.Annotations[connectivityv1alpha1.DNSHostnameAnnotation]).To(Equal("*.gateway.some-gateway-cluster.some-namespace.clusters.xcc.test"))
			Expect(endpointSlice.AddressType).To(Equal(discoveryv1beta1.AddressTypeIPv4))
			Expect(endpointSlice.Endpoints[0].Addresses).To(Equal([]string{"1.2.3.4"}))

			var endpointSliceList discoveryv1beta1.EndpointSliceList

			err = workloadClusterClient.List(context.Background(), &endpointSliceList)
			Expect(err).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(HaveLen(1))
			endpointSlice = endpointSliceList.Items[0]
			Expect(endpointSlice.ObjectMeta.Name).To(Equal("some-namespace-some-gateway-cluster-gateway"))
			Expect(endpointSlice.ObjectMeta.Namespace).To(Equal("capi-dns"))
			Expect(endpointSlice.ObjectMeta.Annotations[connectivityv1alpha1.DNSHostnameAnnotation]).To(Equal("*.gateway.some-gateway-cluster.some-namespace.clusters.xcc.test"))
			Expect(endpointSlice.AddressType).To(Equal(discoveryv1beta1.AddressTypeIPv4))
			Expect(endpointSlice.Endpoints[0].Addresses).To(Equal([]string{"1.2.3.4"}))

			err = otherNamespaceClusterClient.List(context.Background(), &endpointSliceList)
			Expect(err).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(HaveLen(0))
		})
	})

	Context("when a gateway dns is deleted", func() {
		BeforeEach(func() {
			_, err := gatewayDNSReconciler.Reconcile(context.Background(), req)
			Expect(err).NotTo(HaveOccurred())

			err = managementClient.Delete(context.Background(), gatewayDNS)
			Expect(err).NotTo(HaveOccurred())
		})

		It("deletes the associated endpoint slices", func() {
			_, err := gatewayDNSReconciler.Reconcile(context.Background(), req)
			Expect(err).NotTo(HaveOccurred())

			var endpointSliceList discoveryv1beta1.EndpointSliceList
			err = workloadClusterClient.List(context.Background(), &endpointSliceList)
			Expect(err).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(HaveLen(0))
		})
	})

	Context("when the label selector is changed on the gateway dns", func() {
		var (
			anotherGatewayCluster       *clusterv1alpha3.Cluster
			anotherGatewayClusterClient client.Client
		)

		BeforeEach(func() {
			anotherGatewayCluster = &clusterv1alpha3.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "another-gateway-cluster",
					Namespace: "some-namespace",
					Labels: map[string]string{
						"a-different-gateway-label": "true",
					},
				},
			}

			err := managementClient.Create(context.Background(), anotherGatewayCluster)
			Expect(err).NotTo(HaveOccurred())

			anotherGatewayClusterClient = fake.NewClientBuilder().WithScheme(scheme).Build()
			clusterClients["some-namespace/another-gateway-cluster"] = anotherGatewayClusterClient

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
								IP: "1.2.3.6",
							},
						},
					},
				},
			}

			err = anotherGatewayClusterClient.Create(context.Background(), service)
			Expect(err).NotTo(HaveOccurred())

			_, err = gatewayDNSReconciler.Reconcile(context.Background(), req)
			Expect(err).NotTo(HaveOccurred())

			gatewayDNS.Spec.ClusterSelector.MatchLabels = map[string]string{
				"a-different-gateway-label": "true",
			}

			err = managementClient.Update(context.Background(), gatewayDNS)
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates an endpoint slice for newly matched clusters and deletes endpoints slices for previously matched clusters", func() {
			_, err := gatewayDNSReconciler.Reconcile(context.Background(), req)
			Expect(err).NotTo(HaveOccurred())

			var endpointSliceList discoveryv1beta1.EndpointSliceList
			err = workloadClusterClient.List(context.Background(), &endpointSliceList)
			Expect(err).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(HaveLen(1))

			endpointSlice := endpointSliceList.Items[0]
			Expect(endpointSlice.ObjectMeta.Name).To(Equal("some-namespace-another-gateway-cluster-gateway"))
			Expect(endpointSlice.ObjectMeta.Namespace).To(Equal("capi-dns"))
			Expect(endpointSlice.ObjectMeta.Annotations[connectivityv1alpha1.DNSHostnameAnnotation]).To(Equal("*.gateway.another-gateway-cluster.some-namespace.clusters.xcc.test"))
			Expect(endpointSlice.AddressType).To(Equal(discoveryv1beta1.AddressTypeIPv4))
			Expect(endpointSlice.Endpoints[0].Addresses).To(Equal([]string{"1.2.3.6"}))

			err = anotherGatewayClusterClient.List(context.Background(), &endpointSliceList)
			Expect(err).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(HaveLen(1))

			endpointSlice = endpointSliceList.Items[0]
			Expect(endpointSlice.ObjectMeta.Name).To(Equal("some-namespace-another-gateway-cluster-gateway"))
			Expect(endpointSlice.ObjectMeta.Namespace).To(Equal("capi-dns"))
			Expect(endpointSlice.ObjectMeta.Annotations[connectivityv1alpha1.DNSHostnameAnnotation]).To(Equal("*.gateway.another-gateway-cluster.some-namespace.clusters.xcc.test"))
			Expect(endpointSlice.AddressType).To(Equal(discoveryv1beta1.AddressTypeIPv4))
			Expect(endpointSlice.Endpoints[0].Addresses).To(Equal([]string{"1.2.3.6"}))

			err = gatewayClusterClient.List(context.Background(), &endpointSliceList)
			Expect(err).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(HaveLen(1))

			endpointSlice = endpointSliceList.Items[0]
			Expect(endpointSlice.ObjectMeta.Name).To(Equal("some-namespace-another-gateway-cluster-gateway"))
			Expect(endpointSlice.ObjectMeta.Namespace).To(Equal("capi-dns"))
			Expect(endpointSlice.ObjectMeta.Annotations[connectivityv1alpha1.DNSHostnameAnnotation]).To(Equal("*.gateway.another-gateway-cluster.some-namespace.clusters.xcc.test"))
			Expect(endpointSlice.AddressType).To(Equal(discoveryv1beta1.AddressTypeIPv4))
			Expect(endpointSlice.Endpoints[0].Addresses).To(Equal([]string{"1.2.3.6"}))
		})
	})

	Context("when the service changes on the gateway dns", func() {
		BeforeEach(func() {
			_, err := gatewayDNSReconciler.Reconcile(context.Background(), req)
			Expect(err).NotTo(HaveOccurred())

			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "a-different-gateway-service",
					Namespace: "some-service-namespace",
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
				},
				Status: corev1.ServiceStatus{
					LoadBalancer: corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{
							corev1.LoadBalancerIngress{
								IP: "1.2.3.7",
							},
						},
					},
				},
			}

			err = gatewayClusterClient.Create(context.Background(), service)
			Expect(err).NotTo(HaveOccurred())

			gatewayDNS.Spec.Service = "some-service-namespace/a-different-gateway-service"

			err = managementClient.Update(context.Background(), gatewayDNS)
			Expect(err).NotTo(HaveOccurred())
		})

		It("updates the endpoint slices with the new service ip", func() {
			_, err := gatewayDNSReconciler.Reconcile(context.Background(), req)
			Expect(err).NotTo(HaveOccurred())

			var endpointSliceList discoveryv1beta1.EndpointSliceList
			err = workloadClusterClient.List(context.Background(), &endpointSliceList)
			Expect(err).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(HaveLen(1))

			endpointSlice := endpointSliceList.Items[0]
			Expect(endpointSlice.ObjectMeta.Name).To(Equal("some-namespace-some-gateway-cluster-gateway"))
			Expect(endpointSlice.ObjectMeta.Namespace).To(Equal("capi-dns"))
			Expect(endpointSlice.ObjectMeta.Annotations[connectivityv1alpha1.DNSHostnameAnnotation]).To(Equal("*.gateway.some-gateway-cluster.some-namespace.clusters.xcc.test"))
			Expect(endpointSlice.AddressType).To(Equal(discoveryv1beta1.AddressTypeIPv4))
			Expect(endpointSlice.Endpoints[0].Addresses).To(Equal([]string{"1.2.3.7"}))

			err = gatewayClusterClient.List(context.Background(), &endpointSliceList)
			Expect(err).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(HaveLen(1))

			endpointSlice = endpointSliceList.Items[0]
			Expect(endpointSlice.ObjectMeta.Name).To(Equal("some-namespace-some-gateway-cluster-gateway"))
			Expect(endpointSlice.ObjectMeta.Namespace).To(Equal("capi-dns"))
			Expect(endpointSlice.ObjectMeta.Annotations[connectivityv1alpha1.DNSHostnameAnnotation]).To(Equal("*.gateway.some-gateway-cluster.some-namespace.clusters.xcc.test"))
			Expect(endpointSlice.AddressType).To(Equal(discoveryv1beta1.AddressTypeIPv4))
			Expect(endpointSlice.Endpoints[0].Addresses).To(Equal([]string{"1.2.3.7"}))
		})
	})
})
