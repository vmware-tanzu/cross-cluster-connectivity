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
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Cluster Gateway Collector", func() {
	var (
		clusterClient  client.Client
		clientProvider *gatewaydnsfakes.FakeClientProvider

		gatewayDNS     *connectivityv1alpha1.GatewayDNS
		cluster        *clusterv1alpha3.Cluster
		clusters       []clusterv1alpha3.Cluster
		gatewayService *corev1.Service

		clusterGatewayCollector *gatewaydns.ClusterGatewayCollector
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

		log := ctrl.Log.WithName("ClusterGatewayCollector")
		clusterGatewayCollector = &gatewaydns.ClusterGatewayCollector{
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
		clusters = []clusterv1alpha3.Cluster{*cluster}

		gatewayService = &corev1.Service{
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
	})

	Describe("GetGatewaysForCluster", func() {
		Context("when the cluster has a valid gateway", func() {
			BeforeEach(func() {
				err := clusterClient.Create(context.Background(), gatewayService)
				Expect(err).NotTo(HaveOccurred())
			})
			It("the gateway for the cluster is returned", func() {
				gateways, err := clusterGatewayCollector.GetGatewaysForClusters(
					context.Background(),
					*gatewayDNS,
					clusters,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(gateways).To(HaveLen(1))
				Expect(gateways[0].ClusterName).To(Equal(cluster.ObjectMeta.Name))
				Expect(gateways[0].Gateway.Spec.Type).To(Equal(corev1.ServiceTypeLoadBalancer))
				Expect(gateways[0].Gateway.Status.LoadBalancer.Ingress[0].IP).To(Equal("1.2.3.4"))
			})
		})

		Context("when the gateway service name does not match the spec", func() {
			BeforeEach(func() {
				gatewayService.ObjectMeta.Name = "some-other-name"
				err := clusterClient.Create(context.Background(), gatewayService)
				Expect(err).NotTo(HaveOccurred())
			})
			It("does not get returned", func() {
				gateways, err := clusterGatewayCollector.GetGatewaysForClusters(
					context.Background(),
					*gatewayDNS,
					clusters,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(gateways).To(HaveLen(0))
			})
		})

		Context("when the gateway service namespance does not match the spec", func() {
			BeforeEach(func() {
				gatewayService.ObjectMeta.Namespace = "some-other-namespace"
				err := clusterClient.Create(context.Background(), gatewayService)
				Expect(err).NotTo(HaveOccurred())
			})
			It("does not get returned", func() {
				gateways, err := clusterGatewayCollector.GetGatewaysForClusters(
					context.Background(),
					*gatewayDNS,
					clusters,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(gateways).To(HaveLen(0))
			})
		})

		Context("when the gateway service is not of type load balancer", func() {
			BeforeEach(func() {
				gatewayService.Spec.Type = corev1.ServiceTypeClusterIP
				err := clusterClient.Create(context.Background(), gatewayService)
				Expect(err).NotTo(HaveOccurred())
			})
			It("does not get returned", func() {
				gateways, err := clusterGatewayCollector.GetGatewaysForClusters(
					context.Background(),
					*gatewayDNS,
					clusters,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(gateways).To(HaveLen(0))
			})
		})

		Context("when the gateway service status has no IP addresses assigned", func() {
			BeforeEach(func() {
				gatewayService.Status = corev1.ServiceStatus{}
				err := clusterClient.Create(context.Background(), gatewayService)
				Expect(err).NotTo(HaveOccurred())
			})
			It("does not get returned", func() {
				gateways, err := clusterGatewayCollector.GetGatewaysForClusters(
					context.Background(),
					*gatewayDNS,
					clusters,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(gateways).To(HaveLen(0))
			})
		})

	})
})
