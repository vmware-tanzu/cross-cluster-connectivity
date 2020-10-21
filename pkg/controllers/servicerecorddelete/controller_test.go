// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package servicerecorddelete_test

import (
	"time"

	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/servicerecorddelete"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/clientset/versioned/fake"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"

	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	connectivityinformers "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/informers/externalversions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServiceRecordOrphanDeleteController", func() {
	var (
		connClientset    *fake.Clientset
		dynamicClientset *dynamicfake.FakeDynamicClient
		contourInformer  informers.GenericInformer

		httpProxy     *contourv1.HTTPProxy
		serviceRecord *connectivityv1alpha1.ServiceRecord
	)

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		err := servicerecorddelete.ContourV1AddToScheme(scheme)
		Expect(err).NotTo(HaveOccurred())

		connClientset = fake.NewSimpleClientset()
		dynamicClientset = dynamicfake.NewSimpleDynamicClient(scheme)

		contourInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClientset, 30*time.Second)
		contourInformer = contourInformerFactory.ForResource(contourv1.HTTPProxyGVR)

		connectivityInformerFactory := connectivityinformers.NewSharedInformerFactory(connClientset, 30*time.Second)
		serviceRecordInformer := connectivityInformerFactory.Connectivity().V1alpha1().ServiceRecords()

		serviceRecordOrphanDeleteController, err := servicerecorddelete.NewServiceRecordOrphanDeleteController(
			connClientset, serviceRecordInformer, contourInformer)
		Expect(err).NotTo(HaveOccurred())

		contourInformerFactory.Start(nil)
		connectivityInformerFactory.Start(nil)

		contourInformerFactory.WaitForCacheSync(nil)
		connectivityInformerFactory.WaitForCacheSync(nil)

		go serviceRecordOrphanDeleteController.Run(1, nil)

		httpProxy = &contourv1.HTTPProxy{
			TypeMeta: metav1.TypeMeta{
				Kind:       "HTTPProxy",
				APIVersion: "projectcontour.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "default",
				Labels: map[string]string{
					connectivityv1alpha1.ExportLabel: "",
				},
			},
			Spec: contourv1.HTTPProxySpec{
				VirtualHost: &contourv1.VirtualHost{
					Fqdn: "test.some.domain",
				},
				Routes: []contourv1.Route{
					{
						Services: []contourv1.Service{
							{
								Name: "foobar",
								Port: 443,
							},
						},
					},
				},
			},
		}

		serviceRecord = &connectivityv1alpha1.ServiceRecord{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test.some.domain",
				Namespace: "cross-cluster-connectivity",
				Labels: map[string]string{
					connectivityv1alpha1.ExportLabel: "",
				},
				Annotations: map[string]string{
					connectivityv1alpha1.ServicePortAnnotation: "443",
				},
			},
			Spec: connectivityv1alpha1.ServiceRecordSpec{
				FQDN: "test.some.domain",
				Endpoints: []connectivityv1alpha1.Endpoint{
					{
						Address: "10.0.0.1",
						Port:    443,
					},
				},
			},
		}
	})

	When("the service record does not have a parent", func() {
		BeforeEach(func() {
			_, err := connClientset.ConnectivityV1alpha1().ServiceRecords("cross-cluster-connectivity").
				Create(serviceRecord)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]connectivityv1alpha1.ServiceRecord, error) {
				serviceRecordList, err := connClientset.ConnectivityV1alpha1().ServiceRecords("cross-cluster-connectivity").
					List(metav1.ListOptions{})
				return serviceRecordList.Items, err
			}, time.Second, 100*time.Millisecond).Should(HaveLen(1))
		})

		It("deletes the service record", func() {
			Eventually(func() ([]connectivityv1alpha1.ServiceRecord, error) {
				serviceRecordList, err := connClientset.ConnectivityV1alpha1().ServiceRecords("cross-cluster-connectivity").
					List(metav1.ListOptions{})
				return serviceRecordList.Items, err
			}).Should(HaveLen(0))
		})
	})

	When("the service record does have a parent", func() {
		BeforeEach(func() {
			unstructuredHTTPProxy, err := runtime.DefaultUnstructuredConverter.ToUnstructured(httpProxy)
			Expect(err).NotTo(HaveOccurred())

			_, err = dynamicClientset.Resource(contourv1.HTTPProxyGVR).Namespace(metav1.NamespaceDefault).
				Create(&unstructured.Unstructured{
					Object: unstructuredHTTPProxy,
				}, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]runtime.Object, error) {
				return contourInformer.Lister().List(labels.Everything())
			}, time.Second, 100*time.Millisecond).Should(HaveLen(1))

			_, err = connClientset.ConnectivityV1alpha1().ServiceRecords("cross-cluster-connectivity").
				Create(serviceRecord)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ([]connectivityv1alpha1.ServiceRecord, error) {
				serviceRecordList, err := connClientset.ConnectivityV1alpha1().ServiceRecords("cross-cluster-connectivity").
					List(metav1.ListOptions{})
				return serviceRecordList.Items, err
			}, time.Second, 100*time.Millisecond).Should(HaveLen(1))
		})

		It("does not delete the service record", func() {
			Consistently(func() ([]connectivityv1alpha1.ServiceRecord, error) {
				serviceRecordList, err := connClientset.ConnectivityV1alpha1().ServiceRecords("cross-cluster-connectivity").
					List(metav1.ListOptions{})
				return serviceRecordList.Items, err
			}, time.Second, 100*time.Millisecond).Should(HaveLen(1))
		})
	})
})
