// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// RemoteRegistryLister helps list RemoteRegistries.
// All objects returned here must be treated as read-only.
type RemoteRegistryLister interface {
	// List lists all RemoteRegistries in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.RemoteRegistry, err error)
	// RemoteRegistries returns an object that can list and get RemoteRegistries.
	RemoteRegistries(namespace string) RemoteRegistryNamespaceLister
	RemoteRegistryListerExpansion
}

// remoteRegistryLister implements the RemoteRegistryLister interface.
type remoteRegistryLister struct {
	indexer cache.Indexer
}

// NewRemoteRegistryLister returns a new RemoteRegistryLister.
func NewRemoteRegistryLister(indexer cache.Indexer) RemoteRegistryLister {
	return &remoteRegistryLister{indexer: indexer}
}

// List lists all RemoteRegistries in the indexer.
func (s *remoteRegistryLister) List(selector labels.Selector) (ret []*v1alpha1.RemoteRegistry, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.RemoteRegistry))
	})
	return ret, err
}

// RemoteRegistries returns an object that can list and get RemoteRegistries.
func (s *remoteRegistryLister) RemoteRegistries(namespace string) RemoteRegistryNamespaceLister {
	return remoteRegistryNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// RemoteRegistryNamespaceLister helps list and get RemoteRegistries.
// All objects returned here must be treated as read-only.
type RemoteRegistryNamespaceLister interface {
	// List lists all RemoteRegistries in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.RemoteRegistry, err error)
	// Get retrieves the RemoteRegistry from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.RemoteRegistry, error)
	RemoteRegistryNamespaceListerExpansion
}

// remoteRegistryNamespaceLister implements the RemoteRegistryNamespaceLister
// interface.
type remoteRegistryNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all RemoteRegistries in the indexer for a given namespace.
func (s remoteRegistryNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.RemoteRegistry, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.RemoteRegistry))
	})
	return ret, err
}

// Get retrieves the RemoteRegistry from the indexer for a given namespace and name.
func (s remoteRegistryNamespaceLister) Get(name string) (*v1alpha1.RemoteRegistry, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("remoteregistry"), name)
	}
	return obj.(*v1alpha1.RemoteRegistry), nil
}
