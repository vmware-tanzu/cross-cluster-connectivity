// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GatewayDNSSpec defines the desired state of GatewayDNS
type GatewayDNSSpec struct {
	// Important: Run "make generate" to regenerate code after modifying this file

	// clusterSelector is a label selector that matches clusters that shall have
	// their gateway endpoint information propagated.
	ClusterSelector metav1.LabelSelector `json:"clusterSelector,omitempty"`

	// service is the namespace/name of the service to be propagated.
	Service string `json:"service,omitempty"`

	// resolutionType indicates the method the controller will use to discover
	// the ip of the service.
	ResolutionType GatewayResolutionType `json:"resolutionType,omitempty"`
}

type GatewayResolutionType string

const (
	ResolutionTypeLoadBalancer GatewayResolutionType = "loadBalancer"
)

// GatewayDNSStatus defines the observed state of GatewayDNS
type GatewayDNSStatus struct {
	// Important: Run "make generate" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true

// GatewayDNS is the Schema for the gatewaydns API
// +kubebuilder:printcolumn:name="Resolution Type",type=string,JSONPath=`.spec.resolutionType`
// +kubebuilder:printcolumn:name="Service",type=string,JSONPath=`.spec.service`
// +kubebuilder:printcolumn:name="Cluster Selector",type=string,JSONPath=`.spec.clusterSelector`
type GatewayDNS struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GatewayDNSSpec   `json:"spec,omitempty"`
	Status GatewayDNSStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GatewayDNSList contains a list of GatewayDNS
type GatewayDNSList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GatewayDNS `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GatewayDNS{}, &GatewayDNSList{})
}
