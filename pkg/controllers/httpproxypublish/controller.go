// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package httpproxypublish

import (
	"fmt"
	"strings"
	"time"

	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	log "github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	v1informers "k8s.io/client-go/informers/core/v1"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	connectivityclientset "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/clientset/versioned"
	connectivityinformers "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/informers/externalversions/connectivity/v1alpha1"
	connectivitylisters "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/listers/connectivity/v1alpha1"
)

type HTTPProxyPublishController struct {
	scheme *runtime.Scheme

	nodeLister          v1listers.NodeLister
	connClientset       connectivityclientset.Interface
	serviceRecordLister connectivitylisters.ServiceRecordLister
	httpProxyLister     cache.GenericLister

	workqueue workqueue.RateLimitingInterface

	deletedIndexer cache.Indexer
}

var (
	// GroupVersion is group version used to register these objects
	ContourV1GroupVersion = schema.GroupVersion{Group: contourv1.GroupName, Version: "v1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	ContourV1SchemeBuilder = runtime.NewSchemeBuilder(func(scheme *runtime.Scheme) error {
		contourv1.AddKnownTypes(scheme)
		return nil
	})

	// AddToScheme adds the types in this group-version to the given scheme.
	ContourV1AddToScheme = ContourV1SchemeBuilder.AddToScheme

	daemonSetTolerations = map[string]corev1.TaintEffect{
		corev1.TaintNodeNotReady:           corev1.TaintEffectNoExecute,
		corev1.TaintNodeUnreachable:        corev1.TaintEffectNoExecute,
		corev1.TaintNodeDiskPressure:       corev1.TaintEffectNoSchedule,
		corev1.TaintNodeMemoryPressure:     corev1.TaintEffectNoSchedule,
		corev1.TaintNodeUnschedulable:      corev1.TaintEffectNoSchedule,
		corev1.TaintNodeNetworkUnavailable: corev1.TaintEffectNoSchedule,
	}
)

func NewHTTPProxyPublishController(nodeinformer v1informers.NodeInformer,
	contourInformer informers.GenericInformer,
	serviceRecordInformer connectivityinformers.ServiceRecordInformer,
	connClientset connectivityclientset.Interface,
) (*HTTPProxyPublishController, error) {

	controller := &HTTPProxyPublishController{
		scheme:              runtime.NewScheme(),
		nodeLister:          nodeinformer.Lister(),
		httpProxyLister:     contourInformer.Lister(),
		serviceRecordLister: serviceRecordInformer.Lister(),
		connClientset:       connClientset,
		workqueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "HTTPProxies"),
		deletedIndexer:      cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc, cache.Indexers{}),
	}

	if err := ContourV1AddToScheme(controller.scheme); err != nil {
		return nil, fmt.Errorf("error adding contour types to scheme: %v", err)
	}

	contourInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueue,
		UpdateFunc: func(oldObj, newObj interface{}) {
			controller.enqueue(newObj)
		},
		DeleteFunc: controller.handleDelete,
	})

	return controller, nil
}

func (h *HTTPProxyPublishController) Run(threads int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer h.workqueue.ShutDown()

	log.Info("Starting HTTPProxyPublisher controller")
	for i := 0; i < threads; i++ {
		go wait.Until(h.runWorker, time.Second, stopCh)
	}

	log.Info("Started workers")
	<-stopCh
	log.Info("Shutting down workers")

	return nil
}

func (h *HTTPProxyPublishController) runWorker() {
	for h.processNextWorkItem() {
	}
}

