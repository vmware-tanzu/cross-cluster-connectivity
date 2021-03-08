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

var _ = Describe("Cluster Gateway Collector", func() {
	var (
		clusterClient0 client.Client
		clusterClient1 client.Client
		clusterClients map[string]client.Client
		clientProvider *gatewaydnsfakes.FakeClientProvider

		gatewayDNS *connectivityv1alpha1.GatewayDNS
		cluster0   clusterv1alpha3.Cluster
		cluster1   clusterv1alpha3.Cluster

		clusters        []clusterv1alpha3.Cluster
		gatewayService0 *corev1.Service
		gatewayService1 *corev1.Service

		clusterGatewayCollector *gatewaydns.ClusterGatewayCollector
	)

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		_ = clientgoscheme.AddToScheme(scheme)
		_ = connectivityv1alpha1.AddToScheme(scheme)
		_ = clusterv1alpha3.AddToScheme(scheme)
		_ = discoveryv1beta1.AddToScheme(scheme)

		clusterClient0 = fake.NewClientBuilder().WithScheme(scheme).Build()
		clusterClient1 = fake.NewClientBuilder().WithScheme(scheme).Build()

		clusterClients = make(map[string]client.Client)
		clusterClients["some-namespace/cluster-name-0"] = clusterClient0
		clusterClients["some-namespace/cluster-name-1"] = clusterClient1

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
		cluster0 = clusterv1alpha3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster-name-0",
				Namespace: "some-namespace",
				Labels: map[string]string{
					"some-label": "true",
				},
			},
		}
		cluster1 = clusterv1alpha3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster-name-1",
				Namespace: "some-namespace",
				Labels: map[string]string{
					"some-label": "true",
				},
			},
		}
		clusters = []clusterv1alpha3.Cluster{cluster0, cluster1}

		gatewayService0 = &corev1.Service{
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

		gatewayService1 = &corev1.Service{
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
	})

	Describe("GetGatewaysForCluster", func() {
		Context("when the cluster has a valid gateway", func() {
			BeforeEach(func() {
				err := clusterClient0.Create(context.Background(), gatewayService0)
				Expect(err).NotTo(HaveOccurred())

				err = clusterClient1.Create(context.Background(), gatewayService1)
				Expect(err).NotTo(HaveOccurred())
			})
			It("the gateway for the cluster is returned", func() {
				gateways := clusterGatewayCollector.GetGatewaysForClusters(
					context.Background(),
					*gatewayDNS,
					clusters,
				)
				Expect(gateways).To(HaveLen(2))
				Expect(gateways[0].ClusterNamespacedName.Name).To(Equal(clusters[0].Name))
				Expect(gateways[0].ClusterNamespacedName.Namespace).To(Equal(clusters[0].Namespace))
				Expect(gateways[0].Gateway.Spec.Type).To(Equal(corev1.ServiceTypeLoadBalancer))
				Expect(gateways[0].Gateway.Status.LoadBalancer.Ingress[0].IP).To(Equal("1.2.3.4"))

				Expect(gateways[1].ClusterNamespacedName.Name).To(Equal(clusters[1].Name))
				Expect(gateways[1].ClusterNamespacedName.Namespace).To(Equal(clusters[1].Namespace))
				Expect(gateways[1].Gateway.Spec.Type).To(Equal(corev1.ServiceTypeLoadBalancer))
				Expect(gateways[1].Gateway.Status.LoadBalancer.Ingress[0].IP).To(Equal("1.2.3.5"))
			})
		})

		Context("when searching a client for services fails", func() {
			var fakeClusterClient *gatewaydnsfakes.FakeClient
			BeforeEach(func() {
				fakeClusterClient = &gatewaydnsfakes.FakeClient{}
				fakeClusterClient.GetReturns(errors.New("something bad happened"))
				clusterClients["some-namespace/cluster-name-0"] = fakeClusterClient

				err := clusterClient1.Create(context.Background(), gatewayService1)
				Expect(err).NotTo(HaveOccurred())
			})
			It("returns a GatewayCluster marked Unreachable", func() {
				gateways := clusterGatewayCollector.GetGatewaysForClusters(
					context.Background(),
					*gatewayDNS,
					clusters,
				)
				Expect(gateways).To(HaveLen(2))
				Expect(gateways[0].ClusterNamespacedName.Name).To(Equal(clusters[0].Name))
				Expect(gateways[0].ClusterNamespacedName.Namespace).To(Equal(clusters[0].Namespace))
				Expect(gateways[0].Unreachable).To(BeTrue())

				Expect(gateways[1].ClusterNamespacedName.Name).To(Equal(clusters[1].Name))
				Expect(gateways[1].ClusterNamespacedName.Namespace).To(Equal(clusters[1].Namespace))
				Expect(gateways[1].Gateway.Spec.Type).To(Equal(corev1.ServiceTypeLoadBalancer))
				Expect(gateways[1].Gateway.Status.LoadBalancer.Ingress[0].IP).To(Equal("1.2.3.5"))
			})
		})

		Context("when getting a cluster client fails", func() {
			BeforeEach(func() {
				clientProvider.GetClientStub = func(ctx context.Context, namespacedName types.NamespacedName) (client.Client, error) {
					return nil, errors.New("error getting client")
				}
			})
			It("returns a GatewayCluster marked Unreachable", func() {
				gateways := clusterGatewayCollector.GetGatewaysForClusters(
					context.Background(),
					*gatewayDNS,
					clusters,
				)
				Expect(gateways).To(HaveLen(2))
				Expect(gateways[0].ClusterNamespacedName.Name).To(Equal(clusters[0].Name))
				Expect(gateways[0].Unreachable).To(BeTrue())

				Expect(gateways[1].ClusterNamespacedName.Name).To(Equal(clusters[1].Name))
				Expect(gateways[1].Unreachable).To(BeTrue())
			})
		})

		Context("when the gateway service name does not match the spec", func() {
			BeforeEach(func() {
				gatewayService0.ObjectMeta.Name = "some-other-name"
				err := clusterClient0.Create(context.Background(), gatewayService0)
				Expect(err).NotTo(HaveOccurred())
			})
			It("does not get returned", func() {
				gateways := clusterGatewayCollector.GetGatewaysForClusters(
					context.Background(),
					*gatewayDNS,
					clusters,
				)
				Expect(gateways).To(HaveLen(0))
			})
		})

		Context("when the gateway service namespance does not match the spec", func() {
			BeforeEach(func() {
				gatewayService0.ObjectMeta.Namespace = "some-other-namespace"
				err := clusterClient0.Create(context.Background(), gatewayService0)
				Expect(err).NotTo(HaveOccurred())
			})
			It("does not get returned", func() {
				gateways := clusterGatewayCollector.GetGatewaysForClusters(
					context.Background(),
					*gatewayDNS,
					clusters,
				)
				Expect(gateways).To(HaveLen(0))
			})
		})

		Context("when the gateway service is not of type load balancer", func() {
			BeforeEach(func() {
				gatewayService0.Spec.Type = corev1.ServiceTypeClusterIP
				err := clusterClient0.Create(context.Background(), gatewayService0)
				Expect(err).NotTo(HaveOccurred())
			})
			It("does not get returned", func() {
				gateways := clusterGatewayCollector.GetGatewaysForClusters(
					context.Background(),
					*gatewayDNS,
					clusters,
				)
				Expect(gateways).To(HaveLen(0))
			})
		})

		Context("when the gateway service status has no IP addresses assigned", func() {
			BeforeEach(func() {
				gatewayService0.Status = corev1.ServiceStatus{}
				err := clusterClient0.Create(context.Background(), gatewayService0)
				Expect(err).NotTo(HaveOccurred())
			})
			It("does not get returned", func() {
				gateways := clusterGatewayCollector.GetGatewaysForClusters(
					context.Background(),
					*gatewayDNS,
					clusters,
				)
				Expect(gateways).To(HaveLen(0))
			})
		})
	})
})
