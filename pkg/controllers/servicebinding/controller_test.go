// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package servicebinding

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/onsi/gomega/format"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"

	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/clientset/versioned/fake"
	connectivityinformers "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/informers/externalversions"
)

func Test_Controller(t *testing.T) {
	testcases := []struct {
		name           string
		serviceRecords []*connectivityv1alpha1.ServiceRecord
		service        *corev1.Service
		endpoints      *corev1.Endpoints
	}{
		{
			name: "standard imported ServiceRecord",
			serviceRecords: []*connectivityv1alpha1.ServiceRecord{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test.some.domain-abcd1234",
						Namespace: "cross-cluster-connectivity",
						Labels: map[string]string{
							connectivityv1alpha1.ImportedLabel: "",
						},
						Annotations: map[string]string{
							connectivityv1alpha1.ServicePortAnnotation: "443",
						},
						UID: "1",
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
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-some-domain-cc9e50dd",
					Namespace: "cross-cluster-connectivity",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: connectivityv1alpha1.SchemeGroupVersion.String(),
							Kind:       "ServiceRecord",
							Name:       "test.some.domain-abcd1234",
							UID:        "1",
						},
					},
					Annotations: map[string]string{connectivityv1alpha1.FQDNAnnotation: "test.some.domain"},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:     443,
							Protocol: corev1.ProtocolTCP,
						},
					},
				},
			},
			endpoints: &corev1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-some-domain-cc9e50dd",
					Namespace: "cross-cluster-connectivity",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: connectivityv1alpha1.SchemeGroupVersion.String(),
							Kind:       "ServiceRecord",
							Name:       "test.some.domain-abcd1234",
							UID:        "1",
						},
					},
				},
				Subsets: []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{
								IP: "10.0.0.1",
							},
							{
								IP: "10.0.0.2",
							},
						},
						Ports: []corev1.EndpointPort{
							{
								Port: 443,
							},
						},
					},
				},
			},
		},
		{
			name: "standard imported ServiceRecord with VIP",
			serviceRecords: []*connectivityv1alpha1.ServiceRecord{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test.some.domain-abcd1234",
						Namespace: "cross-cluster-connectivity",
						Labels: map[string]string{
							connectivityv1alpha1.ImportedLabel: "",
						},
						Annotations: map[string]string{
							connectivityv1alpha1.ServicePortAnnotation: "443",
							connectivityv1alpha1.GlobalVIPAnnotation:   "1.2.3.4",
						},
						UID: "1",
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
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-some-domain-cc9e50dd",
					Namespace: "cross-cluster-connectivity",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: connectivityv1alpha1.SchemeGroupVersion.String(),
							Kind:       "ServiceRecord",
							Name:       "test.some.domain-abcd1234",
							UID:        "1",
						},
					},
					Annotations: map[string]string{connectivityv1alpha1.FQDNAnnotation: "test.some.domain"},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:     443,
							Protocol: corev1.ProtocolTCP,
						},
					},
				},
				Status: corev1.ServiceStatus{
					LoadBalancer: corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{
							{
								IP: "1.2.3.4",
							},
						},
					},
				},
			},
			endpoints: &corev1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-some-domain-cc9e50dd",
					Namespace: "cross-cluster-connectivity",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: connectivityv1alpha1.SchemeGroupVersion.String(),
							Kind:       "ServiceRecord",
							Name:       "test.some.domain-abcd1234",
							UID:        "1",
						},
					},
				},
				Subsets: []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{
								IP: "10.0.0.1",
							},
							{
								IP: "10.0.0.2",
							},
						},
						Ports: []corev1.EndpointPort{
							{
								Port: 443,
							},
						},
					},
				},
			},
		},
		{
			name: "standard imported ServiceRecord with different service port",
			serviceRecords: []*connectivityv1alpha1.ServiceRecord{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test.some.domain-abcd1234",
						Namespace: "cross-cluster-connectivity",
						Labels: map[string]string{
							connectivityv1alpha1.ImportedLabel: "",
						},
						Annotations: map[string]string{
							connectivityv1alpha1.ServicePortAnnotation: "8080",
							connectivityv1alpha1.GlobalVIPAnnotation:   "1.2.3.4",
						},
						UID: "1",
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
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-some-domain-cc9e50dd",
					Namespace: "cross-cluster-connectivity",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: connectivityv1alpha1.SchemeGroupVersion.String(),
							Kind:       "ServiceRecord",
							Name:       "test.some.domain-abcd1234",
							UID:        "1",
						},
					},
					Annotations: map[string]string{connectivityv1alpha1.FQDNAnnotation: "test.some.domain"},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:     8080,
							Protocol: corev1.ProtocolTCP,
						},
					},
				},
				Status: corev1.ServiceStatus{
					LoadBalancer: corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{
							{
								IP: "1.2.3.4",
							},
						},
					},
				},
			},
			endpoints: &corev1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-some-domain-cc9e50dd",
					Namespace: "cross-cluster-connectivity",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: connectivityv1alpha1.SchemeGroupVersion.String(),
							Kind:       "ServiceRecord",
							Name:       "test.some.domain-abcd1234",
							UID:        "1",
						},
					},
				},
				Subsets: []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{
								IP: "10.0.0.1",
							},
							{
								IP: "10.0.0.2",
							},
						},
						Ports: []corev1.EndpointPort{
							{
								Port: 443,
							},
						},
					},
				},
			},
		},
		{
			name: "multiple imported ServiceRecord with matching FQDN",
			serviceRecords: []*connectivityv1alpha1.ServiceRecord{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test.some.domain-abcd1234",
						Namespace: "cross-cluster-connectivity",
						Labels: map[string]string{
							connectivityv1alpha1.ImportedLabel: "",
						},
						Annotations: map[string]string{
							connectivityv1alpha1.ServicePortAnnotation: "443",
							connectivityv1alpha1.GlobalVIPAnnotation:   "1.2.3.4",
						},
						UID: "1",
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
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test.some.domain-bcde2345",
						Namespace: "cross-cluster-connectivity",
						Labels: map[string]string{
							connectivityv1alpha1.ImportedLabel: "",
						},
						Annotations: map[string]string{
							connectivityv1alpha1.ServicePortAnnotation: "443",
							connectivityv1alpha1.GlobalVIPAnnotation:   "1.2.3.4",
						},
						UID: "2",
					},
					Spec: connectivityv1alpha1.ServiceRecordSpec{
						FQDN: "test.some.domain",
						Endpoints: []connectivityv1alpha1.Endpoint{
							{
								Address: "10.0.0.3",
								Port:    443,
							},
							{
								Address: "10.0.0.4",
								Port:    443,
							},
						},
					},
				},
			},
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-some-domain-cc9e50dd",
					Namespace: "cross-cluster-connectivity",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: connectivityv1alpha1.SchemeGroupVersion.String(),
							Kind:       "ServiceRecord",
							Name:       "test.some.domain-abcd1234",
							UID:        "1",
						},
						{
							APIVersion: connectivityv1alpha1.SchemeGroupVersion.String(),
							Kind:       "ServiceRecord",
							Name:       "test.some.domain-bcde2345",
							UID:        "2",
						},
					},
					Annotations: map[string]string{connectivityv1alpha1.FQDNAnnotation: "test.some.domain"},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:     443,
							Protocol: corev1.ProtocolTCP,
						},
					},
				},
				Status: corev1.ServiceStatus{
					LoadBalancer: corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{
							{
								IP: "1.2.3.4",
							},
						},
					},
				},
			},
			endpoints: &corev1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-some-domain-cc9e50dd",
					Namespace: "cross-cluster-connectivity",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: connectivityv1alpha1.SchemeGroupVersion.String(),
							Kind:       "ServiceRecord",
							Name:       "test.some.domain-abcd1234",
							UID:        "1",
						},
						{
							APIVersion: connectivityv1alpha1.SchemeGroupVersion.String(),
							Kind:       "ServiceRecord",
							Name:       "test.some.domain-bcde2345",
							UID:        "2",
						},
					},
				},
				Subsets: []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{
								IP: "10.0.0.1",
							},
							{
								IP: "10.0.0.2",
							},
							{
								IP: "10.0.0.3",
							},
							{
								IP: "10.0.0.4",
							},
						},
						Ports: []corev1.EndpointPort{
							{
								Port: 443,
							},
						},
					},
				},
			},
		},
		{
			name: "multiple imported ServiceRecord but some have different FQDN",
			serviceRecords: []*connectivityv1alpha1.ServiceRecord{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test.some.domain-abcd1234",
						Namespace: "cross-cluster-connectivity",
						Labels: map[string]string{
							connectivityv1alpha1.ImportedLabel: "",
						},
						Annotations: map[string]string{
							connectivityv1alpha1.ServicePortAnnotation: "443",
							connectivityv1alpha1.GlobalVIPAnnotation:   "1.2.3.4",
						},
						UID: "1",
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
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test.some.domain-bcde2345",
						Namespace: "cross-cluster-connectivity",
						Labels: map[string]string{
							connectivityv1alpha1.ImportedLabel: "",
						},
						Annotations: map[string]string{
							connectivityv1alpha1.ServicePortAnnotation: "443",
							connectivityv1alpha1.GlobalVIPAnnotation:   "1.2.3.4",
						},
						UID: "2",
					},
					Spec: connectivityv1alpha1.ServiceRecordSpec{
						FQDN: "test.some.domain",
						Endpoints: []connectivityv1alpha1.Endpoint{
							{
								Address: "10.0.0.3",
								Port:    443,
							},
							{
								Address: "10.0.0.4",
								Port:    443,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foobar.some.domain-bcde2345",
						Namespace: "cross-cluster-connectivity",
						Labels: map[string]string{
							connectivityv1alpha1.ImportedLabel: "",
						},
						Annotations: map[string]string{
							connectivityv1alpha1.ServicePortAnnotation: "443",
							connectivityv1alpha1.GlobalVIPAnnotation:   "1.2.3.4",
						},
						UID: "3",
					},
					Spec: connectivityv1alpha1.ServiceRecordSpec{
						FQDN: "foobar.some.domain",
						Endpoints: []connectivityv1alpha1.Endpoint{
							{
								Address: "10.0.0.5",
								Port:    443,
							},
							{
								Address: "10.0.0.6",
								Port:    443,
							},
						},
					},
				},
			},
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-some-domain-cc9e50dd",
					Namespace: "cross-cluster-connectivity",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: connectivityv1alpha1.SchemeGroupVersion.String(),
							Kind:       "ServiceRecord",
							Name:       "test.some.domain-abcd1234",
							UID:        "1",
						},
						{
							APIVersion: connectivityv1alpha1.SchemeGroupVersion.String(),
							Kind:       "ServiceRecord",
							Name:       "test.some.domain-bcde2345",
							UID:        "2",
						},
					},
					Annotations: map[string]string{connectivityv1alpha1.FQDNAnnotation: "test.some.domain"},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:     443,
							Protocol: corev1.ProtocolTCP,
						},
					},
				},
				Status: corev1.ServiceStatus{
					LoadBalancer: corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{
							{
								IP: "1.2.3.4",
							},
						},
					},
				},
			},
			endpoints: &corev1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-some-domain-cc9e50dd",
					Namespace: "cross-cluster-connectivity",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: connectivityv1alpha1.SchemeGroupVersion.String(),
							Kind:       "ServiceRecord",
							Name:       "test.some.domain-abcd1234",
							UID:        "1",
						},
						{
							APIVersion: connectivityv1alpha1.SchemeGroupVersion.String(),
							Kind:       "ServiceRecord",
							Name:       "test.some.domain-bcde2345",
							UID:        "2",
						},
					},
				},
				Subsets: []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{
								IP: "10.0.0.1",
							},
							{
								IP: "10.0.0.2",
							},
							{
								IP: "10.0.0.3",
							},
							{
								IP: "10.0.0.4",
							},
						},
						Ports: []corev1.EndpointPort{
							{
								Port: 443,
							},
						},
					},
				},
			},
		},
		{
			name: "multiple ServiceRecord with matching FQDN but one is exported",
			serviceRecords: []*connectivityv1alpha1.ServiceRecord{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test.some.domain-abcd1234",
						Namespace: "cross-cluster-connectivity",
						Labels: map[string]string{
							connectivityv1alpha1.ImportedLabel: "",
						},
						Annotations: map[string]string{
							connectivityv1alpha1.ServicePortAnnotation: "443",
							connectivityv1alpha1.GlobalVIPAnnotation:   "1.2.3.4",
						},
						UID: "1",
					},
					Spec: connectivityv1alpha1.ServiceRecordSpec{
						FQDN: "test.some.domain",
						Endpoints: []connectivityv1alpha1.Endpoint{
							{
								Address: "10.0.0.1",
								Port:    443,
							},
							{
								Address: "10.0.0.3",
								Port:    443,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test.some.domain-bcde2345",
						Namespace: "cross-cluster-connectivity",
						Labels: map[string]string{
							connectivityv1alpha1.ImportedLabel: "",
						},
						Annotations: map[string]string{
							connectivityv1alpha1.ServicePortAnnotation: "443",
							connectivityv1alpha1.GlobalVIPAnnotation:   "1.2.3.4",
						},
						UID: "2",
					},
					Spec: connectivityv1alpha1.ServiceRecordSpec{
						FQDN: "test.some.domain",
						Endpoints: []connectivityv1alpha1.Endpoint{
							{
								Address: "10.0.0.2",
								Port:    443,
							},
							{
								Address: "10.0.0.4",
								Port:    443,
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test.some.domain.shared-services-cls-3",
						Namespace: "cross-cluster-connectivity",
						Labels: map[string]string{
							connectivityv1alpha1.ExportLabel: "",
						},
						Annotations: map[string]string{
							connectivityv1alpha1.ServicePortAnnotation: "443",
							connectivityv1alpha1.GlobalVIPAnnotation:   "1.2.3.4",
						},
						UID: "3",
					},
					Spec: connectivityv1alpha1.ServiceRecordSpec{
						FQDN: "test.some.domain",
						Endpoints: []connectivityv1alpha1.Endpoint{
							{
								Address: "10.0.0.5",
								Port:    443,
							},
							{
								Address: "10.0.0.6",
								Port:    443,
							},
						},
					},
				},
			},
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-some-domain-cc9e50dd",
					Namespace: "cross-cluster-connectivity",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: connectivityv1alpha1.SchemeGroupVersion.String(),
							Kind:       "ServiceRecord",
							Name:       "test.some.domain-abcd1234",
							UID:        "1",
						},
						{
							APIVersion: connectivityv1alpha1.SchemeGroupVersion.String(),
							Kind:       "ServiceRecord",
							Name:       "test.some.domain-bcde2345",
							UID:        "2",
						},
					},
					Annotations: map[string]string{connectivityv1alpha1.FQDNAnnotation: "test.some.domain"},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:     443,
							Protocol: corev1.ProtocolTCP,
						},
					},
				},
				Status: corev1.ServiceStatus{
					LoadBalancer: corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{
							{
								IP: "1.2.3.4",
							},
						},
					},
				},
			},
			endpoints: &corev1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-some-domain-cc9e50dd",
					Namespace: "cross-cluster-connectivity",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: connectivityv1alpha1.SchemeGroupVersion.String(),
							Kind:       "ServiceRecord",
							Name:       "test.some.domain-abcd1234",
							UID:        "1",
						},
						{
							APIVersion: connectivityv1alpha1.SchemeGroupVersion.String(),
							Kind:       "ServiceRecord",
							Name:       "test.some.domain-bcde2345",
							UID:        "2",
						},
					},
				},
				Subsets: []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{
								IP: "10.0.0.1",
							},
							{
								IP: "10.0.0.2",
							},
							{
								IP: "10.0.0.3",
							},
							{
								IP: "10.0.0.4",
							},
						},
						Ports: []corev1.EndpointPort{
							{
								Port: 443,
							},
						},
					},
				},
			},
		},
		{
			name: "single ServiceRecord with 63+ character fqdn",
			serviceRecords: []*connectivityv1alpha1.ServiceRecord{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test.some.domain-abcd1234",
						Namespace: "cross-cluster-connectivity",
						Labels: map[string]string{
							connectivityv1alpha1.ImportedLabel: "",
						},
						Annotations: map[string]string{
							connectivityv1alpha1.ServicePortAnnotation: "443",
							connectivityv1alpha1.GlobalVIPAnnotation:   "1.2.3.4",
						},
						UID: "1",
					},
					Spec: connectivityv1alpha1.ServiceRecordSpec{
						FQDN: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.some.domain",
						Endpoints: []connectivityv1alpha1.Endpoint{
							{
								Address: "10.0.0.1",
								Port:    443,
							},
							{
								Address: "10.0.0.3",
								Port:    443,
							},
						},
					},
				},
			},
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-c2657973",
					Namespace: "cross-cluster-connectivity",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: connectivityv1alpha1.SchemeGroupVersion.String(),
							Kind:       "ServiceRecord",
							Name:       "test.some.domain-abcd1234",
							UID:        "1",
						},
					},
					Annotations: map[string]string{
						connectivityv1alpha1.FQDNAnnotation: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.some.domain",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Port:     443,
							Protocol: corev1.ProtocolTCP,
						},
					},
				},
				Status: corev1.ServiceStatus{
					LoadBalancer: corev1.LoadBalancerStatus{
						Ingress: []corev1.LoadBalancerIngress{
							{
								IP: "1.2.3.4",
							},
						},
					},
				},
			},
			endpoints: &corev1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-c2657973",
					Namespace: "cross-cluster-connectivity",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: connectivityv1alpha1.SchemeGroupVersion.String(),
							Kind:       "ServiceRecord",
							Name:       "test.some.domain-abcd1234",
							UID:        "1",
						},
					},
				},
				Subsets: []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{
								IP: "10.0.0.1",
							},
							{
								IP: "10.0.0.3",
							},
						},
						Ports: []corev1.EndpointPort{
							{
								Port: 443,
							},
						},
					},
				},
			},
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			_, kubeClientset := setupTestcase(t, testcase.serviceRecords)

			if err := wait.Poll(100*time.Millisecond, 5*time.Second, checkService(t, kubeClientset, testcase.service)); err != nil {
				t.Errorf("Service did not match: %v", err)
			}

			if err := wait.Poll(100*time.Millisecond, 5*time.Second, checkEndpoints(t, kubeClientset, testcase.endpoints)); err != nil {
				t.Errorf("Endpoints did not match: %v", err)
			}
		})
	}
}