func (h *HTTPProxyPublishController) processNextWorkItem() bool {
	obj, shutdown := h.workqueue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer h.workqueue.Done(obj)

		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			h.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		if err := h.sync(key); err != nil {
			h.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}

		h.workqueue.Forget(obj)
		log.Infof("Successfully synced HTTPProxy '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (h *HTTPProxyPublishController) sync(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	obj, err := h.httpProxyLister.ByNamespace(namespace).Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return h.deleteServiceRecordAndCachedHTTPProxy(key)
		}

		return fmt.Errorf("error getting HTTPProxy from cache: %v", err)
	}

	httpProxy, err := h.convertToHTTPProxy(obj)
	if err != nil {
		return fmt.Errorf("error converting object to HTTPProxy: %v", err)
	}

	desiredServiceRecord, err := h.convertToServiceRecord(httpProxy)
	if err != nil {
		return fmt.Errorf("error converting HTTPProxy to ServiceRecord: %v", err)
	}

	// Delete the ServiceRecord if the label does not exist on the HTTPProxy
	if _, ok := httpProxy.Labels[connectivityv1alpha1.ExportLabel]; !ok {
		currentServiceRecord, err := h.serviceRecordLister.ServiceRecords(desiredServiceRecord.Namespace).
			Get(desiredServiceRecord.Name)
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting latest ServiceRecord from cache: %v", err)
		}

		if currentServiceRecord != nil {
			log.Infof("Syncing deleted service export label")
			err = h.connClientset.ConnectivityV1alpha1().ServiceRecords(desiredServiceRecord.Namespace).
				Delete(desiredServiceRecord.Name, &metav1.DeleteOptions{})
			if err != nil {
				return fmt.Errorf("error deleting ServiceRecord: %v", err)
			}
		}

		return nil
	}

	currentServiceRecord, err := h.serviceRecordLister.ServiceRecords(desiredServiceRecord.Namespace).
		Get(desiredServiceRecord.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// create exported ServiceRecord if it doesn't exist.
			_, err := h.connClientset.ConnectivityV1alpha1().ServiceRecords(desiredServiceRecord.Namespace).
				Create(desiredServiceRecord)
			if err != nil {
				return fmt.Errorf("error creating ServiceRecord: %v", err)
			}

			return nil
		}

		return fmt.Errorf("error getting latest ServiceRecord from cache: %v", err)
	}

	newServiceRecord := currentServiceRecord.DeepCopy()
	newServiceRecord.Annotations = desiredServiceRecord.Annotations
	newServiceRecord.Labels = desiredServiceRecord.Labels
	newServiceRecord.Spec = desiredServiceRecord.Spec

	// current state matches desired state, skip sync
	if apiequality.Semantic.DeepEqual(currentServiceRecord.Annotations, desiredServiceRecord.Annotations) &&
		apiequality.Semantic.DeepEqual(currentServiceRecord.Labels, desiredServiceRecord.Labels) &&
		apiequality.Semantic.DeepEqual(currentServiceRecord.Spec, desiredServiceRecord.Spec) {
		return nil
	}

	_, err = h.connClientset.ConnectivityV1alpha1().ServiceRecords(newServiceRecord.Namespace).
		Update(newServiceRecord)
	if err != nil {
		return fmt.Errorf("error updating ServiceRecord: %v", err)
	}

	return nil
}

func (h *HTTPProxyPublishController) enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	h.workqueue.Add(key)
}

func (h *HTTPProxyPublishController) handleDelete(obj interface{}) {
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

	err := h.deletedIndexer.Add(object)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	h.enqueue(object)
}

func (h *HTTPProxyPublishController) convertToHTTPProxy(obj runtime.Object) (*contourv1.HTTPProxy, error) {
	unstructured, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("object was not Unstructured")
	}

	switch unstructured.GetKind() {
	case "HTTPProxy":
		httpProxy := &contourv1.HTTPProxy{}
		err := h.scheme.Convert(unstructured, httpProxy, nil)
		return httpProxy, err
	default:
		return nil, fmt.Errorf("unsupported object type: %T", unstructured)
	}
}

func isNodeReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func isNodeTaintedForDaemonsets(node *corev1.Node) bool {
	for _, taint := range node.Spec.Taints {
		if _, exists := daemonSetTolerations[taint.Key]; !exists {
			return true
		}
	}
	return false
}

