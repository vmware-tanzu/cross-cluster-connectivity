// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package orphandeleter_test

import (
	"context"
	"log"
	"time"

	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/registryclient/internal/orphandeleter"
	clientsetfake "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/clientset/versioned/fake"
	connectivityinformers "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/informers/externalversions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apmachinerytypes "k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
)

var _ = Describe("OrphanDeleter", func() {
	var (
		connClientset *clientsetfake.Clientset
		orphanDeleter *orphandeleter.OrphanDeleter
	)

	BeforeEach(func() {
		log.SetOutput(GinkgoWriter)

		connClientset = clientsetfake.NewSimpleClientset()
		connectivityInformerFactory := connectivityinformers.NewSharedInformerFactory(connClientset, 30*time.Second)
		serviceRecordInformer := connectivityInformerFactory.Connectivity().V1alpha1().ServiceRecords()
		deleteDelay := 100 * time.Millisecond
		orphanDeleter = orphandeleter.NewOrphanDeleter(
			serviceRecordInformer.Lister(),
			connClientset,
			"cross-cluster-connectivity",
			deleteDelay,
		)

		_, err := connClientset.ConnectivityV1alpha1().ServiceRecords("cross-cluster-connectivity").
			Create(context.Background(), &connectivityv1alpha1.ServiceRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-service.some.domain-06df7236",
					Namespace: "cross-cluster-connectivity",
				},
				Spec: connectivityv1alpha1.ServiceRecordSpec{
					FQDN: "some-service.some.domain",
				},
			}, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		_, err = connClientset.ConnectivityV1alpha1().ServiceRecords("cross-cluster-connectivity").
			Create(context.Background(), &connectivityv1alpha1.ServiceRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "orphaned-service-record-06df7236",
					Namespace: "cross-cluster-connectivity",
				},
				Spec: connectivityv1alpha1.ServiceRecordSpec{
					FQDN: "orphaned-service.some.domain",
				},
			}, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		connectivityInformerFactory.Start(nil)
		connectivityInformerFactory.WaitForCacheSync(nil)
	})

	It("deletes service records that have not been removed prior to the timer running out", func() {
		orphanDeleter.AddRemoteServiceRecord(apmachinerytypes.NamespacedName{
			Namespace: "cross-cluster-connectivity",
			Name:      "some-service.some.domain-06df7236",
		})

		Eventually(func() (*connectivityv1alpha1.ServiceRecordList, error) {
			return connClientset.ConnectivityV1alpha1().ServiceRecords("cross-cluster-connectivity").
				List(context.Background(), metav1.ListOptions{})
		}, 2*time.Second, 100*time.Millisecond).Should(
			MatchServiceRecords(
				gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"ObjectMeta": gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
						"Name": Equal("some-service.some.domain-06df7236"),
					}),
				}),
			),
		)
	})

	It("does not remember service records added before a Reset", func() {
		orphanDeleter.AddRemoteServiceRecord(apmachinerytypes.NamespacedName{
			Namespace: "cross-cluster-connectivity",
			Name:      "some-service.some.domain-06df7236",
		})
		orphanDeleter.Reset()

		Consistently(func() (*connectivityv1alpha1.ServiceRecordList, error) {
			return connClientset.ConnectivityV1alpha1().ServiceRecords("cross-cluster-connectivity").
				List(context.Background(), metav1.ListOptions{})
		}, 1*time.Second, 100*time.Millisecond).Should(
			MatchServiceRecords(
				gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"ObjectMeta": gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
						"Name": Equal("some-service.some.domain-06df7236"),
					}),
				}),
				gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"ObjectMeta": gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
						"Name": Equal("orphaned-service-record-06df7236"),
					}),
				}),
			),
		)

		orphanDeleter.AddRemoteServiceRecord(apmachinerytypes.NamespacedName{
			Namespace: "cross-cluster-connectivity",
			Name:      "some-service.some.domain-06df7236",
		})

		Eventually(func() (*connectivityv1alpha1.ServiceRecordList, error) {
			return connClientset.ConnectivityV1alpha1().ServiceRecords("cross-cluster-connectivity").
				List(context.Background(), metav1.ListOptions{})
		}, 2*time.Second, 100*time.Millisecond).Should(
			MatchServiceRecords(
				gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
					"ObjectMeta": gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
						"Name": Equal("some-service.some.domain-06df7236"),
					}),
				}),
			),
		)
	})
})

func MatchServiceRecords(matchers ...types.GomegaMatcher) types.GomegaMatcher {
	return WithTransform(transformServiceRecordListToItems, ConsistOf(matchers))
}

func transformServiceRecordListToItems(srl *connectivityv1alpha1.ServiceRecordList) []connectivityv1alpha1.ServiceRecord {
	if srl == nil || len(srl.Items) == 0 {
		return nil
	}
	return srl.Items
}