func Test_Update(t *testing.T) {
	testServiceRecords := []*connectivityv1alpha1.ServiceRecord{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test.some.domain-abcd1234",
				Namespace: "cross-cluster-connectivity",
				Labels: map[string]string{
					connectivityv1alpha1.ImportedLabel: "",
				},
				Annotations: map[string]string{
					connectivityv1alpha1.ServicePortAnnotation: "443",
					connectivityv1alpha1.GlobalVIPAnnotation:   "1.2.3.4",
				},
				UID: "1",
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
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test.some.domain-bcde2345",
				Namespace: "cross-cluster-connectivity",
				Labels: map[string]string{
					connectivityv1alpha1.ImportedLabel: "",
				},
				Annotations: map[string]string{
					connectivityv1alpha1.ServicePortAnnotation: "443",
					connectivityv1alpha1.GlobalVIPAnnotation:   "1.2.3.4",
				},
				UID: "2",
			},
			Spec: connectivityv1alpha1.ServiceRecordSpec{
				FQDN: "test.some.domain",
				Endpoints: []connectivityv1alpha1.Endpoint{
					{
						Address: "10.0.0.3",
						Port:    443,
					},
					{
						Address: "10.0.0.4",
						Port:    443,
					},
				},
			},
		},
	}

	testService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-some-domain-cc9e50dd",
			Namespace: "cross-cluster-connectivity",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: connectivityv1alpha1.SchemeGroupVersion.String(),
					Kind:       "ServiceRecord",
					Name:       "test.some.domain-abcd1234",
					UID:        "1",
				},
				{
					APIVersion: connectivityv1alpha1.SchemeGroupVersion.String(),
					Kind:       "ServiceRecord",
					Name:       "test.some.domain-bcde2345",
					UID:        "2",
				},
			},
			Annotations: map[string]string{connectivityv1alpha1.FQDNAnnotation: "test.some.domain"},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port:     443,
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{
						IP: "1.2.3.4",
					},
				},
			},
		},
	}

	expectedService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-some-domain-cc9e50dd",
			Namespace: "cross-cluster-connectivity",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: connectivityv1alpha1.SchemeGroupVersion.String(),
					Kind:       "ServiceRecord",
					Name:       "test.some.domain-bcde2345",
					UID:        "2",
				},
			},
			Annotations: map[string]string{connectivityv1alpha1.FQDNAnnotation: "test.some.domain"},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port:     443,
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{
						IP: "1.2.3.4",
					},
				},
			},
		},
	}

	testEndpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-some-domain-cc9e50dd",
			Namespace: "cross-cluster-connectivity",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: connectivityv1alpha1.SchemeGroupVersion.String(),
					Kind:       "ServiceRecord",
					Name:       "test.some.domain-abcd1234",
					UID:        "1",
				},
				{
					APIVersion: connectivityv1alpha1.SchemeGroupVersion.String(),
					Kind:       "ServiceRecord",
					Name:       "test.some.domain-bcde2345",
					UID:        "2",
				},
			},
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{
						IP: "10.0.0.1",
					},
					{
						IP: "10.0.0.2",
					},
					{
						IP: "10.0.0.3",
					},
					{
						IP: "10.0.0.4",
					},
				},
				Ports: []corev1.EndpointPort{
					{
						Port: 443,
					},
				},
			},
		},
	}

	expectedEndpoints := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-some-domain-cc9e50dd",
			Namespace: "cross-cluster-connectivity",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: connectivityv1alpha1.SchemeGroupVersion.String(),
					Kind:       "ServiceRecord",
					Name:       "test.some.domain-bcde2345",
					UID:        "2",
				},
			},
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{
						IP: "10.0.0.3",
					},
					{
						IP: "10.0.0.4",
					},
				},
				Ports: []corev1.EndpointPort{
					{
						Port: 443,
					},
				},
			},
		},
	}

	t.Run("deletion of the one of multiple service records matching an FQDN", func(t *testing.T) {
		connClientset, kubeClientset := setupTestcase(t, testServiceRecords)

		if err := wait.Poll(100*time.Millisecond, 5*time.Second, checkService(t, kubeClientset, testService)); err != nil {
			t.Errorf("Service did not match: %v", err)
		}

		if err := wait.Poll(100*time.Millisecond, 5*time.Second, checkEndpoints(t, kubeClientset, testEndpoints)); err != nil {
			t.Errorf("Endpoints did not match: %v", err)
		}

		if err := connClientset.ConnectivityV1alpha1().ServiceRecords(testServiceRecords[0].Namespace).
			Delete(testServiceRecords[0].Name, &metav1.DeleteOptions{}); err != nil {
			t.Fatalf("error deleting ServiceRecord resource: %v", err)
		}

		if err := wait.Poll(100*time.Millisecond, 5*time.Second, checkService(t, kubeClientset, expectedService)); err != nil {
			t.Errorf("Service did not match: %v", err)
		}

		if err := wait.Poll(100*time.Millisecond, 5*time.Second, checkEndpoints(t, kubeClientset, expectedEndpoints)); err != nil {
			t.Errorf("Endpoints did not match: %v", err)
		}
	})
}

