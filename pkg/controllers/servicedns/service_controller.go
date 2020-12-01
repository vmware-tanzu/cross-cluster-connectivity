// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package servicedns

import (
	"fmt"
	"net"
	"time"

	log "github.com/sirupsen/logrus"

	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	informercorev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type ServiceDNSController struct {
	scheme *runtime.Scheme

	serviceInformer informercorev1.ServiceInformer
	dnsCache        *DNSCache

	workqueue workqueue.RateLimitingInterface
}

func NewServiceDNSController(serviceInformer informercorev1.ServiceInformer, dnsCache *DNSCache) *ServiceDNSController {
	controller := &ServiceDNSController{
		scheme:          runtime.NewScheme(),
		serviceInformer: serviceInformer,
		dnsCache:        dnsCache,
		workqueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Services"),
	}

	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueue,
		UpdateFunc: func(oldObj, newObj interface{}) {
			controller.enqueue(newObj)
		},
		DeleteFunc: controller.handleDelete,
	})

	return controller
}

func (s *ServiceDNSController) Run(threads int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer s.workqueue.ShutDown()

	log.Info("Starting ServiceDNS controller")
	for i := 0; i < threads; i++ {
		go wait.Until(s.runWorker, time.Second, stopCh)
	}

	log.Info("Started workers")
	<-stopCh
	log.Info("Shutting down workers")

	return nil
}

func (s *ServiceDNSController) runWorker() {
	for s.processNextWorkItem() {
	}
}

func (s *ServiceDNSController) processNextWorkItem() bool {
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
		log.Infof("Successfully synced Service '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (s *ServiceDNSController) sync(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	service, err := s.serviceInformer.Lister().Services(namespace).Get(name)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			s.dnsCache.DeleteByServiceKey(key)

			return nil
		}

		return err
	}

	fqdn := service.Annotations[connectivityv1alpha1.FQDNAnnotation]

	clusterIP := net.ParseIP(service.Spec.ClusterIP)
	if clusterIP == nil {
		utilruntime.HandleError(fmt.Errorf("invalid cluster ip: %s", service.Spec.ClusterIP))
		return nil
	}

	s.dnsCache.Upsert(DNSCacheEntry{
		ServiceKey: key,
		FQDN:       fqdn,
		IPs:        []net.IP{clusterIP},
	})

	return nil
}

func (s *ServiceDNSController) enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	s.workqueue.Add(key)
}

func (s *ServiceDNSController) handleDelete(obj interface{}) {
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
