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
		endpointSliceReconciler gatewaydns.EndpointSliceReconciler
		clientProvider          *gatewaydnsfakes.FakeClientProvider
		clusterClient0          client.Client
		clusterClient1          client.Client
		endpointSlices          []discoveryv1beta1.EndpointSlice
		clusters                []clusterv1alpha3.Cluster
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

		endpointSliceReconciler = gatewaydns.EndpointSliceReconciler{ClientProvider: clientProvider}

		endpointSlices = []discoveryv1beta1.EndpointSlice{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "endpoint-slice-1",
					Namespace: "capi-dns",
					Annotations: map[string]string{
						connectivityv1alpha1.DNSHostnameAnnotation: "*.gateway.cluster-name-1.gateway-dns-namespace.xcc.test",
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
					Namespace: "capi-dns",
					Annotations: map[string]string{
						connectivityv1alpha1.DNSHostnameAnnotation: "*.gateway.cluster-name-2.gateway-dns-namespace.xcc.test",
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

	Context("when writing succeeds", func() {
		JustBeforeEach(func() {
			err := endpointSliceReconciler.WriteEndpointSlicesToClusters(context.Background(), clusters, endpointSlices)
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
			err := clusterClient0.List(context.Background(), &endpointSliceList)
			Expect(err).NotTo(HaveOccurred())
			actualEndpointSlices := endpointSliceList.Items
			Expect(actualEndpointSlices[0].Name).To(Equal("endpoint-slice-1"))
			Expect(actualEndpointSlices[1].Name).To(Equal("endpoint-slice-2"))

			err = clusterClient1.List(context.Background(), &endpointSliceList)
			Expect(err).NotTo(HaveOccurred())
			actualEndpointSlices = endpointSliceList.Items
			Expect(actualEndpointSlices[0].Name).To(Equal("endpoint-slice-1"))
			Expect(actualEndpointSlices[1].Name).To(Equal("endpoint-slice-2"))
		})
	})

	Context("when getting a client from the provider errors", func() {
		BeforeEach(func() {
			clientProvider.GetClientStub = func(context.Context, types.NamespacedName) (client.Client, error) {
				return nil, errors.New("oopa")
			}
		})
		It("returns the error", func() {
			err := endpointSliceReconciler.WriteEndpointSlicesToClusters(context.Background(), clusters, endpointSlices)
			Expect(err).To(MatchError("oopa"))
		})
	})
})