func setupTestcase(t *testing.T, testCaseServiceRecords []*connectivityv1alpha1.ServiceRecord) (*fake.Clientset, *kubefake.Clientset) {
	var objs []runtime.Object
	for _, sr := range testCaseServiceRecords {
		objs = append(objs, sr)
	}

	connClientset := fake.NewSimpleClientset(objs...)
	kubeClientset := kubefake.NewSimpleClientset()

	connectivityInformerFactory := connectivityinformers.NewSharedInformerFactory(connClientset, 30*time.Second)
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClientset, 30*time.Second)
	serviceRecordInformer := connectivityInformerFactory.Connectivity().V1alpha1().ServiceRecords()
	serviceInformer := kubeInformerFactory.Core().V1().Services()
	endpointsInformer := kubeInformerFactory.Core().V1().Endpoints()

	serviceBindingController := NewServiceBindingController(kubeClientset, connClientset, serviceRecordInformer, serviceInformer, endpointsInformer)

	connectivityInformerFactory.Start(nil)
	kubeInformerFactory.Start(nil)

	connectivityInformerFactory.WaitForCacheSync(nil)
	kubeInformerFactory.WaitForCacheSync(nil)

	go serviceBindingController.Run(1, nil)

	return connClientset, kubeClientset
}

func checkService(t *testing.T, kubeClientset *kubefake.Clientset, testCaseService *corev1.Service) func() (bool, error) {
	return func() (bool, error) {
		service, err := kubeClientset.CoreV1().Services(testCaseService.Namespace).Get(
			testCaseService.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if !reflect.DeepEqual(service, testCaseService) {
			t.Logf("actual Service: %v", format.Object(service, 4))
			t.Logf("expected Service: %v", format.Object(testCaseService, 4))
			return true, errors.New("Services were not equal")
		}

		return true, nil
	}
}

func checkEndpoints(t *testing.T, kubeClientset *kubefake.Clientset, testCaseEndpoints *corev1.Endpoints) func() (bool, error) {
	return func() (bool, error) {
		endpoints, err := kubeClientset.CoreV1().Endpoints(testCaseEndpoints.Namespace).Get(
			testCaseEndpoints.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if !reflect.DeepEqual(endpoints, testCaseEndpoints) {
			t.Logf("actual Endpoints: %v", format.Object(endpoints, 4))
			t.Logf("expected Endpoints: %v", format.Object(testCaseEndpoints, 4))
			return true, errors.New("Endpoints were not equal")
		}

		return true, nil
	}
}
