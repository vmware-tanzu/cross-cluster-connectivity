// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package httpproxypublish

import (
	"errors"
	"sort"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic/dynamicinformer"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"

	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"

	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/clientset/versioned/fake"
	connectivityinformers "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/informers/externalversions"
)

func Test_Controller(t *testing.T) {
	testcases := []struct {
		name          string
		httpProxy     *contourv1.HTTPProxy
		nodes         []*corev1.Node
		serviceRecord *connectivityv1alpha1.ServiceRecord
	}{
		{
			name: "standard exported HTTPProxy",
			httpProxy: &contourv1.HTTPProxy{
				TypeMeta: metav1.TypeMeta{
					Kind:       "HTTPProxy",
					APIVersion: "projectcontour.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					UID:       "http-proxy-uid",
					Namespace: "http-proxy-namespace",
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
			},
			nodes: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-01",
					},
					Status: corev1.NodeStatus{
						Addresses: []corev1.NodeAddress{
							{
								Address: "10.0.0.1",
								Type:    corev1.NodeInternalIP,
							},
						},
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			},
			serviceRecord: &connectivityv1alpha1.ServiceRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test.some.domain",
					Namespace: "http-proxy-namespace",
					Labels: map[string]string{
						connectivityv1alpha1.ExportLabel: "",
					},
					Annotations: map[string]string{
						connectivityv1alpha1.ServicePortAnnotation: "443",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "projectcontour.io/v1",
							Kind:       "HTTPProxy",
							UID:        "http-proxy-uid",
							Name:       "test",
						},
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
			},
		},
		{
			name: "standard exported HTTPProxy with VIP annotation",
			httpProxy: &contourv1.HTTPProxy{
				TypeMeta: metav1.TypeMeta{
					Kind:       "HTTPProxy",
					APIVersion: "projectcontour.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "http-proxy-namespace",
					UID:       "http-proxy-uid",
					Labels: map[string]string{
						connectivityv1alpha1.ExportLabel: "",
					},
					Annotations: map[string]string{
						connectivityv1alpha1.GlobalVIPAnnotation: "1.2.3.4",
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
			},
			nodes: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-01",
					},
					Status: corev1.NodeStatus{
						Addresses: []corev1.NodeAddress{
							{
								Address: "10.0.0.1",
								Type:    corev1.NodeInternalIP,
							},
						},
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			},
			serviceRecord: &connectivityv1alpha1.ServiceRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test.some.domain",
					Namespace: "http-proxy-namespace",
					Labels: map[string]string{
						connectivityv1alpha1.ExportLabel: "",
					},
					Annotations: map[string]string{
						connectivityv1alpha1.ServicePortAnnotation: "443",
						connectivityv1alpha1.GlobalVIPAnnotation:   "1.2.3.4",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "projectcontour.io/v1",
							Kind:       "HTTPProxy",
							UID:        "http-proxy-uid",
							Name:       "test",
						},
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
			},
		},
		{
			name: "standard exported HTTPProxy with multiple nodes",
			httpProxy: &contourv1.HTTPProxy{
				TypeMeta: metav1.TypeMeta{
					Kind:       "HTTPProxy",
					APIVersion: "projectcontour.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "http-proxy-namespace",
					UID:       "http-proxy-uid",
					Labels: map[string]string{
						connectivityv1alpha1.ExportLabel: "",
					},
					Annotations: map[string]string{
						connectivityv1alpha1.GlobalVIPAnnotation: "1.2.3.4",
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
			},
			nodes: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-01",
					},
					Status: corev1.NodeStatus{
						Addresses: []corev1.NodeAddress{
							{
								Address: "10.0.0.1",
								Type:    corev1.NodeInternalIP,
							},
						},
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-02",
					},
					Status: corev1.NodeStatus{
						Addresses: []corev1.NodeAddress{
							{
								Address: "10.0.0.2",
								Type:    corev1.NodeInternalIP,
							},
						},
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			},
			serviceRecord: &connectivityv1alpha1.ServiceRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test.some.domain",
					Namespace: "http-proxy-namespace",
					Labels: map[string]string{
						connectivityv1alpha1.ExportLabel: "",
					},
					Annotations: map[string]string{
						connectivityv1alpha1.ServicePortAnnotation: "443",
						connectivityv1alpha1.GlobalVIPAnnotation:   "1.2.3.4",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "projectcontour.io/v1",
							Kind:       "HTTPProxy",
							UID:        "http-proxy-uid",
							Name:       "test",
						},
					},
				},
				Spec: connectivityv1alpha1.ServiceRecordSpec{
					FQDN: "test.some.domain",
					Endpoints: []connectivityv1alpha1.Endpoint{
						{
							Address: "10.0.0.1",
							Port:    443,
						},
						{
							Address: "10.0.0.2",
							Port:    443,
						},
					},
				},
			},
		},
		{
			name: "standard exported HTTPProxy with node notready",
			httpProxy: &contourv1.HTTPProxy{
				TypeMeta: metav1.TypeMeta{
					Kind:       "HTTPProxy",
					APIVersion: "projectcontour.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "http-proxy-namespace",
					UID:       "http-proxy-uid",
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
			},
			nodes: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-01",
					},
					Status: corev1.NodeStatus{
						Addresses: []corev1.NodeAddress{
							{
								Address: "10.0.0.1",
								Type:    corev1.NodeInternalIP,
							},
						},
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionFalse,
							},
						},
					},
				},
			},
			serviceRecord: &connectivityv1alpha1.ServiceRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test.some.domain",
					Namespace: "http-proxy-namespace",
					Labels: map[string]string{
						connectivityv1alpha1.ExportLabel: "",
					},
					Annotations: map[string]string{
						connectivityv1alpha1.ServicePortAnnotation: "443",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "projectcontour.io/v1",
							Kind:       "HTTPProxy",
							UID:        "http-proxy-uid",
							Name:       "test",
						},
					},
				},
				Spec: connectivityv1alpha1.ServiceRecordSpec{
					FQDN:      "test.some.domain",
					Endpoints: nil,
				},
			},
		},
		{
			name: "standard exported HTTPProxy with tolerable node taint",
			httpProxy: &contourv1.HTTPProxy{
				TypeMeta: metav1.TypeMeta{
					Kind:       "HTTPProxy",
					APIVersion: "projectcontour.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "http-proxy-namespace",
					UID:       "http-proxy-uid",
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
			},
			nodes: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-01",
					},
					Status: corev1.NodeStatus{
						Addresses: []corev1.NodeAddress{
							{
								Address: "10.0.0.1",
								Type:    corev1.NodeInternalIP,
							},
						},
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
					Spec: corev1.NodeSpec{
						Taints: []corev1.Taint{
							{
								Key:    corev1.TaintNodeNotReady,
								Effect: corev1.TaintEffectNoExecute,
							},
						},
					},
				},
			},
			serviceRecord: &connectivityv1alpha1.ServiceRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test.some.domain",
					Namespace: "http-proxy-namespace",
					Labels: map[string]string{
						connectivityv1alpha1.ExportLabel: "",
					},
					Annotations: map[string]string{
						connectivityv1alpha1.ServicePortAnnotation: "443",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "projectcontour.io/v1",
							Kind:       "HTTPProxy",
							UID:        "http-proxy-uid",
							Name:       "test",
						},
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
			},
		},
		{
			name: "standard exported HTTPProxy with multiple nodes, one node taint is not intolerable by daemonSet",
			httpProxy: &contourv1.HTTPProxy{
				TypeMeta: metav1.TypeMeta{
					Kind:       "HTTPProxy",
					APIVersion: "projectcontour.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "http-proxy-namespace",
					UID:       "http-proxy-uid",
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
			},
			nodes: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-01",
					},
					Status: corev1.NodeStatus{
						Addresses: []corev1.NodeAddress{
							{
								Address: "10.0.0.1",
								Type:    corev1.NodeInternalIP,
							},
						},
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
					Spec: corev1.NodeSpec{
						Taints: []corev1.Taint{
							{
								Key:    "TaintKeyNotExist",
								Effect: "TaintEffectNotExist",
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-02",
					},
					Status: corev1.NodeStatus{
						Addresses: []corev1.NodeAddress{
							{
								Address: "10.0.0.2",
								Type:    corev1.NodeInternalIP,
							},
						},
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
					Spec: corev1.NodeSpec{
						Taints: []corev1.Taint{
							{
								Key:    corev1.TaintNodeNotReady,
								Effect: corev1.TaintEffectNoExecute,
							},
						},
					},
				},
			},
			serviceRecord: &connectivityv1alpha1.ServiceRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test.some.domain",
					Namespace: "http-proxy-namespace",
					Labels: map[string]string{
						connectivityv1alpha1.ExportLabel: "",
					},
					Annotations: map[string]string{
						connectivityv1alpha1.ServicePortAnnotation: "443",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "projectcontour.io/v1",
							Kind:       "HTTPProxy",
							UID:        "http-proxy-uid",
							Name:       "test",
						},
					},
				},
				Spec: connectivityv1alpha1.ServiceRecordSpec{
					FQDN: "test.some.domain",
					Endpoints: []connectivityv1alpha1.Endpoint{
						{
							Address: "10.0.0.2",
							Port:    443,
						},
					},
				},
			},
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			connClientset, dynamicClientset := setupTestcase(t, testcase.httpProxy, testcase.nodes)

			checkServiceRecord := func() (bool, error) {
				serviceRecord, err := connClientset.ConnectivityV1alpha1().
					ServiceRecords(testcase.serviceRecord.Namespace).
					Get(testcase.serviceRecord.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}

				// Endpoints may be out of order, sort for determinstic results
				sort.Slice(testcase.serviceRecord.Spec.Endpoints, func(i, j int) bool {
					return testcase.serviceRecord.Spec.Endpoints[i].Address < testcase.serviceRecord.Spec.Endpoints[j].Address
				})
				sort.Slice(serviceRecord.Spec.Endpoints, func(i, j int) bool {
					return serviceRecord.Spec.Endpoints[i].Address < serviceRecord.Spec.Endpoints[j].Address
				})

				if !apiequality.Semantic.DeepEqual(serviceRecord, testcase.serviceRecord) {
					t.Logf("actual ServiceRecord: %v", serviceRecord)
					t.Logf("expected ServiceRecord: %v", testcase.serviceRecord)
					return true, errors.New("ServiceRecords were not equal")
				}

				return true, nil
			}

			if err := wait.Poll(100*time.Millisecond, 5*time.Second, checkServiceRecord); err != nil {
				t.Errorf("ServiceRecord did not match: %v", err)
			}

			delete(testcase.httpProxy.ObjectMeta.Labels, connectivityv1alpha1.ExportLabel)
			newUnstructuredHTTPProxy, err := runtime.DefaultUnstructuredConverter.ToUnstructured(testcase.httpProxy)
			if err != nil {
				t.Fatalf("error converting HTTPProxy to unstructured: %v", err)
			}
			newHttpProxy := &unstructured.Unstructured{
				Object: newUnstructuredHTTPProxy,
			}

			if _, err = dynamicClientset.Resource(contourv1.HTTPProxyGVR).Namespace(testcase.httpProxy.Namespace).Update(newHttpProxy, metav1.UpdateOptions{}); err != nil {
				t.Fatalf("error updating HTTPProxy resource to delete the label: %v", err)
			}

			checkServiceRecordIsDeleted := func() (bool, error) {
				_, err := connClientset.ConnectivityV1alpha1().
					ServiceRecords(testcase.serviceRecord.Namespace).
					Get(testcase.serviceRecord.Name, metav1.GetOptions{})
				if err != nil {
					if k8serrors.IsNotFound(err) {
						return true, nil
					}
					return false, err
				}

				return false, nil
			}

			if err := wait.Poll(100*time.Millisecond, 5*time.Second, checkServiceRecordIsDeleted); err != nil {
				t.Errorf("error checking ServiceRecord is deleted: %v", err)
			}
		})
	}
}