func (h *HTTPProxyPublishController) convertToServiceRecord(httpProxy *contourv1.HTTPProxy) (*connectivityv1alpha1.ServiceRecord, error) {
	nodes, err := h.nodeLister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("error listing nodes from cache: %v", err)
	}

	endpoints := []connectivityv1alpha1.Endpoint{}
	for _, node := range nodes {
		if !isNodeReady(node) || isNodeTaintedForDaemonsets(node) {
			continue
		}
		// skip control plane nodes for now
		if _, exists := node.Labels["node-role.kubernetes.io/master"]; exists {
			continue
		}

		for _, address := range node.Status.Addresses {
			if address.Type != corev1.NodeInternalIP {
				continue
			}

			endpoints = append(endpoints, connectivityv1alpha1.Endpoint{
				Address: address.Address,
				// assume 443 for now cause we're using Contour for ingress,
				// Hamlet v1alpha2 will also support port mappings better
				Port: 443,
			})
			break
		}
	}

	// add annotations with the connectivity label prefix
	annotations := map[string]string{}
	for key, value := range httpProxy.Annotations {
		if !strings.HasPrefix(key, connectivityv1alpha1.ConnectivityLabelPrefix) {
			continue
		}

		annotations[key] = value
	}

	// set the port annotation which we pass through labels in the Hamlet API.
	// for now we always assume the client-side port binding is 443 but this may chance in the future
	annotations[connectivityv1alpha1.ServicePortAnnotation] = "443"
	serviceRecord := &connectivityv1alpha1.ServiceRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      httpProxy.Spec.VirtualHost.Fqdn,
			Namespace: httpProxy.Namespace,
			Labels: map[string]string{
				connectivityv1alpha1.ExportLabel: "",
			},
			Annotations: annotations,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: contourv1.SchemeGroupVersion.String(),
					Kind:       "HTTPProxy",
					UID:        httpProxy.ObjectMeta.UID,
					Name:       httpProxy.ObjectMeta.Name,
				},
			},
		},
		Spec: connectivityv1alpha1.ServiceRecordSpec{
			FQDN:      httpProxy.Spec.VirtualHost.Fqdn,
			Endpoints: endpoints,
		},
	}

	return serviceRecord, nil
}

// deleteServiceRecordAndCachedHTTPProxy uses the deletedIndexer to delete the
// Service Record object. The deletedIndexer is populated by the handleDelete
// function, which occurs when the HTTPProxy is actually deleted from the
// Kubernetes API.
func (h *HTTPProxyPublishController) deleteServiceRecordAndCachedHTTPProxy(key string) error {
	deletedObj, exists, err := h.deletedIndexer.GetByKey(key)
	if !exists {
		log.Infof("Service Record for %s not found, assuming it has already been deleted.", key)
		return nil
	}
	if err != nil {
		return fmt.Errorf("error getting deleted object from cache: %v", err)
	}

	deletedObject, ok := deletedObj.(runtime.Object)
	if !ok {
		return fmt.Errorf("error converting deleted object to runtime.Object for %s", key)
	}

	deletedHTTPProxy, err := h.convertToHTTPProxy(deletedObject)
	if err != nil {
		return fmt.Errorf("error converting object to HTTPProxy: %v", err)
	}

	toBeDeletedServiceRecord, err := h.convertToServiceRecord(deletedHTTPProxy)
	if err != nil {
		return fmt.Errorf("error converting HTTPProxy to ServiceRecord: %v", err)
	}

	err = h.connClientset.ConnectivityV1alpha1().ServiceRecords(toBeDeletedServiceRecord.Namespace).
		Delete(toBeDeletedServiceRecord.Name, &metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("error deleting object: %v", err)
	}

	err = h.deletedIndexer.Delete(deletedObj)
	if err != nil {
		log.Warnf("error deleting object from cache: %v", err)
		return nil
	}

	return nil
}
