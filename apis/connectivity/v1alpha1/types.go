// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConnectivityLabelPrefix = "connectivity.tanzu.vmware.com"
	ExportLabel             = "connectivity.tanzu.vmware.com/export"
	ImportedLabel           = "connectivity.tanzu.vmware.com/imported"
	GlobalVIPAnnotation     = "connectivity.tanzu.vmware.com/vip"
	FQDNAnnotation          = "connectivity.tanzu.vmware.com/fqdn"
	DNSHostnameAnnotation   = "connectivity.tanzu.vmware.com/dns-hostname"
	// ServicePortAnnotation is an annotation used to specify the client-side Service
	// port binding for a ServiceRecord. This annotation is temporary until hamlet
	// v1alpha2 supports port mappings.
	ServicePortAnnotation = "connectivity.tanzu.vmware.com/service-port"

	ConnectivityRemoteRegistryLabel   = "connectivity.tanzu.vmware.com/remote-registry"
	ConnectivityClusterNameLabel      = "connectivity.tanzu.vmware.com/cluster-name"
	ConnectivityClusterNamespaceLabel = "connectivity.tanzu.vmware.com/cluster-namespace"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceRecord is
// +k8s:openapi-gen=true
// +kubebuilder:resource:scope=Namespaced,path=servicerecords,singular=servicerecord
// +kubebuilder:subresource:status
type ServiceRecord struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ServiceRecordSpec `json:"spec"`
}

// ServiceRecordSpec ...
type ServiceRecordSpec struct {
	// FQDN...
	FQDN string `json:"fqdn"`
	// Endpoints...
	Endpoints []Endpoint `json:"endpoints"`
}

// Endpoint ...
type Endpoint struct {
	// Address ...
	Address string `json:"address"`
	// Port ...
	Port uint32 `json:"port"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ServiceRecordList is a list of ServiceRecord resources
type ServiceRecordList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ServiceRecord `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RemoteRegistry ...
// +k8s:openapi-gen=true
// +kubebuilder:resource:scope=Namespaced,path=remoteregistries,singular=remoteregistry
// +kubebuilder:subresource:status
type RemoteRegistry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec RemoteRegistrySpec `json:"spec"`

	// +optional
	Status RemoteRegistryStatus `json:"status"`
}

// RemoteRegistrySpec...
type RemoteRegistrySpec struct {
	Address   string            `json:"address"`
	TLSConfig RegistryTLSConfig `json:"tlsConfig"`

	// +optional
	AllowedDomains []string `json:"allowedDomains,omitempty"`
}

// RemoteRegistryStatus...
type RemoteRegistryStatus struct {
	// List of status conditions to indicate the status of RemoteRegistry.
	// +optional
	Conditions []RemoteRegistryCondition `json:"conditions,omitempty"`

	// ObservedGeneration is the latest generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// RemoteRegistryConditionType is a valid value for RemoteRegistryCondition.Type
type RemoteRegistryConditionType string

// RemoteRegistryCondition defines an observation of a RemoteRegistry resource state
type RemoteRegistryCondition struct {
	// Type of the condition, known values are ('Valid').
	// +required
	Type RemoteRegistryConditionType `json:"type"`

	// Status of the condition, one of ('True', 'False', 'Unknown').
	// +required
	Status corev1.ConditionStatus `json:"status"`

	// LastTransitionTime describes the last time the condition transitioned from
	// one status to another.
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`

	// Reason is a brief machine readable explanation for the condition's last
	// transition.
	// +optional
	Reason string `json:"reason,omitempty"`

	// Message is a human readable description of the details of the last
	// transition, complementing reason.
	// +optional
	Message string `json:"message,omitempty"`
}

// RemoteRegistryConditionValid indicates that this RemoteRegistry resource
// is ready to be read for connection details about a remote registry server.
const RemoteRegistryConditionValid RemoteRegistryConditionType = "Valid"

// RegistryTLSConfig...
type RegistryTLSConfig struct {
	ServerCA   []byte `json:"serverCA"`
	ServerName string `json:"serverName"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RemoteRegistryList is a list of RemoteRegistry resources
type RemoteRegistryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []RemoteRegistry `json:"items"`
}
