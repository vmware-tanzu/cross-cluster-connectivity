// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConnectivityLabelPrefix = "connectivity.tanzu.vmware.com"
	ExportLabel             = "connectivity.tanzu.vmware.com/export"
	ImportedLabel           = "connectivity.tanzu.vmware.com/imported"
	GlobalVIPAnnotation     = "connectivity.tanzu.vmware.com/vip"
	FQDNAnnotation          = "connectivity.tanzu.vmware.com/fqdn"
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
}

// RemoteRegistrySpec...
type RemoteRegistrySpec struct {
	Address   string            `json:"address"`
	TLSConfig RegistryTLSConfig `json:"tlsConfig"`

	AllowedDomains []string `json:"allowedDomains"`
}

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