func Test_UnexportedHTTPProxy(t *testing.T) {
	var httpProxy = &contourv1.HTTPProxy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HTTPProxy",
			APIVersion: "projectcontour.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "http-proxy-namespace",
			UID:       "http-proxy-uid",
			Labels:    map[string]string{},
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
	var nodes = []*corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-01",
			},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{
					{
						Address: "10.0.0.1",
						Type:    corev1.NodeInternalIP,
					},
				},
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
	}

	t.Run("unexported HTTPProxy", func(t *testing.T) {
		connClientset, _ := setupTestcase(t, httpProxy, nodes)

		checkServiceRecordDoesNotExist := func() (bool, error) {
			_, err := connClientset.ConnectivityV1alpha1().
				ServiceRecords("http-proxy-namespace").
				Get("test.some.domain", metav1.GetOptions{})
			if err != nil {
				if k8serrors.IsNotFound(err) {
					return true, nil
				}

				return false, err
			}

			return false, errors.New("expected service record to not exist")
		}

		if err := wait.Poll(100*time.Millisecond, 5*time.Second, checkServiceRecordDoesNotExist); err != nil {
			t.Errorf("checkServiceRecordDoesNotExist returned error: %v", err)
		}
	})
}

func Test_DeleteHttpProxy(t *testing.T) {
	var httpProxy = &contourv1.HTTPProxy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HTTPProxy",
			APIVersion: "projectcontour.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "http-proxy-namespace",
			UID:       "http-proxy-uid",
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
	var nodes = []*corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-01",
			},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{
					{
						Address: "10.0.0.1",
						Type:    corev1.NodeInternalIP,
					},
				},
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
	}
	var serviceRecord = &connectivityv1alpha1.ServiceRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test.some.domain",
			Namespace: httpProxy.Namespace,
			Labels: map[string]string{
				connectivityv1alpha1.ExportLabel: "",
			},
			Annotations: map[string]string{
				connectivityv1alpha1.ServicePortAnnotation: "443",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "projectcontour.io/v1",
					Kind:       "HTTPProxy",
					UID:        "http-proxy-uid",
					Name:       "test",
				},
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

	t.Run("delete HTTPProxy", func(t *testing.T) {
		connClientset, dynamicClientset := setupTestcase(t, httpProxy, nodes)

		checkServiceRecord := func() (bool, error) {
			actualServiceRecord, err := connClientset.ConnectivityV1alpha1().
				ServiceRecords(httpProxy.Namespace).
				Get(serviceRecord.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}

			// Endpoints may be out of order, sort for determinstic results
			sort.Slice(serviceRecord.Spec.Endpoints, func(i, j int) bool {
				return serviceRecord.Spec.Endpoints[i].Address < serviceRecord.Spec.Endpoints[j].Address
			})
			sort.Slice(actualServiceRecord.Spec.Endpoints, func(i, j int) bool {
				return actualServiceRecord.Spec.Endpoints[i].Address < actualServiceRecord.Spec.Endpoints[j].Address
			})

			if !apiequality.Semantic.DeepEqual(actualServiceRecord, serviceRecord) {
				t.Logf("actual ServiceRecord: %v", actualServiceRecord)
				t.Logf("expected ServiceRecord: %v", serviceRecord)
				return true, errors.New("ServiceRecords were not equal")
			}

			return true, nil
		}

		if err := wait.Poll(100*time.Millisecond, 5*time.Second, checkServiceRecord); err != nil {
			t.Errorf("ServiceRecord did not match: %v", err)
		}

		if err := dynamicClientset.Resource(contourv1.HTTPProxyGVR).Namespace(httpProxy.Namespace).
			Delete(httpProxy.Name, &metav1.DeleteOptions{}); err != nil {
			t.Fatalf("error updating HTTPProxy resource to delete the label: %v", err)
		}

		checkServiceRecordDoesNotExist := func() (bool, error) {
			_, err := connClientset.ConnectivityV1alpha1().
				ServiceRecords(httpProxy.Namespace).
				Get(serviceRecord.Name, metav1.GetOptions{})
			if err != nil {
				if k8serrors.IsNotFound(err) {
					return true, nil
				}

				return false, err
			}

			return false, errors.New("expected service record to not exist")
		}

		if err := wait.Poll(100*time.Millisecond, 5*time.Second, checkServiceRecordDoesNotExist); err != nil {
			t.Errorf("checkServiceRecordDoesNotExist returned error: %v", err)
		}
	})
}

