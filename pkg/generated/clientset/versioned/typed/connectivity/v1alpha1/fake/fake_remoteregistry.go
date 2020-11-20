// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeRemoteRegistries implements RemoteRegistryInterface
type FakeRemoteRegistries struct {
	Fake *FakeConnectivityV1alpha1
	ns   string
}

var remoteregistriesResource = schema.GroupVersionResource{Group: "connectivity.tanzu.vmware.com", Version: "v1alpha1", Resource: "remoteregistries"}

var remoteregistriesKind = schema.GroupVersionKind{Group: "connectivity.tanzu.vmware.com", Version: "v1alpha1", Kind: "RemoteRegistry"}

// Get takes name of the remoteRegistry, and returns the corresponding remoteRegistry object, and an error if there is any.
func (c *FakeRemoteRegistries) Get(name string, options v1.GetOptions) (result *v1alpha1.RemoteRegistry, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(remoteregistriesResource, c.ns, name), &v1alpha1.RemoteRegistry{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.RemoteRegistry), err
}

// List takes label and field selectors, and returns the list of RemoteRegistries that match those selectors.
func (c *FakeRemoteRegistries) List(opts v1.ListOptions) (result *v1alpha1.RemoteRegistryList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(remoteregistriesResource, remoteregistriesKind, c.ns, opts), &v1alpha1.RemoteRegistryList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.RemoteRegistryList{ListMeta: obj.(*v1alpha1.RemoteRegistryList).ListMeta}
	for _, item := range obj.(*v1alpha1.RemoteRegistryList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested remoteRegistries.
func (c *FakeRemoteRegistries) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(remoteregistriesResource, c.ns, opts))

}

// Create takes the representation of a remoteRegistry and creates it.  Returns the server's representation of the remoteRegistry, and an error, if there is any.
func (c *FakeRemoteRegistries) Create(remoteRegistry *v1alpha1.RemoteRegistry) (result *v1alpha1.RemoteRegistry, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(remoteregistriesResource, c.ns, remoteRegistry), &v1alpha1.RemoteRegistry{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.RemoteRegistry), err
}

// Update takes the representation of a remoteRegistry and updates it. Returns the server's representation of the remoteRegistry, and an error, if there is any.
func (c *FakeRemoteRegistries) Update(remoteRegistry *v1alpha1.RemoteRegistry) (result *v1alpha1.RemoteRegistry, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(remoteregistriesResource, c.ns, remoteRegistry), &v1alpha1.RemoteRegistry{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.RemoteRegistry), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeRemoteRegistries) UpdateStatus(remoteRegistry *v1alpha1.RemoteRegistry) (*v1alpha1.RemoteRegistry, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(remoteregistriesResource, "status", c.ns, remoteRegistry), &v1alpha1.RemoteRegistry{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.RemoteRegistry), err
}

// Delete takes name of the remoteRegistry and deletes it. Returns an error if one occurs.
func (c *FakeRemoteRegistries) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(remoteregistriesResource, c.ns, name), &v1alpha1.RemoteRegistry{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeRemoteRegistries) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(remoteregistriesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.RemoteRegistryList{})
	return err
}

// Patch applies the patch and returns the patched remoteRegistry.
func (c *FakeRemoteRegistries) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.RemoteRegistry, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(remoteregistriesResource, c.ns, name, pt, data, subresources...), &v1alpha1.RemoteRegistry{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.RemoteRegistry), err
}
