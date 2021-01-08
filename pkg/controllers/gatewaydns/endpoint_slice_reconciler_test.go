// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package gatewaydns_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/gatewaydns"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/gatewaydns/gatewaydnsfakes"
	discoveryv1beta1 "k8s.io/api/discovery/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Endpoint Slice Reconciler", func() {
	var (
		endpointSliceReconciler  gatewaydns.EndpointSliceReconciler
		clientProvider           *gatewaydnsfakes.FakeClientProvider
		clusterClient0           client.Client
		clusterClient1           client.Client
		gatewayDNSNamespacedName types.NamespacedName
		endpointSlices           []discoveryv1beta1.EndpointSlice
		clusters                 []clusterv1alpha3.Cluster
		namespace                string
	)

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		_ = clientgoscheme.AddToScheme(scheme)
		_ = connectivityv1alpha1.AddToScheme(scheme)
		_ = clusterv1alpha3.AddToScheme(scheme)
		_ = discoveryv1beta1.AddToScheme(scheme)

		clusterClient0 = fake.NewClientBuilder().WithScheme(scheme).Build()
		clusterClient1 = fake.NewClientBuilder().WithScheme(scheme).Build()

		clusterClients := make(map[string]client.Client)
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
		namespace = "capi-dns"

		endpointSliceReconciler = gatewaydns.EndpointSliceReconciler{
			ClientProvider: clientProvider,
			Namespace:      namespace,
		}

		gatewayDNSNamespacedName = types.NamespacedName{
			Namespace: "some-namespace",
			Name:      "some-gateway-dns",
		}

		endpointSlices = []discoveryv1beta1.EndpointSlice{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "endpoint-slice-1",
					Namespace: namespace,
					Annotations: map[string]string{
						connectivityv1alpha1.DNSHostnameAnnotation:   "*.gateway.cluster-name-1.gateway-dns-namespace.xcc.test",
						connectivityv1alpha1.GatewayDNSRefAnnotation: "some-namespace/some-gateway-dns",
					},
				},
				AddressType: discoveryv1beta1.AddressTypeIPv4,
				Endpoints: []discoveryv1beta1.Endpoint{
					{
						Addresses: []string{"1.1.0.1"},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "endpoint-slice-2",
					Namespace: namespace,
					Annotations: map[string]string{
						connectivityv1alpha1.DNSHostnameAnnotation:   "*.gateway.cluster-name-2.gateway-dns-namespace.xcc.test",
						connectivityv1alpha1.GatewayDNSRefAnnotation: "some-namespace/some-gateway-dns",
					},
				},
				AddressType: discoveryv1beta1.AddressTypeIPv4,
				Endpoints: []discoveryv1beta1.Endpoint{
					{
						Addresses: []string{"1.1.0.2"},
					},
				},
			},
		}

		clusters = []clusterv1alpha3.Cluster{
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

	})

	Context("when the cluster contains no previous endpoint slices", func() {
		BeforeEach(func() {
			err := endpointSliceReconciler.ConvergeEndpointSlicesToClusters(context.Background(), clusters, gatewayDNSNamespacedName, endpointSlices)
			Expect(err).ToNot(HaveOccurred())
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
			var endpointSliceList discoveryv1beta1.EndpointSliceList
			Expect(clusterClient0.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(WithTransform(endpointSliceItemsToName, ConsistOf("endpoint-slice-1", "endpoint-slice-2")))

			Expect(clusterClient1.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(WithTransform(endpointSliceItemsToName, ConsistOf("endpoint-slice-1", "endpoint-slice-2")))
		})
	})

	Context("when the cluster already contains a matching endpoint slices", func() {
		BeforeEach(func() {
			existingEndpointSlices := make([]discoveryv1beta1.EndpointSlice, 2)
			copy(existingEndpointSlices, endpointSlices)
			Expect(clusterClient0.Create(context.Background(), &existingEndpointSlices[0])).ToNot(HaveOccurred())
			Expect(clusterClient1.Create(context.Background(), &existingEndpointSlices[1])).ToNot(HaveOccurred())

			err := endpointSliceReconciler.ConvergeEndpointSlicesToClusters(context.Background(), clusters, gatewayDNSNamespacedName, endpointSlices)
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates only the missting endpoint slices on each cluster client", func() {
			var endpointSliceList discoveryv1beta1.EndpointSliceList
			Expect(clusterClient0.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(WithTransform(endpointSliceItemsToName, ConsistOf("endpoint-slice-1", "endpoint-slice-2")))

			Expect(clusterClient1.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(WithTransform(endpointSliceItemsToName, ConsistOf("endpoint-slice-1", "endpoint-slice-2")))
		})
	})

	Context("when the cluster contains an endpoint slice without a dns hostname annotation", func() {
		BeforeEach(func() {
			existingEndpointSlices := make([]discoveryv1beta1.EndpointSlice, 2)
			copy(existingEndpointSlices, endpointSlices)
			delete(existingEndpointSlices[1].Annotations, connectivityv1alpha1.DNSHostnameAnnotation)
			Expect(clusterClient0.Create(context.Background(), &existingEndpointSlices[1])).ToNot(HaveOccurred())

			copy(existingEndpointSlices, endpointSlices)
			delete(existingEndpointSlices[1].Annotations, connectivityv1alpha1.DNSHostnameAnnotation)
			Expect(clusterClient1.Create(context.Background(), &existingEndpointSlices[1])).ToNot(HaveOccurred())

			onlyTheAnnotatedEndpointSlices := endpointSlices[0:1]
			err := endpointSliceReconciler.ConvergeEndpointSlicesToClusters(context.Background(), clusters, gatewayDNSNamespacedName, onlyTheAnnotatedEndpointSlices)
			Expect(err).NotTo(HaveOccurred())
		})

		It("doesn't delete the unannotated endpoint slice", func() {
			var endpointSliceList discoveryv1beta1.EndpointSliceList
			Expect(clusterClient0.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(WithTransform(endpointSliceItemsToName, ConsistOf("endpoint-slice-1", "endpoint-slice-2")))

			Expect(clusterClient1.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(WithTransform(endpointSliceItemsToName, ConsistOf("endpoint-slice-1", "endpoint-slice-2")))
		})
	})

	Context("when the cluster has an endpoint slice that is undesired", func() {
		BeforeEach(func() {
			existingEndpointSlices := make([]discoveryv1beta1.EndpointSlice, 2)
			copy(existingEndpointSlices, endpointSlices)
			Expect(clusterClient0.Create(context.Background(), &existingEndpointSlices[0])).ToNot(HaveOccurred())
			Expect(clusterClient0.Create(context.Background(), &existingEndpointSlices[1])).ToNot(HaveOccurred())

			copy(existingEndpointSlices, endpointSlices)
			Expect(clusterClient1.Create(context.Background(), &existingEndpointSlices[0])).ToNot(HaveOccurred())
			Expect(clusterClient1.Create(context.Background(), &existingEndpointSlices[1])).ToNot(HaveOccurred())

			onlyTheFirstEndpointSlice := endpointSlices[0:1]
			err := endpointSliceReconciler.ConvergeEndpointSlicesToClusters(context.Background(), clusters, gatewayDNSNamespacedName, onlyTheFirstEndpointSlice)
			Expect(err).NotTo(HaveOccurred())
		})

		It("deletes the undesired endpoint slice", func() {
			var endpointSliceList discoveryv1beta1.EndpointSliceList
			Expect(clusterClient0.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(WithTransform(endpointSliceItemsToName, ConsistOf("endpoint-slice-1")))

			Expect(clusterClient1.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(WithTransform(endpointSliceItemsToName, ConsistOf("endpoint-slice-1")))
		})
	})

	Context("when the cluster has an endpoint slice that has changed", func() {
		BeforeEach(func() {
			existingEndpointSlices := make([]discoveryv1beta1.EndpointSlice, 2)
			copy(existingEndpointSlices, endpointSlices)
			Expect(clusterClient0.Create(context.Background(), &existingEndpointSlices[0])).ToNot(HaveOccurred())

			copy(existingEndpointSlices, endpointSlices)
			Expect(clusterClient1.Create(context.Background(), &existingEndpointSlices[0])).ToNot(HaveOccurred())

			endpointSlices[0].Annotations[connectivityv1alpha1.DNSHostnameAnnotation] = "*.changed"
			endpointSlices[0].AddressType = discoveryv1beta1.AddressTypeIP
			endpointSlices[0].Endpoints = []discoveryv1beta1.Endpoint{
				{
					Addresses: []string{"1.1.0.3"},
				},
			}
			endpointSlices[0].Ports = []discoveryv1beta1.EndpointPort{
				{
					Name: stringPtr("port"),
				},
			}
			onlyTheFirstEndpointSlice := endpointSlices[0:1]
			err := endpointSliceReconciler.ConvergeEndpointSlicesToClusters(context.Background(), clusters, gatewayDNSNamespacedName, onlyTheFirstEndpointSlice)
			Expect(err).NotTo(HaveOccurred())
		})

		It("updates the changed endpoint slice on each cluster client", func() {
			var endpointSliceList discoveryv1beta1.EndpointSliceList
			Expect(clusterClient0.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items[0].Annotations[connectivityv1alpha1.DNSHostnameAnnotation]).To(Equal("*.changed"))
			Expect(endpointSliceList.Items[0].AddressType).To(Equal(discoveryv1beta1.AddressTypeIP))
			Expect(endpointSliceList.Items[0].Endpoints[0].Addresses[0]).To(Equal("1.1.0.3"))
			Expect(*endpointSliceList.Items[0].Ports[0].Name).To(Equal("port"))

			Expect(clusterClient1.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items[0].Annotations[connectivityv1alpha1.DNSHostnameAnnotation]).To(Equal("*.changed"))
			Expect(endpointSliceList.Items[0].AddressType).To(Equal(discoveryv1beta1.AddressTypeIP))
			Expect(endpointSliceList.Items[0].Endpoints[0].Addresses[0]).To(Equal("1.1.0.3"))
			Expect(*endpointSliceList.Items[0].Ports[0].Name).To(Equal("port"))
		})
	})

	Context("when there are endpoint slices in other namespaces", func() {
		BeforeEach(func() {
			existingEndpointSlices := make([]discoveryv1beta1.EndpointSlice, 2)
			copy(existingEndpointSlices, endpointSlices)
			Expect(clusterClient0.Create(context.Background(), &existingEndpointSlices[0])).ToNot(HaveOccurred())
			existingEndpointSlices[1].Namespace = "not-capi-dns"
			Expect(clusterClient0.Create(context.Background(), &existingEndpointSlices[1])).ToNot(HaveOccurred())

			copy(existingEndpointSlices, endpointSlices)
			Expect(clusterClient1.Create(context.Background(), &existingEndpointSlices[0])).ToNot(HaveOccurred())
			existingEndpointSlices[1].Namespace = "not-capi-dns"
			Expect(clusterClient1.Create(context.Background(), &existingEndpointSlices[1])).ToNot(HaveOccurred())

			onlyTheFirstEndpointSlice := endpointSlices[0:1]
			err := endpointSliceReconciler.ConvergeEndpointSlicesToClusters(context.Background(), clusters, gatewayDNSNamespacedName, onlyTheFirstEndpointSlice)
			Expect(err).NotTo(HaveOccurred())
		})

		It("leaves them alone and does not delete them", func() {
			var endpointSliceList discoveryv1beta1.EndpointSliceList
			Expect(clusterClient0.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(WithTransform(endpointSliceItemsToName, ConsistOf("endpoint-slice-1", "endpoint-slice-2")))

			Expect(clusterClient1.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(WithTransform(endpointSliceItemsToName, ConsistOf("endpoint-slice-1", "endpoint-slice-2")))
		})
	})

	Context("when an endpoint slice is owned by a different gateway dns", func() {
		BeforeEach(func() {
			existingEndpointSlices := make([]discoveryv1beta1.EndpointSlice, 2)
			copy(existingEndpointSlices, endpointSlices)
			Expect(clusterClient0.Create(context.Background(), &existingEndpointSlices[0])).ToNot(HaveOccurred())
			existingEndpointSlices[1].Annotations[connectivityv1alpha1.GatewayDNSRefAnnotation] = "some-namespace/some-other-gateway-dns"
			Expect(clusterClient0.Create(context.Background(), &existingEndpointSlices[1])).ToNot(HaveOccurred())

			copy(existingEndpointSlices, endpointSlices)
			Expect(clusterClient1.Create(context.Background(), &existingEndpointSlices[0])).ToNot(HaveOccurred())
			existingEndpointSlices[1].Annotations[connectivityv1alpha1.GatewayDNSRefAnnotation] = "some-namespace/some-other-gateway-dns"
			Expect(clusterClient1.Create(context.Background(), &existingEndpointSlices[1])).ToNot(HaveOccurred())

			onlyTheFirstEndpointSlice := endpointSlices[0:1]
			err := endpointSliceReconciler.ConvergeEndpointSlicesToClusters(context.Background(), clusters, gatewayDNSNamespacedName, onlyTheFirstEndpointSlice)
			Expect(err).NotTo(HaveOccurred())
		})

		It("leaves them alone and does not delete them", func() {
			var endpointSliceList discoveryv1beta1.EndpointSliceList
			Expect(clusterClient0.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(WithTransform(endpointSliceItemsToName, ConsistOf("endpoint-slice-1", "endpoint-slice-2")))

			Expect(clusterClient1.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items).To(WithTransform(endpointSliceItemsToName, ConsistOf("endpoint-slice-1", "endpoint-slice-2")))
		})
	})

	Context("when an endpoint slice doesn't have the annotations, but it has a conflicting namespace/name", func() {
		BeforeEach(func() {
			endpointSlice := discoveryv1beta1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "endpoint-slice-1",
					Namespace: namespace,
				},
				AddressType: discoveryv1beta1.AddressTypeIPv6,
				Endpoints: []discoveryv1beta1.Endpoint{
					{
						Addresses: []string{"2.2.0.2"},
					},
				},
				Ports: []discoveryv1beta1.EndpointPort{
					{
						Name: stringPtr("port"),
					},
				},
			}
			Expect(clusterClient0.Create(context.Background(), &endpointSlice)).ToNot(HaveOccurred())

			err := endpointSliceReconciler.ConvergeEndpointSlicesToClusters(context.Background(), clusters, gatewayDNSNamespacedName, endpointSlices)
			Expect(err).NotTo(HaveOccurred())
		})

		It("updates it with the desired endpoint slice information", func() {
			var endpointSliceList discoveryv1beta1.EndpointSliceList
			Expect(clusterClient0.List(context.Background(), &endpointSliceList)).NotTo(HaveOccurred())
			Expect(endpointSliceList.Items[0].Annotations[connectivityv1alpha1.DNSHostnameAnnotation]).To(Equal("*.gateway.cluster-name-1.gateway-dns-namespace.xcc.test"))
			Expect(endpointSliceList.Items[0].Annotations[connectivityv1alpha1.GatewayDNSRefAnnotation]).To(Equal("some-namespace/some-gateway-dns"))
			Expect(endpointSliceList.Items[0].AddressType).To(Equal(discoveryv1beta1.AddressTypeIPv4))
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
		It("returns the error", func() {
			err := endpointSliceReconciler.ConvergeEndpointSlicesToClusters(context.Background(), clusters, gatewayDNSNamespacedName, endpointSlices)
			Expect(err).To(MatchError("oopa"))
		})
	})
})

func endpointSliceItemsToName(items []discoveryv1beta1.EndpointSlice) []string {
	var names []string
	for _, item := range items {
		names = append(names, item.Name)
	}

	return names
}

func stringPtr(value string) *string {
	return &value
}
