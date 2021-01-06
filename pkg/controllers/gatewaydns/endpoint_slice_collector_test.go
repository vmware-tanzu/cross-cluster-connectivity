// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package gatewaydns_test

import (
	"context"

	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/gatewaydns"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/gatewaydns/gatewaydnsfakes"
	discoveryv1beta1 "k8s.io/api/discovery/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EndpointSliceCollector", func() {
	var (
		clusterClient  client.Client
		clientProvider *gatewaydnsfakes.FakeClientProvider

		gatewayDNS *connectivityv1alpha1.GatewayDNS
		cluster    *clusterv1alpha3.Cluster

		endpointSliceCollector *gatewaydns.EndpointSliceCollector
	)

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		_ = clientgoscheme.AddToScheme(scheme)
		_ = connectivityv1alpha1.AddToScheme(scheme)
		_ = clusterv1alpha3.AddToScheme(scheme)
		_ = discoveryv1beta1.AddToScheme(scheme)

		clusterClient = fake.NewClientBuilder().WithScheme(scheme).Build()

		clientProvider = &gatewaydnsfakes.FakeClientProvider{}
		clientProvider.GetClientReturns(clusterClient, nil)

		ctrl.SetLogger(zap.New(
			zap.UseDevMode(true),
			zap.WriteTo(GinkgoWriter),
		))

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

		log := ctrl.Log.WithName("EndpointSliceCollector")
		endpointSliceCollector = &gatewaydns.EndpointSliceCollector{
			Log:            log,
			ClientProvider: clientProvider,
		}

		cluster = &clusterv1alpha3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-cluster",
				Namespace: "some-namespace",
				Labels: map[string]string{
					"some-label": "true",
				},
			},
		}
	})

	Describe("createEndpointSlicesForCluster", func() {
		It("queries the clusters for their load balanced services and returns corresponding EndpointSlices", func() {
			clusters := []clusterv1alpha3.Cluster{*cluster}
			createdSlices, err := endpointSliceCollector.CreateEndpointSlicesForClusters(
				context.Background(),
				*gatewayDNS,
				clusters,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdSlices).To(HaveLen(1))
		})
	})
})
