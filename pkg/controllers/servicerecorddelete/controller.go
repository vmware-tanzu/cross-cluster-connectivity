// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package servicerecorddelete

import (
	"fmt"
	"time"

	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	connectivityclientset "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/clientset/versioned"
	connectivityinformers "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/informers/externalversions/connectivity/v1alpha1"
	connectivitylisters "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/listers/connectivity/v1alpha1"
)

type ServiceRecordOrphanDeleteController struct {
	scheme *runtime.Scheme

	connClient          connectivityclientset.Interface
	serviceRecordLister connectivitylisters.ServiceRecordLister
	httpProxyLister     cache.GenericLister

	workqueue workqueue.RateLimitingInterface
}

var (
	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	ContourV1SchemeBuilder = runtime.NewSchemeBuilder(func(scheme *runtime.Scheme) error {
		contourv1.AddKnownTypes(scheme)
		return nil
	})

	// AddToScheme adds the types in this group-version to the given scheme.
	ContourV1AddToScheme = ContourV1SchemeBuilder.AddToScheme
)

func NewServiceRecordOrphanDeleteController(connClient connectivityclientset.Interface,
	serviceRecordInformer connectivityinformers.ServiceRecordInformer, contourInformer informers.GenericInformer) (*ServiceRecordOrphanDeleteController, error) {

	controller := &ServiceRecordOrphanDeleteController{
		scheme:              runtime.NewScheme(),
		connClient:          connClient,
		serviceRecordLister: serviceRecordInformer.Lister(),
		httpProxyLister:     contourInformer.Lister(),
		workqueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ServiceRecords"),
	}

	if err := ContourV1AddToScheme(controller.scheme); err != nil {
		return nil, fmt.Errorf("error adding contour types to scheme: %v", err)
	}

	serviceRecordInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueue,
		UpdateFunc: func(oldObj, newObj interface{}) {
			controller.enqueue(newObj)
		},
	})

	return controller, nil
}

func (s *ServiceRecordOrphanDeleteController) Run(threads int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer s.workqueue.ShutDown()

	log.Info("Starting ServiceRecordOrphanDelete controller")
	for i := 0; i < threads; i++ {
		go wait.Until(s.runWorker, time.Second, stopCh)
	}

	log.Info("Started workers")
	<-stopCh
	log.Info("Shutting down workers")

	return nil
}

func (s *ServiceRecordOrphanDeleteController) runWorker() {
	for s.processNextWorkItem() {
	}
}

func (s *ServiceRecordOrphanDeleteController) processNextWorkItem() bool {
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
		log.Infof("Successfully synced ServiceRecord '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (s *ServiceRecordOrphanDeleteController) enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	s.workqueue.Add(key)
}

func (s *ServiceRecordOrphanDeleteController) sync(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// fetch the latest ServiceRecord from cache
	serviceRecord, err := s.serviceRecordLister.ServiceRecords(namespace).Get(name)
	if err != nil {
		return fmt.Errorf("error getting ServiceRecords from cache: %v", err)
	}

	httpProxies, err := s.httpProxyLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("error listing HTTPProxies from cache: %v", err)
	}

	for _, obj := range httpProxies {
		httpProxy, err := s.convertToHTTPProxy(obj)
		if err != nil {
			return fmt.Errorf("error converting object to HTTPProxy: %v", err)
		}

		if serviceRecord.Name == httpProxy.Spec.VirtualHost.Fqdn {
			return nil
		}
	}

	err = s.connClient.ConnectivityV1alpha1().ServiceRecords(namespace).
		Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("error deleting ServiceRecord: %v", err)
	}

	return nil
}

func (s *ServiceRecordOrphanDeleteController) convertToHTTPProxy(obj runtime.Object) (*contourv1.HTTPProxy, error) {
	unstructured, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("object was not Unstructured")
	}

	switch unstructured.GetKind() {
	case "HTTPProxy":
		httpProxy := &contourv1.HTTPProxy{}
		err := s.scheme.Convert(unstructured, httpProxy, nil)
		return httpProxy, err
	default:
		return nil, fmt.Errorf("unsupported object type: %T", unstructured)
	}
}