func setupTestcase(t *testing.T, testCaseHttpProxy *contourv1.HTTPProxy, testCaseNodes []*corev1.Node) (*fake.Clientset, *dynamicfake.FakeDynamicClient) {
	scheme := runtime.NewScheme()
	if err := ContourV1AddToScheme(scheme); err != nil {
		t.Fatal("error adding contour types to scheme")
	}

	nodeList := make([]runtime.Object, len(testCaseNodes))
	for i, node := range testCaseNodes {
		nodeList[i] = node
	}

	unstructuredHTTPProxy, err := runtime.DefaultUnstructuredConverter.ToUnstructured(testCaseHttpProxy)
	if err != nil {
		t.Fatalf("error converting HTTPProxy to unstructured")
	}
	httpProxy := &unstructured.Unstructured{
		Object: unstructuredHTTPProxy,
	}

	connClientset := fake.NewSimpleClientset()
	kubeClientset := kubefake.NewSimpleClientset(nodeList...)
	dynamicClientset := dynamicfake.NewSimpleDynamicClient(scheme, httpProxy)
	coreInformerFactory := informers.NewSharedInformerFactory(kubeClientset, 30*time.Second)
	nodeInformer := coreInformerFactory.Core().V1().Nodes()

	contourInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClientset, 30*time.Second)
	contourInformer := contourInformerFactory.ForResource(contourv1.HTTPProxyGVR)

	connectivityInformerFactory := connectivityinformers.NewSharedInformerFactory(connClientset, 30*time.Second)
	serviceRecordInformer := connectivityInformerFactory.Connectivity().V1alpha1().ServiceRecords()

	httpProxyPublishController, err := NewHTTPProxyPublishController(
		nodeInformer, contourInformer, serviceRecordInformer, connClientset)
	if err != nil {
		t.Fatalf("error creating HTTPProxyPublish controller: %v", err)
	}

	contourInformerFactory.Start(nil)
	coreInformerFactory.Start(nil)
	connectivityInformerFactory.Start(nil)

	contourInformerFactory.WaitForCacheSync(nil)
	coreInformerFactory.WaitForCacheSync(nil)
	connectivityInformerFactory.WaitForCacheSync(nil)

	go httpProxyPublishController.Run(1, nil)

	return connClientset, dynamicClientset
}
