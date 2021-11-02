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
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Endpoint Slice Reconciler", func() {
	var (
		endpointSliceReconciler  gatewaydns.EndpointSliceReconciler
		clientProvider           *gatewaydnsfakes.FakeClientProvider
		clusterClient0           client.Client
		clusterClient1           client.Client
		clusterClients           map[string]client.Client
		gatewayDNSNamespacedName types.NamespacedName
		endpointSlices           []discoveryv1.EndpointSlice
		clusters                 []clusterv1beta1.Cluster
		namespace                string
		clusterGateways          []gatewaydns.ClusterGateway
	)

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		_ = clientgoscheme.AddToScheme(scheme)
		_ = connectivityv1alpha1.AddToScheme(scheme)
		_ = clusterv1beta1.AddToScheme(scheme)
		_ = discoveryv1.AddToScheme(scheme)

		clusterClient0 = fake.NewClientBuilder().WithScheme(scheme).Build()
		clusterClient1 = fake.NewClientBuilder().WithScheme(scheme).Build()

		clusterClients = make(map[string]client.Client)
		clusterClients["cluster-namespace-0/cluster-name-0"] = clusterClient0
		clusterClients["cluster-namespace-1/cluster-name-1"] = clusterClient1

		clientProvider = &gatewaydnsfakes.FakeClientProvider{}
		clientProvider.GetClientStub = func(ctx context.Context, namespacedName types.NamespacedName) (client.Client, error) {
			clusterClient, ok := clusterClients[namespacedName.String()]
			if !ok {
				return nil, errors.New("unexpected namespaced name")
			}
			return clusterClient, nil
		}
		namespace = "xcc-dns"
		domainSuffix := "xcc.test"

		ctrl.SetLogger(zap.New(
			zap.UseDevMode(true),
			zap.WriteTo(GinkgoWriter),
		))

		log := ctrl.Log.WithName("controllers").WithName("GatewayDNS")

		endpointSliceReconciler = gatewaydns.EndpointSliceReconciler{
			ClientProvider: clientProvider,
			Namespace:      namespace,
			Log:            log,
		}

		gatewayDNSNamespacedName = types.NamespacedName{
			Namespace: "gateway-dns-namespace",
			Name:      "gateway-dns-name",
		}

		clusterGateways = []gatewaydns.ClusterGateway{
			{
				ClusterNamespacedName: types.NamespacedName{
					Name:      "cluster-name-0",
					Namespace: "cluster-namespace-0",
				},
				ControllerNamespace:      namespace,
				DomainSuffix:             domainSuffix,
				GatewayDNSNamespacedName: gatewayDNSNamespacedName,
				Gateway: &corev1.Service{
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{{IP: "1.1.0.1"}},
						},
					},
				},
			},
			{
				ClusterNamespacedName: types.NamespacedName{
					Name:      "cluster-name-1",
					Namespace: "cluster-namespace-1",
				},
				ControllerNamespace:      namespace,
				DomainSuffix:             domainSuffix,
				GatewayDNSNamespacedName: gatewayDNSNamespacedName,
				Gateway: &corev1.Service{
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{{IP: "1.1.0.2"}},
						},
					},
				},
			},
		}

		endpointSlices = []discoveryv1.EndpointSlice{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-namespace-0-cluster-name-0-gateway",
					Namespace: namespace,
					Annotations: map[string]string{
						connectivityv1alpha1.DNSHostnameAnnotation:   "*.gateway.cluster-name-0.cluster-namesapce-0.clusters.xcc.test",
						connectivityv1alpha1.GatewayDNSRefAnnotation: "gateway-dns-namespace/gateway-dns-name",
					},
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints: []discoveryv1.Endpoint{
					{
						Addresses: []string{"1.1.0.1"},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-namespace-1-cluster-name-1-gateway",
					Namespace: namespace,
					Annotations: map[string]string{
						connectivityv1alpha1.DNSHostnameAnnotation:   "*.gateway.cluster-name-1.cluster-namespace-1.clusters.xcc.test",
						connectivityv1alpha1.GatewayDNSRefAnnotation: "gateway-dns-namespace/gateway-dns-name",
					},
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints:   []discoveryv1.Endpoint{{Addresses: []string{"1.1.0.2"}}},
			},
		}

		clusters = []clusterv1beta1.Cluster{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-name-0",
					Namespace: "cluster-namespace-0",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-name-1",
					Namespace: "cluster-namespace-1",
				},
			},
		}

		corev1Namespace := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		err := clusterClient0.Create(context.Background(), &corev1Namespace)
		Expect(err).NotTo(HaveOccurred())

		corev1Namespace = corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		err = clusterClient1.Create(context.Background(), &corev1Namespace)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when the cluster contains no previous endpoint slices", func() {
		BeforeEach(func() {
			errs := endpointSliceReconciler.ConvergeToClusters(context.Background(), clusters, gatewayDNSNamespacedName, clusterGateways)
			Expect(errs).To(BeEmpty())
		})

		It("retrieves clients from the client provider", func() {
			_, namespacedName := clientProvider.GetClientArgsForCall(0)
			Expect(namespacedName).To(Equal(types.NamespacedName{
				Namespace: "cluster-namespace-0",
				Name:      "cluster-name-0",
			}))

			_, namespacedName = clientProvider.GetClientArgsForCall(1)
			Expect(namespacedName).To(Equal(types.NamespacedName{
				Namespace: "cluster-namespace-1",
				Name:      "cluster-name-1",
			}))
		})

		It("creates the endpoint slices on each cluster client", func() {
			var endpointSliceList discoveryv1.EndpointSliceList
			Expect(clusterClient0.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(WithTransform(endpointSliceItemsToName, ConsistOf("cluster-namespace-0-cluster-name-0-gateway", "cluster-namespace-1-cluster-name-1-gateway")))

			Expect(clusterClient1.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(WithTransform(endpointSliceItemsToName, ConsistOf("cluster-namespace-0-cluster-name-0-gateway", "cluster-namespace-1-cluster-name-1-gateway")))
		})
	})

	Context("when the cluster already contains a matching endpoint slices", func() {
		BeforeEach(func() {
			existingEndpointSlices := make([]discoveryv1.EndpointSlice, 2)
			copy(existingEndpointSlices, endpointSlices)
			Expect(clusterClient0.Create(context.Background(), &existingEndpointSlices[0])).ToNot(HaveOccurred())
			Expect(clusterClient1.Create(context.Background(), &existingEndpointSlices[1])).ToNot(HaveOccurred())

			errs := endpointSliceReconciler.ConvergeToClusters(context.Background(), clusters, gatewayDNSNamespacedName, clusterGateways)
			Expect(errs).To(BeEmpty())
		})

		It("creates only the missing endpoint slices on each cluster client", func() {
			var endpointSliceList discoveryv1.EndpointSliceList
			Expect(clusterClient0.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(WithTransform(endpointSliceItemsToName, ConsistOf("cluster-namespace-0-cluster-name-0-gateway", "cluster-namespace-1-cluster-name-1-gateway")))

			Expect(clusterClient1.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(WithTransform(endpointSliceItemsToName, ConsistOf("cluster-namespace-0-cluster-name-0-gateway", "cluster-namespace-1-cluster-name-1-gateway")))
		})
	})

	Context("when the cluster contains an endpoint slice without a dns hostname annotation", func() {
		BeforeEach(func() {
			existingEndpointSlices := make([]discoveryv1.EndpointSlice, 2)
			copy(existingEndpointSlices, endpointSlices)
			delete(existingEndpointSlices[1].Annotations, connectivityv1alpha1.DNSHostnameAnnotation)
			Expect(clusterClient0.Create(context.Background(), &existingEndpointSlices[1])).ToNot(HaveOccurred())

			copy(existingEndpointSlices, endpointSlices)
			delete(existingEndpointSlices[1].Annotations, connectivityv1alpha1.DNSHostnameAnnotation)
			Expect(clusterClient1.Create(context.Background(), &existingEndpointSlices[1])).ToNot(HaveOccurred())

			onlyTheAnnotatedEndpointSlices := clusterGateways[0:1]
			errs := endpointSliceReconciler.ConvergeToClusters(context.Background(), clusters, gatewayDNSNamespacedName, onlyTheAnnotatedEndpointSlices)
			Expect(errs).To(BeEmpty())
		})

		It("doesn't delete the unannotated endpoint slice", func() {
			var endpointSliceList discoveryv1.EndpointSliceList
			Expect(clusterClient0.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(WithTransform(endpointSliceItemsToName, ConsistOf("cluster-namespace-0-cluster-name-0-gateway", "cluster-namespace-1-cluster-name-1-gateway")))

			Expect(clusterClient1.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(WithTransform(endpointSliceItemsToName, ConsistOf("cluster-namespace-0-cluster-name-0-gateway", "cluster-namespace-1-cluster-name-1-gateway")))
		})
	})

	Context("when the desired ClusterGateway indicates the cluster was not reachable", func() {
		BeforeEach(func() {
			existingEndpointSlices := make([]discoveryv1.EndpointSlice, 2)
			copy(existingEndpointSlices, endpointSlices)
			Expect(clusterClient0.Create(context.Background(), &existingEndpointSlices[0])).ToNot(HaveOccurred())
			Expect(clusterClient0.Create(context.Background(), &existingEndpointSlices[1])).ToNot(HaveOccurred())

			copy(existingEndpointSlices, endpointSlices)
			Expect(clusterClient1.Create(context.Background(), &existingEndpointSlices[0])).ToNot(HaveOccurred())
			Expect(clusterClient1.Create(context.Background(), &existingEndpointSlices[1])).ToNot(HaveOccurred())

			clusterGateways[0].Unreachable = true
			clusterGateways[0].Gateway = nil
			errs := endpointSliceReconciler.ConvergeToClusters(context.Background(), clusters, gatewayDNSNamespacedName, clusterGateways)
			Expect(errs).To(BeEmpty())
		})

		It("does not attempt to delete endpoint slices from any clusters", func() {
			var endpointSliceList discoveryv1.EndpointSliceList
			Expect(clusterClient0.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(WithTransform(endpointSliceItemsToName, ConsistOf("cluster-namespace-0-cluster-name-0-gateway", "cluster-namespace-1-cluster-name-1-gateway")))

			Expect(clusterClient1.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(WithTransform(endpointSliceItemsToName, ConsistOf("cluster-namespace-0-cluster-name-0-gateway", "cluster-namespace-1-cluster-name-1-gateway")))
		})
	})

	Context("when the cluster has an endpoint slice that is undesired", func() {
		BeforeEach(func() {
			existingEndpointSlices := make([]discoveryv1.EndpointSlice, 2)
			copy(existingEndpointSlices, endpointSlices)
			Expect(clusterClient0.Create(context.Background(), &existingEndpointSlices[0])).ToNot(HaveOccurred())
			Expect(clusterClient0.Create(context.Background(), &existingEndpointSlices[1])).ToNot(HaveOccurred())

			copy(existingEndpointSlices, endpointSlices)
			Expect(clusterClient1.Create(context.Background(), &existingEndpointSlices[0])).ToNot(HaveOccurred())
			Expect(clusterClient1.Create(context.Background(), &existingEndpointSlices[1])).ToNot(HaveOccurred())

			onlyTheFirstClusterGateway := clusterGateways[0:1]
			errs := endpointSliceReconciler.ConvergeToClusters(context.Background(), clusters, gatewayDNSNamespacedName, onlyTheFirstClusterGateway)
			Expect(errs).To(BeEmpty())
		})

		It("deletes the undesired endpoint slice", func() {
			var endpointSliceList discoveryv1.EndpointSliceList
			Expect(clusterClient0.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(WithTransform(endpointSliceItemsToName, ConsistOf("cluster-namespace-0-cluster-name-0-gateway")))

			Expect(clusterClient1.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(WithTransform(endpointSliceItemsToName, ConsistOf("cluster-namespace-0-cluster-name-0-gateway")))
		})
	})

	Context("when the cluster has an endpoint slice that has changed", func() {
		BeforeEach(func() {
			existingEndpointSlices := make([]discoveryv1.EndpointSlice, 2)

			copy(existingEndpointSlices, endpointSlices)
			existingEndpointSlices[0].Annotations[connectivityv1alpha1.DNSHostnameAnnotation] = "*.user.mangled"
			existingEndpointSlices[0].AddressType = discoveryv1.AddressTypeIPv4
			existingEndpointSlices[0].Endpoints = []discoveryv1.Endpoint{{Addresses: []string{"1.1.0.3"}}}
			existingEndpointSlices[0].Ports = []discoveryv1.EndpointPort{{Name: stringPtr("port")}}
			Expect(clusterClient0.Create(context.Background(), &existingEndpointSlices[0])).ToNot(HaveOccurred())

			copy(existingEndpointSlices, endpointSlices)
			existingEndpointSlices[0].Annotations[connectivityv1alpha1.DNSHostnameAnnotation] = "*.user.mangled"
			existingEndpointSlices[0].AddressType = discoveryv1.AddressTypeIPv4
			existingEndpointSlices[0].Endpoints = []discoveryv1.Endpoint{{Addresses: []string{"1.1.0.3"}}}
			existingEndpointSlices[0].Ports = []discoveryv1.EndpointPort{{Name: stringPtr("port")}}
			Expect(clusterClient1.Create(context.Background(), &existingEndpointSlices[0])).ToNot(HaveOccurred())

			onlyTheFirstClusterGateway := clusterGateways[0:1]
			errs := endpointSliceReconciler.ConvergeToClusters(context.Background(), clusters, gatewayDNSNamespacedName, onlyTheFirstClusterGateway)
			Expect(errs).To(BeEmpty())
		})

		It("updates the changed endpoint slice on each cluster client", func() {
			var endpointSliceList discoveryv1.EndpointSliceList
			Expect(clusterClient0.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items[0].Annotations[connectivityv1alpha1.DNSHostnameAnnotation]).To(Equal("*.gateway.cluster-name-0.cluster-namespace-0.clusters.xcc.test"))
			Expect(endpointSliceList.Items[0].AddressType).To(Equal(discoveryv1.AddressTypeIPv4))
			Expect(endpointSliceList.Items[0].Endpoints[0].Addresses[0]).To(Equal("1.1.0.1"))
			Expect(endpointSliceList.Items[0].Ports).To(BeEmpty())

			Expect(clusterClient1.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items[0].Annotations[connectivityv1alpha1.DNSHostnameAnnotation]).To(Equal("*.gateway.cluster-name-0.cluster-namespace-0.clusters.xcc.test"))
			Expect(endpointSliceList.Items[0].AddressType).To(Equal(discoveryv1.AddressTypeIPv4))
			Expect(endpointSliceList.Items[0].Endpoints[0].Addresses[0]).To(Equal("1.1.0.1"))
			Expect(endpointSliceList.Items[0].Ports).To(BeEmpty())
		})
	})

	Context("when there are endpoint slices in other namespaces", func() {
		BeforeEach(func() {
			existingEndpointSlices := make([]discoveryv1.EndpointSlice, 2)
			copy(existingEndpointSlices, endpointSlices)
			Expect(clusterClient0.Create(context.Background(), &existingEndpointSlices[0])).ToNot(HaveOccurred())
			existingEndpointSlices[1].Namespace = "not-xcc-dns"
			Expect(clusterClient0.Create(context.Background(), &existingEndpointSlices[1])).ToNot(HaveOccurred())

			copy(existingEndpointSlices, endpointSlices)
			Expect(clusterClient1.Create(context.Background(), &existingEndpointSlices[0])).ToNot(HaveOccurred())
			existingEndpointSlices[1].Namespace = "not-xcc-dns"
			Expect(clusterClient1.Create(context.Background(), &existingEndpointSlices[1])).ToNot(HaveOccurred())

			onlyTheFirstClusterGateway := clusterGateways[0:1]
			errs := endpointSliceReconciler.ConvergeToClusters(context.Background(), clusters, gatewayDNSNamespacedName, onlyTheFirstClusterGateway)
			Expect(errs).To(BeEmpty())
		})

		It("leaves them alone and does not delete them", func() {
			var endpointSliceList discoveryv1.EndpointSliceList
			Expect(clusterClient0.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(WithTransform(endpointSliceItemsToName, ConsistOf("cluster-namespace-0-cluster-name-0-gateway", "cluster-namespace-1-cluster-name-1-gateway")))

			Expect(clusterClient1.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(WithTransform(endpointSliceItemsToName, ConsistOf("cluster-namespace-0-cluster-name-0-gateway", "cluster-namespace-1-cluster-name-1-gateway")))
		})
	})

	Context("when an endpoint slice is owned by a different gateway dns", func() {
		BeforeEach(func() {
			existingEndpointSlices := make([]discoveryv1.EndpointSlice, 2)
			copy(existingEndpointSlices, endpointSlices)
			Expect(clusterClient0.Create(context.Background(), &existingEndpointSlices[0])).ToNot(HaveOccurred())
			existingEndpointSlices[1].Annotations[connectivityv1alpha1.GatewayDNSRefAnnotation] = "some-namespace/some-other-gateway-dns"
			Expect(clusterClient0.Create(context.Background(), &existingEndpointSlices[1])).ToNot(HaveOccurred())

			copy(existingEndpointSlices, endpointSlices)
			Expect(clusterClient1.Create(context.Background(), &existingEndpointSlices[0])).ToNot(HaveOccurred())
			existingEndpointSlices[1].Annotations[connectivityv1alpha1.GatewayDNSRefAnnotation] = "some-namespace/some-other-gateway-dns"
			Expect(clusterClient1.Create(context.Background(), &existingEndpointSlices[1])).ToNot(HaveOccurred())

			onlyTheFirstClusterGateway := clusterGateways[0:1]
			errs := endpointSliceReconciler.ConvergeToClusters(context.Background(), clusters, gatewayDNSNamespacedName, onlyTheFirstClusterGateway)
			Expect(errs).To(BeEmpty())
		})

		It("leaves them alone and does not delete them", func() {
			var endpointSliceList discoveryv1.EndpointSliceList
			Expect(clusterClient0.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(WithTransform(endpointSliceItemsToName, ConsistOf("cluster-namespace-0-cluster-name-0-gateway", "cluster-namespace-1-cluster-name-1-gateway")))

			Expect(clusterClient1.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(WithTransform(endpointSliceItemsToName, ConsistOf("cluster-namespace-0-cluster-name-0-gateway", "cluster-namespace-1-cluster-name-1-gateway")))
		})
	})

	Context("when an endpoint slice doesn't have the annotations, but it has a conflicting namespace/name", func() {
		BeforeEach(func() {
			endpointSlice := discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gateway-dns-namespace-cluster-name-0-gateway",
					Namespace: namespace,
				},
				AddressType: discoveryv1.AddressTypeIPv6,
				Endpoints:   []discoveryv1.Endpoint{{Addresses: []string{"2.2.0.2"}}},
				Ports: []discoveryv1.EndpointPort{
					{
						Name: stringPtr("port"),
					},
				},
			}
			Expect(clusterClient0.Create(context.Background(), &endpointSlice)).ToNot(HaveOccurred())

			errs := endpointSliceReconciler.ConvergeToClusters(context.Background(), clusters, gatewayDNSNamespacedName, clusterGateways)
			Expect(errs).To(BeEmpty())
		})

		It("updates it with the desired endpoint slice information", func() {
			var endpointSliceList discoveryv1.EndpointSliceList
			Expect(clusterClient0.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items[0].Annotations[connectivityv1alpha1.DNSHostnameAnnotation]).To(Equal("*.gateway.cluster-name-0.cluster-namespace-0.clusters.xcc.test"))
			Expect(endpointSliceList.Items[0].Annotations[connectivityv1alpha1.GatewayDNSRefAnnotation]).To(Equal("gateway-dns-namespace/gateway-dns-name"))
			Expect(endpointSliceList.Items[0].AddressType).To(Equal(discoveryv1.AddressTypeIPv4))
			Expect(endpointSliceList.Items[0].Endpoints[0].Addresses[0]).To(Equal("1.1.0.1"))
			Expect(endpointSliceList.Items[0].Ports).To(HaveLen(0))
		})
	})

	Context("when updating a cluster's endpoint slices errors", func() {
		var fakeClusterClient *gatewaydnsfakes.FakeClient
		BeforeEach(func() {
			fakeClusterClient = &gatewaydnsfakes.FakeClient{}
			fakeClusterClient.CreateReturns(errors.New("something bad happened"))

			clusterClients["cluster-namespace-0/cluster-name-0"] = fakeClusterClient
		})
		It("continues onto the next cluster", func() {
			errs := endpointSliceReconciler.ConvergeToClusters(context.Background(), clusters, gatewayDNSNamespacedName, clusterGateways)
			Expect(errs).To(ConsistOf(errors.New("something bad happened")))

			var endpointSliceList discoveryv1.EndpointSliceList
			Expect(clusterClient1.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items[0].Annotations[connectivityv1alpha1.DNSHostnameAnnotation]).To(Equal("*.gateway.cluster-name-0.cluster-namespace-0.clusters.xcc.test"))
			Expect(endpointSliceList.Items[0].Annotations[connectivityv1alpha1.GatewayDNSRefAnnotation]).To(Equal("gateway-dns-namespace/gateway-dns-name"))
			Expect(endpointSliceList.Items[0].AddressType).To(Equal(discoveryv1.AddressTypeIPv4))
			Expect(endpointSliceList.Items[0].Endpoints[0].Addresses[0]).To(Equal("1.1.0.1"))
			Expect(endpointSliceList.Items[0].Ports).To(HaveLen(0))
		})
	})

	Context("when getting a client from the provider errors", func() {
		BeforeEach(func() {
			clientProvider.GetClientStub = func(context.Context, types.NamespacedName) (client.Client, error) {
				return nil, errors.New("oopa")
			}
		})
		It("continues onto the next cluster", func() {
			errs := endpointSliceReconciler.ConvergeToClusters(context.Background(), clusters, gatewayDNSNamespacedName, clusterGateways)
			Expect(errs).To(ConsistOf(errors.New("oopa"), errors.New("oopa")))
			Expect(clientProvider.GetClientCallCount()).To(Equal(2))
		})
	})

	Context("when the namespace does not exist on the cluster", func() {
		BeforeEach(func() {
			corev1Namespace := corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			}
			err := clusterClient0.Delete(context.Background(), &corev1Namespace)
			Expect(err).NotTo(HaveOccurred())
		})

		It("skips the cluster without the namespace and without erroring, converges the other cluster", func() {
			errs := endpointSliceReconciler.ConvergeToClusters(context.Background(), clusters, gatewayDNSNamespacedName, clusterGateways)
			Expect(errs).To(HaveLen(0))

			var endpointSliceList discoveryv1.EndpointSliceList
			Expect(clusterClient0.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(HaveLen(0))

			Expect(clusterClient1.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(WithTransform(endpointSliceItemsToName, ConsistOf("cluster-namespace-0-cluster-name-0-gateway", "cluster-namespace-1-cluster-name-1-gateway")))
		})
	})
})

func endpointSliceItemsToName(items []discoveryv1.EndpointSlice) []string {
	var names []string
	for _, item := range items {
		names = append(names, item.Name)
	}

	return names
}

func stringPtr(value string) *string {
	return &value
}
