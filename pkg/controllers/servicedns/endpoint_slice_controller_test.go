// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package servicedns_test

import (
	"context"
	"fmt"
	"net"
	"time"

	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/servicedns"
	discoveryv1beta1 "k8s.io/api/discovery/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("EndpointSliceDNS", func() {
	var (
		kubeClientset *kubefake.Clientset
		dnsCache      *servicedns.DNSCache

		endpointSlice *discoveryv1beta1.EndpointSlice
	)

	BeforeEach(func() {
		kubeClientset = kubefake.NewSimpleClientset()
		kubeInformerFactory := informers.NewSharedInformerFactory(kubeClientset, 30*time.Second)
		endpointSliceInformer := kubeInformerFactory.Discovery().V1beta1().EndpointSlices()

		dnsCache = new(servicedns.DNSCache)

		endpointSliceDNSController := servicedns.NewEndpointSliceDNSController(endpointSliceInformer, dnsCache)

		kubeInformerFactory.Start(nil)
		kubeInformerFactory.WaitForCacheSync(nil)

		go endpointSliceDNSController.Run(1, nil)
	})

	When("an EndpointSlice exists with a DNS hostname annotation", func() {
		BeforeEach(func() {
			endpointSlice = &discoveryv1beta1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-endpoint-slice",
					Namespace: "cross-cluster-connectivity",
					Annotations: map[string]string{
						connectivityv1alpha1.DNSHostnameAnnotation: "foo.xcc.test",
					},
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

			_, err := kubeClientset.DiscoveryV1beta1().EndpointSlices("cross-cluster-connectivity").
				Create(context.Background(), endpointSlice, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("populates the dns cache with the domain name and endpoints from the EndpointSlice", func() {
			Eventually(func() ([]string, error) {
				cacheEntry := dnsCache.Lookup("foo.xcc.test")
				if cacheEntry == nil {
					return []string{}, fmt.Errorf("fqdn foo.xcc.test does not exist in dns cache")
				}
				return ipsToStrings(cacheEntry.IPs), nil
			}, time.Second*5, time.Second).Should(ConsistOf("1.2.3.4", "1.2.3.5", "1.2.3.6", "1.2.3.7"))
		})

		When("the domain name is a wildcard domain", func() {
			BeforeEach(func() {
				endpointSlice = &discoveryv1beta1.EndpointSlice{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "some-wildcard-endpoint-slice",
						Namespace: "cross-cluster-connectivity",
						Annotations: map[string]string{
							connectivityv1alpha1.DNSHostnameAnnotation: "*.gateway.xcc.test",
						},
					},
					AddressType: discoveryv1beta1.AddressTypeIPv4,
					Endpoints: []discoveryv1beta1.Endpoint{
						{
							Addresses: []string{"1.2.3.4"},
						},
					},
				}

				_, err := kubeClientset.DiscoveryV1beta1().EndpointSlices("cross-cluster-connectivity").
					Create(context.Background(), endpointSlice, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())
			})

			It("can lookup any domain on the wildcard domain", func() {
				Eventually(func() ([]string, error) {
					cacheEntry := dnsCache.Lookup("foo.gateway.xcc.test")
					if cacheEntry == nil {
						return []string{}, fmt.Errorf("fqdn foo.gateway.xcc.test does not exist in dns cache")
					}
					return ipsToStrings(cacheEntry.IPs), nil
				}, time.Second*5, time.Second).Should(ConsistOf("1.2.3.4"))

				Eventually(func() ([]string, error) {
					cacheEntry := dnsCache.Lookup("bar.gateway.xcc.test")
					if cacheEntry == nil {
						return []string{}, fmt.Errorf("fqdn bar.gateway.xcc.test does not exist in dns cache")
					}
					return ipsToStrings(cacheEntry.IPs), nil
				}, time.Second*5, time.Second).Should(ConsistOf("1.2.3.4"))
			})
		})

		When("the domain name is an IPv6 domain", func() {
			BeforeEach(func() {
				endpointSlice = &discoveryv1beta1.EndpointSlice{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "some-ipv4-endpoint-slice",
						Namespace: "cross-cluster-connectivity",
						Annotations: map[string]string{
							connectivityv1alpha1.DNSHostnameAnnotation: "bar.xcc.test",
						},
					},
					AddressType: discoveryv1beta1.AddressTypeIPv6,
					Endpoints: []discoveryv1beta1.Endpoint{
						{
							Addresses: []string{"::1"},
						},
					},
				}

				_, err := kubeClientset.DiscoveryV1beta1().EndpointSlices("cross-cluster-connectivity").
					Create(context.Background(), endpointSlice, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())
			})

			It("does not allow lookups on this domain", func() {
				Consistently(func() error {
					cacheEntry := dnsCache.Lookup("bar.xcc.test")
					if cacheEntry == nil {
						return fmt.Errorf("fqdn bar.xcc.test does not exist in dns cache")
					}
					return nil
				}, time.Second*5, time.Second).ShouldNot(Succeed())
			})
		})

		When("the EndpointSlice is updated", func() {
			BeforeEach(func() {
				Eventually(func() ([]string, error) {
					cacheEntry := dnsCache.Lookup("foo.xcc.test")
					if cacheEntry == nil {
						return []string{}, fmt.Errorf("fqdn foo.xcc.test does not exist in dns cache")
					}
					return ipsToStrings(cacheEntry.IPs), nil
				}, time.Second*5, time.Second).Should(ConsistOf("1.2.3.4", "1.2.3.5", "1.2.3.6", "1.2.3.7"))
			})

			It("updates the dns cache", func() {
				endpointSlice.ObjectMeta.Annotations[connectivityv1alpha1.DNSHostnameAnnotation] = "*.updated-gateway.xcc.test"
				_, err := kubeClientset.DiscoveryV1beta1().EndpointSlices("cross-cluster-connectivity").
					Update(context.Background(), endpointSlice, metav1.UpdateOptions{})
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() ([]string, error) {
					cacheEntry := dnsCache.Lookup("foo.updated-gateway.xcc.test")
					if cacheEntry == nil {
						return []string{}, fmt.Errorf("fqdn foo.updated-gateway.xcc.test does not exist in dns cache")
					}
					return ipsToStrings(cacheEntry.IPs), nil
				}, time.Second*5, time.Second).Should(ConsistOf("1.2.3.4", "1.2.3.5", "1.2.3.6", "1.2.3.7"))
				Expect(dnsCache.Lookup("foo.gateway.xcc.test")).To(BeNil())
			})
		})
	})

	When("the EndpointSlice does not have a domain name annotation set", func() {
		BeforeEach(func() {
			endpointSlice = &discoveryv1beta1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "some-ipv4-endpoint-slice",
					Namespace:   "cross-cluster-connectivity",
					Annotations: map[string]string{},
				},
				AddressType: discoveryv1beta1.AddressTypeIPv4,
				Endpoints: []discoveryv1beta1.Endpoint{
					{
						Addresses: []string{"10.4.5.6"},
					},
				},
			}

			_, err := kubeClientset.DiscoveryV1beta1().EndpointSlices("cross-cluster-connectivity").
				Create(context.Background(), endpointSlice, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("does not allow lookups on this domain", func() {
			Consistently(func() error {
				cacheEntry := dnsCache.Lookup("")
				if cacheEntry == nil {
					return fmt.Errorf("fqdn '' does not exist in dns cache")
				}
				return nil
			}, time.Second*5, time.Second).ShouldNot(Succeed())
		})
	})

	When("a EndpointSlice is deleted", func() {
		BeforeEach(func() {
			endpointSlice = &discoveryv1beta1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-endpoint-slice",
					Namespace: "cross-cluster-connectivity",
					Annotations: map[string]string{
						connectivityv1alpha1.DNSHostnameAnnotation: "*.gateway.xcc.test",
					},
				},
				AddressType: discoveryv1beta1.AddressTypeIPv4,
				Endpoints: []discoveryv1beta1.Endpoint{
					{
						Addresses: []string{"1.2.3.4"},
					},
				},
			}

			_, err := kubeClientset.DiscoveryV1beta1().EndpointSlices("cross-cluster-connectivity").
				Create(context.Background(), endpointSlice, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]string, error) {
				cacheEntry := dnsCache.Lookup("foo.gateway.xcc.test")
				if cacheEntry == nil {
					return []string{}, fmt.Errorf("fqdn foo.gateway.xcc.test does not exist in dns cache")
				}
				return ipsToStrings(cacheEntry.IPs), nil
			}, time.Second*5, time.Second).Should(ConsistOf("1.2.3.4"))

			err = kubeClientset.DiscoveryV1beta1().EndpointSlices("cross-cluster-connectivity").
				Delete(context.Background(), "some-endpoint-slice", metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("removes the entry from the dns cache", func() {
			Eventually(func() []string {
				cacheEntry := dnsCache.Lookup("foo.gateway.xcc.test")
				if cacheEntry == nil {
					return []string{}
				}
				return ipsToStrings(cacheEntry.IPs)
			}, time.Second*5, time.Second).Should(BeEmpty())
		})
	})
})

func ipsToStrings(ips []net.IP) []string {
	strIPs := []string{}
	for _, ip := range ips {
		strIPs = append(strIPs, ip.String())
	}

	return strIPs
}
