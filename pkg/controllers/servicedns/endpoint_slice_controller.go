// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package servicedns

import (
	"fmt"
	"net"
	"time"

	log "github.com/sirupsen/logrus"

	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	discoveryv1beta1 "k8s.io/api/discovery/v1beta1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	informerdiscoveryv1beta1 "k8s.io/client-go/informers/discovery/v1beta1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type EndpointSliceDNSController struct {
	scheme *runtime.Scheme

	endpointSliceInformer informerdiscoveryv1beta1.EndpointSliceInformer
	dnsCache              *DNSCache

	workqueue workqueue.RateLimitingInterface
}

func NewEndpointSliceDNSController(endpointSliceInformer informerdiscoveryv1beta1.EndpointSliceInformer, dnsCache *DNSCache) *EndpointSliceDNSController {
	controller := &EndpointSliceDNSController{
		scheme:                runtime.NewScheme(),
		endpointSliceInformer: endpointSliceInformer,
		dnsCache:              dnsCache,
		workqueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "EndpointSlice"),
	}

	endpointSliceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueue,
		UpdateFunc: func(oldObj, newObj interface{}) {
			controller.enqueue(newObj)
		},
		DeleteFunc: controller.handleDelete,
	})

	return controller
}

func (s *EndpointSliceDNSController) Run(threads int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer s.workqueue.ShutDown()

	log.Info("Starting EndpointSliceDNS controller")
	for i := 0; i < threads; i++ {
		go wait.Until(s.runWorker, time.Second, stopCh)
	}

	log.Info("Started workers")
	<-stopCh
	log.Info("Shutting down workers")

	return nil
}

func (s *EndpointSliceDNSController) runWorker() {
	for s.processNextWorkItem() {
	}
}

func (s *EndpointSliceDNSController) processNextWorkItem() bool {
	obj, shutdown := s.workqueue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer s.workqueue.Done(obj)

		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			s.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		if err := s.sync(key); err != nil {
			s.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}

		s.workqueue.Forget(obj)
		log.Infof("Successfully synced EndpointSlice '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (s *EndpointSliceDNSController) sync(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	endpointSlice, err := s.endpointSliceInformer.Lister().EndpointSlices(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			s.dnsCache.DeleteByEndpointSliceKey(key)

			return nil
		}

		return err
	}

	domainName, ok := endpointSlice.Annotations[connectivityv1alpha1.DNSHostnameAnnotation]
	if !ok {
		return nil
	}

	if endpointSlice.AddressType != discoveryv1beta1.AddressTypeIPv4 {
		log.Warnf("Skipping EndpointSlice '%s' since it is not an IPv4 EndpointSlice", key)
		return nil
	}

	ips := []net.IP{}
	for _, endpoint := range endpointSlice.Endpoints {
		for _, address := range endpoint.Addresses {
			ip := net.ParseIP(address)
			if ip == nil {
				utilruntime.HandleError(fmt.Errorf("invalid ip: %s", ip))
				return nil
			}

			ips = append(ips, ip)
		}
	}

	s.dnsCache.Upsert(DNSCacheEntry{
		EndpointSliceKey: key,
		FQDN:             domainName,
		IPs:              ips,
	})

	return nil
}

func (s *EndpointSliceDNSController) enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	s.workqueue.Add(key)
}

func (s *EndpointSliceDNSController) handleDelete(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
	}

	s.enqueue(object)
}
