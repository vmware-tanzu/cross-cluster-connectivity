// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package servicedns_test

import (
	"context"
	"fmt"
	"time"

	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/servicedns"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServiceDNS", func() {
	var (
		kubeClientset *kubefake.Clientset
		dnsCache      *servicedns.DNSCache
		service       *corev1.Service
	)

	BeforeEach(func() {
		kubeClientset = kubefake.NewSimpleClientset()
		kubeInformerFactory := informers.NewSharedInformerFactory(kubeClientset, 30*time.Second)
		serviceInformer := kubeInformerFactory.Core().V1().Services()

		dnsCache = new(servicedns.DNSCache)

		serviceDNSController := servicedns.NewServiceDNSController(serviceInformer, dnsCache)

		kubeInformerFactory.Start(nil)
		kubeInformerFactory.WaitForCacheSync(nil)

		go serviceDNSController.Run(1, nil)
	})

	When("a Service exists with a FQDN annotation", func() {
		BeforeEach(func() {
			service = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-service",
					Namespace: "cross-cluster-connectivity",
					Annotations: map[string]string{
						connectivityv1alpha1.FQDNAnnotation: "some-service.some.domain",
					},
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: "1.2.3.4",
				},
			}

			_, err := kubeClientset.CoreV1().Services("cross-cluster-connectivity").Create(context.Background(), service, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("populates the dns cache with the FQDN and ClusterIP from the Service", func() {
			Eventually(func() ([]string, error) {
				cacheEntry := dnsCache.Lookup("some-service.some.domain")
				if cacheEntry == nil {
					return []string{}, fmt.Errorf("fqdn some-service.some.domain does not exist in dns cache")
				}
				return ipsToStrings(cacheEntry.IPs), nil
			}, time.Second*5, time.Second).Should(ConsistOf("1.2.3.4"))
		})

		When("the service is updated", func() {
			BeforeEach(func() {
				Eventually(func() ([]string, error) {
					cacheEntry := dnsCache.Lookup("some-service.some.domain")
					if cacheEntry == nil {
						return []string{}, fmt.Errorf("fqdn some-service.some.domain does not exist in dns cache")
					}
					return ipsToStrings(cacheEntry.IPs), nil
				}, time.Second*5, time.Second).Should(ConsistOf("1.2.3.4"))
			})

			It("updates the dns cache", func() {
				service.ObjectMeta.Annotations[connectivityv1alpha1.FQDNAnnotation] = "some-edited-service.some.domain"
				_, err := kubeClientset.CoreV1().Services("cross-cluster-connectivity").Update(context.Background(), service, metav1.UpdateOptions{})
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() ([]string, error) {
					cacheEntry := dnsCache.Lookup("some-edited-service.some.domain")
					if cacheEntry == nil {
						return []string{}, fmt.Errorf("fqdn some-edited-service.some.domain does not exist in dns cache")
					}
					return ipsToStrings(cacheEntry.IPs), nil
				}, time.Second*5, time.Second).Should(ConsistOf("1.2.3.4"))
				Expect(dnsCache.Lookup("some-service.some.domain")).To(BeNil())
			})
		})
	})

	When("a Service is deleted", func() {
		BeforeEach(func() {
			service = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-service",
					Namespace: "cross-cluster-connectivity",
					Annotations: map[string]string{
						connectivityv1alpha1.FQDNAnnotation: "some-service.some.domain",
					},
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: "1.2.3.4",
				},
			}

			_, err := kubeClientset.CoreV1().Services("cross-cluster-connectivity").Create(context.Background(), service, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]string, error) {
				cacheEntry := dnsCache.Lookup("some-service.some.domain")
				if cacheEntry == nil {
					return []string{}, fmt.Errorf("fqdn some-service.some.domain does not exist in dns cache")
				}
				return ipsToStrings(cacheEntry.IPs), nil
			}, time.Second*5, time.Second).Should(ConsistOf("1.2.3.4"))

			err = kubeClientset.CoreV1().Services("cross-cluster-connectivity").Delete(context.Background(), "some-service", metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("removes the entry from the dns cache", func() {
			Eventually(func() []string {
				cacheEntry := dnsCache.Lookup("some-service.some.domain")
				if cacheEntry == nil {
					return []string{}
				}
				return ipsToStrings(cacheEntry.IPs)
			}, time.Second*5, time.Second).Should(BeEmpty())
		})
	})
})
