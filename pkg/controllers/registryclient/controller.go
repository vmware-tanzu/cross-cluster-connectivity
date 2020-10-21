// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package registryclient

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	connectivityclientset "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/clientset/versioned"
	connectivityinformers "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/informers/externalversions/connectivity/v1alpha1"
	connectivitylisters "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/listers/connectivity/v1alpha1"
)

type RegistryClientController struct {
	sync.Mutex

	clients map[string]*registryClient

	connClientSet  connectivityclientset.Interface
	registryLister connectivitylisters.RemoteRegistryLister

	serviceRecordLister connectivitylisters.ServiceRecordLister

	workqueue workqueue.RateLimitingInterface
}

func NewRegistryClientController(connClientSet connectivityclientset.Interface,
	remoteRegistryInformer connectivityinformers.RemoteRegistryInformer,
	serviceRecordInformer connectivityinformers.ServiceRecordInformer) *RegistryClientController {
	controller := &RegistryClientController{
		clients:             map[string]*registryClient{},
		connClientSet:       connClientSet,
		registryLister:      remoteRegistryInformer.Lister(),
		serviceRecordLister: serviceRecordInformer.Lister(),
		workqueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "RemoteRegistries"),
	}

	remoteRegistryInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueRemoteRegistry,
		UpdateFunc: func(oldObj, newObj interface{}) {
			controller.enqueueRemoteRegistry(newObj)
		},
		DeleteFunc: controller.handleDelete,
	})

	return controller
}

func (r *RegistryClientController) Run(threads int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer r.workqueue.ShutDown()

	log.Info("Starting RegistryClient controller")
	for i := 0; i < threads; i++ {
		go wait.Until(r.runWorker, time.Second, stopCh)
	}

	log.Info("Started workers")
	<-stopCh
	log.Info("Shutting down workers")

	return nil
}

func (r *RegistryClientController) runWorker() {
	for r.processNextWorkItem() {
	}
}

func (r *RegistryClientController) processNextWorkItem() bool {
	obj, shutdown := r.workqueue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer r.workqueue.Done(obj)

		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			r.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		if err := r.syncHandler(key); err != nil {
			r.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}

		r.workqueue.Forget(obj)
		log.Infof("Successfully synced RemoteRegistry '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (r *RegistryClientController) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// fetch the latest remote registry from cache
	remoteRegistry, err := r.registryLister.RemoteRegistries(namespace).Get(name)
	if err != nil {
		return err
	}

	r.Lock()
	defer r.Unlock()

	// check if an existing remote registry watcher has started
	cachedRegistryClient, exists := r.clients[key]
	if exists {
		// RemoteRegistry hasn't changed, do nothing
		if registriesEqual(remoteRegistry, cachedRegistryClient.remoteRegistry) {
			return nil
		}

		// RemoteRegistry has changed, redial to restart connection
		cachedRegistryClient.redial(remoteRegistry)
	} else {
		// client for RemoteRegistry doesn't exist, create a remote registry client
		registryClient := newRegistryClient(remoteRegistry, r.connClientSet,
			r.serviceRecordLister)

		log.WithField("remoteregistry", fmt.Sprintf("%s/%s",
			remoteRegistry.Namespace, remoteRegistry.Name)).
			Info("starting new registry client")
		go registryClient.run()
		r.clients[key] = registryClient
	}

	return nil
}

func (r *RegistryClientController) enqueueRemoteRegistry(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	r.workqueue.Add(key)
}

func (r *RegistryClientController) handleDelete(obj interface{}) {
	var remoteRegistry *connectivityv1alpha1.RemoteRegistry
	var ok bool
	if remoteRegistry, ok = obj.(*connectivityv1alpha1.RemoteRegistry); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		remoteRegistry, ok = tombstone.Obj.(*connectivityv1alpha1.RemoteRegistry)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
	}

	clientCacheKey := remoteRegistry.Namespace + "/" + remoteRegistry.Name

	r.Lock()
	defer r.Unlock()

	registryClient, exists := r.clients[clientCacheKey]
	if !exists {
		// client was already terminated
		return
	}

	log.Infof("stopping remote registry %s", clientCacheKey)

	// stop the registry client and delete it from the client cache
	registryClient.stop()
	delete(r.clients, clientCacheKey)
}

func registriesEqual(r1, r2 *connectivityv1alpha1.RemoteRegistry) bool {
	return reflect.DeepEqual(r1.Spec, r2.Spec)
}
