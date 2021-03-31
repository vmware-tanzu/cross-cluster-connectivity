// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package endpointslicedns_test

import (
	"context"

	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/endpointslicedns"
	discoveryv1beta1 "k8s.io/api/discovery/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Reconcile", func() {
	var (
		kubeClient              client.Client
		dnsCache                *endpointslicedns.DNSCache
		endpointSliceReconciler *endpointslicedns.EndpointSliceReconciler

		endpointSlice *discoveryv1beta1.EndpointSlice

		req         ctrl.Request
		expectedIPs []string
	)

	BeforeEach(func() {
		dnsCache = new(endpointslicedns.DNSCache)

		scheme := runtime.NewScheme()
		_ = clientgoscheme.AddToScheme(scheme)
		_ = discoveryv1beta1.AddToScheme(scheme)

		kubeClient = fake.NewClientBuilder().WithScheme(scheme).Build()

		ctrl.SetLogger(zap.New(
			zap.UseDevMode(true),
			zap.WriteTo(GinkgoWriter),
		))

		endpointSliceReconciler = &endpointslicedns.EndpointSliceReconciler{
			Client:       kubeClient,
			Log:          ctrl.Log.WithName("controllers").WithName("EndpointSlice"),
			Scheme:       kubeClient.Scheme(),
			RecordsCache: dnsCache,
		}

		endpointSlice = &discoveryv1beta1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "some-endpoint-slice",
				Namespace:   "cross-cluster-connectivity",
				Annotations: map[string]string{},
			},
			AddressType: discoveryv1beta1.AddressTypeIPv4,
			Endpoints: []discoveryv1beta1.Endpoint{
				{
					Addresses: []string{"1.2.3.4", "1.2.3.5"},
				},
				{
					Addresses: []string{"1.2.3.6", "1.2.3.7"},
				},
			},
		}
		expectedIPs = []string{"1.2.3.4", "1.2.3.5", "1.2.3.6", "1.2.3.7"}

		err := kubeClient.Create(context.Background(), endpointSlice)
		Expect(err).NotTo(HaveOccurred())

		req.Name = endpointSlice.Name
		req.Namespace = endpointSlice.Namespace
	})

	When("an EndpointSlice exists with a DNS hostname annotation", func() {
		BeforeEach(func() {
			endpointSlice.Annotations[connectivityv1alpha1.DNSHostnameAnnotation] = "foo.xcc.test"
			err := kubeClient.Update(context.Background(), endpointSlice)
			Expect(err).NotTo(HaveOccurred())
		})

		It("populates the dns cache with the domain name and endpoints from the EndpointSlice", func() {
			result, err := endpointSliceReconciler.Reconcile(context.Background(), req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeFalse())

			cacheEntries := dnsCache.Lookup("foo.xcc.test")
			Expect(cacheEntries).NotTo(BeEmpty())
			Expect(cacheEntriesToIPStrings(cacheEntries)).To(ConsistOf(expectedIPs))
		})

		When("the domain name is a wildcard domain", func() {
			BeforeEach(func() {
				endpointSlice.Annotations[connectivityv1alpha1.DNSHostnameAnnotation] = "*.gateway.xcc.test"
				err := kubeClient.Update(context.Background(), endpointSlice)
				Expect(err).NotTo(HaveOccurred())
			})

			It("can lookup any domain on the wildcard domain", func() {
				_, err := endpointSliceReconciler.Reconcile(context.Background(), req)
				Expect(err).NotTo(HaveOccurred())

				cacheEntries := dnsCache.Lookup("foo.gateway.xcc.test")
				Expect(cacheEntries).NotTo(BeEmpty())
				Expect(cacheEntriesToIPStrings(cacheEntries)).To(ConsistOf(expectedIPs))

				cacheEntries = dnsCache.Lookup("bar.gateway.xcc.test")
				Expect(cacheEntries).NotTo(BeEmpty())
				Expect(cacheEntriesToIPStrings(cacheEntries)).To(ConsistOf(expectedIPs))
			})
		})

		When("multiple EndpointSlices exists with the same DNS hostname annotation", func() {
			BeforeEach(func() {
				endpointSlice.Annotations[connectivityv1alpha1.DNSHostnameAnnotation] = "foo.xcc.test"
				err := kubeClient.Update(context.Background(), endpointSlice)
				Expect(err).NotTo(HaveOccurred())

				anotherEndpointSlice := &discoveryv1beta1.EndpointSlice{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "another-endpoint-slice",
						Namespace: "cross-cluster-connectivity",
						Annotations: map[string]string{
							connectivityv1alpha1.DNSHostnameAnnotation: "foo.xcc.test",
						},
					},
					AddressType: discoveryv1beta1.AddressTypeIPv4,
					Endpoints: []discoveryv1beta1.Endpoint{
						{
							Addresses: []string{"2.2.3.4", "2.2.3.5"},
						},
						{
							Addresses: []string{"2.2.3.6", "2.2.3.7"},
						},
					},
				}

				expectedIPs = append(expectedIPs, "2.2.3.4", "2.2.3.5", "2.2.3.6", "2.2.3.7")

				err = kubeClient.Create(context.Background(), anotherEndpointSlice)
				Expect(err).NotTo(HaveOccurred())

				differentEndpointSlice := &discoveryv1beta1.EndpointSlice{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "different-endpoint-slice",
						Namespace: "cross-cluster-connectivity",
						Annotations: map[string]string{
							connectivityv1alpha1.DNSHostnameAnnotation: "bar.xcc.test",
						},
					},
					AddressType: discoveryv1beta1.AddressTypeIPv4,
					Endpoints: []discoveryv1beta1.Endpoint{
						{
							Addresses: []string{"3.2.3.4", "3.2.3.5"},
						},
						{
							Addresses: []string{"3.2.3.6", "3.2.3.7"},
						},
					},
				}

				err = kubeClient.Create(context.Background(), differentEndpointSlice)
				Expect(err).NotTo(HaveOccurred())

				result, err := endpointSliceReconciler.Reconcile(context.Background(), req)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())

				req.Name = "another-endpoint-slice"
				result, err = endpointSliceReconciler.Reconcile(context.Background(), req)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())

				req.Name = "different-endpoint-slice"
				result, err = endpointSliceReconciler.Reconcile(context.Background(), req)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())
			})

			It("populates the dns cache with the domain name and endpoints from both EndpointSlices", func() {
				cacheEntries := dnsCache.Lookup("foo.xcc.test")
				Expect(cacheEntries).NotTo(BeEmpty())
				Expect(cacheEntriesToIPStrings(cacheEntries)).To(ConsistOf(expectedIPs))
			})

			When("one EndpointSlice is deleted", func() {
				It("removes the endpoints of deleted EndpointSlice without deleting the record", func() {
					By("deleting one EndpointSlice and reconcile again")
					err := kubeClient.Delete(context.Background(), endpointSlice)
					Expect(err).NotTo(HaveOccurred())

					req.Name = "some-endpoint-slice"
					_, err = endpointSliceReconciler.Reconcile(context.Background(), req)
					Expect(err).NotTo(HaveOccurred())

					By("checking that the DNS doesn't have entries of the deleted EndpointSlice")
					cacheEntries := dnsCache.Lookup("foo.xcc.test")
					Expect(cacheEntries).NotTo(BeEmpty())
					Expect(cacheEntriesToIPStrings(cacheEntries)).To(ConsistOf("2.2.3.4", "2.2.3.5", "2.2.3.6", "2.2.3.7"))
				})
			})
		})

		When("the domain name is an IPv6 domain", func() {
			BeforeEach(func() {
				endpointSlice.AddressType = discoveryv1beta1.AddressTypeIPv6
				endpointSlice.Endpoints = []discoveryv1beta1.Endpoint{
					{Addresses: []string{"::1"}},
				}
				err := kubeClient.Update(context.Background(), endpointSlice)
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not allow lookups on this domain", func() {
				_, err := endpointSliceReconciler.Reconcile(context.Background(), req)
				Expect(err).NotTo(HaveOccurred())

				cacheEntries := dnsCache.Lookup("foo.xcc.test")
				Expect(cacheEntries).To(BeEmpty())
			})
		})

		When("the EndpointSlice is updated", func() {
			It("updates the dns cache", func() {
				By("testing that the reconciler reconciles on the created resource")
				_, err := endpointSliceReconciler.Reconcile(context.Background(), req)
				Expect(err).NotTo(HaveOccurred())

				cacheEntries := dnsCache.Lookup("foo.xcc.test")
				Expect(cacheEntries).NotTo(BeEmpty())
				Expect(cacheEntriesToIPStrings(cacheEntries)).To(ConsistOf(expectedIPs))

				By("updating the EndpointSlice and reconciling again")

				endpointSlice.Endpoints = []discoveryv1beta1.Endpoint{
					{Addresses: []string{"1.2.3.4"}},
				}
				err = kubeClient.Update(context.Background(), endpointSlice)
				Expect(err).NotTo(HaveOccurred())

				_, err = endpointSliceReconciler.Reconcile(context.Background(), req)
				Expect(err).NotTo(HaveOccurred())

				By("checking that the DNS entry is changed")

				cacheEntries = dnsCache.Lookup("foo.xcc.test")
				Expect(cacheEntries).NotTo(BeEmpty())
				Expect(cacheEntriesToIPStrings(cacheEntries)).To(ConsistOf("1.2.3.4"))
			})
		})

		When("the EndpointSlice is deleted", func() {
			It("removes the entry from the dns cache", func() {
				By("testing that the reconciler reconciles on the created resource")
				_, err := endpointSliceReconciler.Reconcile(context.Background(), req)
				Expect(err).NotTo(HaveOccurred())

				cacheEntries := dnsCache.Lookup("foo.xcc.test")
				Expect(cacheEntries).NotTo(BeEmpty())
				Expect(cacheEntriesToIPStrings(cacheEntries)).To(ConsistOf(expectedIPs))

				By("deleting the EndpointSlice and reconciling again")

				err = kubeClient.Delete(context.Background(), endpointSlice)
				Expect(err).NotTo(HaveOccurred())

				_, err = endpointSliceReconciler.Reconcile(context.Background(), req)
				Expect(err).NotTo(HaveOccurred())

				By("checking that the DNS entry no longer exists")

				cacheEntries = dnsCache.Lookup("foo.xcc.test")
				Expect(cacheEntries).To(BeEmpty())
			})
		})
	})

	When("the EndpointSlice does not have a domain name annotation set", func() {
		It("does not allow lookups on any domain", func() {
			_, err := endpointSliceReconciler.Reconcile(context.Background(), req)
			Expect(err).NotTo(HaveOccurred())

			Expect(dnsCache).To(Equal(new(endpointslicedns.DNSCache)))
		})
	})
})

func cacheEntriesToIPStrings(cacheEntries []endpointslicedns.DNSCacheEntry) []string {
	strIPs := []string{}
	for _, entry := range cacheEntries {
		for _, ip := range entry.IPs {
			strIPs = append(strIPs, ip.String())
		}
	}
	return strIPs
}
