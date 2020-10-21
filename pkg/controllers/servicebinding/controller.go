// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package servicebinding

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	connectivityclientset "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/clientset/versioned"
	connectivityinformers "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/informers/externalversions/connectivity/v1alpha1"
	connectivitylisters "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/listers/connectivity/v1alpha1"
)

type ServiceBindingController struct {
	kubeClient          kubernetes.Interface
	connClient          connectivityclientset.Interface
	serviceRecordLister connectivitylisters.ServiceRecordLister
	serviceLister       corelisters.ServiceLister
	endpointsLister     corelisters.EndpointsLister

	workqueue workqueue.RateLimitingInterface

	deletedIndexer cache.Indexer
}

func NewServiceBindingController(kubeClient kubernetes.Interface, connClient connectivityclientset.Interface,
	serviceRecordInformer connectivityinformers.ServiceRecordInformer,
	serviceInformer coreinformers.ServiceInformer,
	endpointsInformer coreinformers.EndpointsInformer) *ServiceBindingController {

	controller := &ServiceBindingController{
		kubeClient:          kubeClient,
		connClient:          connClient,
		serviceRecordLister: serviceRecordInformer.Lister(),
		serviceLister:       serviceInformer.Lister(),
		endpointsLister:     endpointsInformer.Lister(),
		workqueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ServiceRecords"),
		deletedIndexer:      cache.NewIndexer(cache.DeletionHandlingMetaNamespaceKeyFunc, cache.Indexers{}),
	}

	serviceRecordInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueue,
		UpdateFunc: func(oldObj, newObj interface{}) {
			controller.enqueue(newObj)
		},
		DeleteFunc: controller.handleDelete,
	})

	return controller
}

func (s *ServiceBindingController) Run(threads int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer s.workqueue.ShutDown()

	log.Info("Starting ServiceBinding controller")
	for i := 0; i < threads; i++ {
		go wait.Until(s.runWorker, time.Second, stopCh)
	}

	log.Info("Started workers")
	<-stopCh
	log.Info("Shutting down workers")

	return nil
}

func (s *ServiceBindingController) runWorker() {
	for s.processNextWorkItem() {
	}
}

func (s *ServiceBindingController) processNextWorkItem() bool {
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

func (s *ServiceBindingController) enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	s.workqueue.Add(key)
}

func (s *ServiceBindingController) handleDelete(obj interface{}) {
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

	log.Infof("handling delete for Service Record %v", obj)
	err := s.deletedIndexer.Add(object)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	s.enqueue(object)
}

func (s *ServiceBindingController) sync(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// fetch the latest imported ServiceRecord from cache
	serviceRecord, err := s.serviceRecordLister.ServiceRecords(namespace).Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			deletedObj, exists, indexerErr := s.deletedIndexer.GetByKey(key)
			if !exists {
				log.Infof("Service Record %s not found, assuming it has already been deleted.", key)
				return err
			}
			if indexerErr != nil {
				return fmt.Errorf("error getting deleted object from cache: %v", err)
			}
			var ok bool
			if serviceRecord, ok = deletedObj.(*connectivityv1alpha1.ServiceRecord); !ok {
				return fmt.Errorf("error getting deleted object from cache: object %v was not a ServiceRecord", key)
			}
		}
	}

	importLabelRequirement, err := labels.NewRequirement(connectivityv1alpha1.ImportedLabel, selection.Exists, []string{})
	if err != nil {
		return err
	}

	// fetch all imported ServiceRecords from the same namespace
	importedServiceRecords, err := s.serviceRecordLister.ServiceRecords(namespace).List(labels.NewSelector().Add(*importLabelRequirement))
	if err != nil {
		return err
	}

	// find all imported ServiceRecords in the same namespace and the same FQDN
	desiredFQDN := serviceRecord.Spec.FQDN
	matchingServiceRecords := []*connectivityv1alpha1.ServiceRecord{}
	for _, serviceRecord := range importedServiceRecords {
		if serviceRecord.Spec.FQDN != desiredFQDN {
			continue
		}

		matchingServiceRecords = append(matchingServiceRecords, serviceRecord)
	}

	// Don't do any updates if there are no more matching service records.
	// Services and endpoints should be garbage collected because they have
	// owner references.
	if len(matchingServiceRecords) == 0 {
		return nil
	}

	ownerReferences := []metav1.OwnerReference{}

	// copy the current serviceRecord and reset endpoints which will be populated based
	// on all imported ServiceRecords w/ a matching FQDN
	consolidatedServiceRecord := serviceRecord.DeepCopy()
	consolidatedServiceRecord.Spec.Endpoints = []connectivityv1alpha1.Endpoint{}

	for _, serviceRecord := range matchingServiceRecords {
		consolidatedServiceRecord.Spec.Endpoints = append(consolidatedServiceRecord.Spec.Endpoints,
			serviceRecord.Spec.Endpoints...)

		ownerReferences = append(ownerReferences, metav1.OwnerReference{
			APIVersion: connectivityv1alpha1.SchemeGroupVersion.String(),
			Kind:       "ServiceRecord",
			UID:        serviceRecord.GetUID(),
			Name:       serviceRecord.GetName(),
		})
	}

	// sort ServiceRecord endpoints by address to ensure equality against existing Endpoints
	sort.Slice(consolidatedServiceRecord.Spec.Endpoints, func(i, j int) bool {
		return consolidatedServiceRecord.Spec.Endpoints[i].Address < consolidatedServiceRecord.Spec.Endpoints[j].Address
	})

	// sort OwnerReferences by UID to ensure equality against existing OwnerReferences
	sort.Slice(ownerReferences, func(i, j int) bool {
		return ownerReferences[i].UID < ownerReferences[j].UID
	})

	if err := s.syncService(consolidatedServiceRecord, ownerReferences); err != nil {
		return err
	}

	if err := s.syncEndpoints(consolidatedServiceRecord, ownerReferences); err != nil {
		return err
	}

	return nil
}

func (s *ServiceBindingController) syncService(fs *connectivityv1alpha1.ServiceRecord, ownerReferences []metav1.OwnerReference) error {
	serviceName := serviceNameForFQDN(fs.Spec.FQDN)

	portString, found := fs.Annotations[connectivityv1alpha1.ServicePortAnnotation]
	if !found {
		return fmt.Errorf("error syncing Service, port annotation not set on ServiceRecord")
	}

	servicePort, err := strconv.Atoi(portString)
	if err != nil {
		return fmt.Errorf("error converting port string to int: %v", err)
	}

	// fetch the latest Service from cache
	currentService, err := s.serviceLister.Services(connectivityv1alpha1.ConnectivityNamespace).Get(serviceName)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error syncing Service for imported ServiceRecord %s: %v", fs.Namespace+"/"+fs.Name, err)
	}

	if apierrors.IsNotFound(err) {
		newService := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:            serviceName,
				Namespace:       connectivityv1alpha1.ConnectivityNamespace,
				OwnerReferences: ownerReferences,
				Annotations: map[string]string{
					connectivityv1alpha1.FQDNAnnotation: fs.Spec.FQDN,
				},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Port:     int32(servicePort),
						Protocol: corev1.ProtocolTCP,
					},
				},
			},
		}

		newService, err = s.kubeClient.CoreV1().Services(newService.Namespace).Create(newService)
		if err != nil {
			return err
		}

		globalVIP, exists := fs.Annotations[connectivityv1alpha1.GlobalVIPAnnotation]
		if exists {
			newService.Status = corev1.ServiceStatus{
				LoadBalancer: corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{
						{
							IP: globalVIP,
						},
					},
				},
			}

			_, err = s.kubeClient.CoreV1().Services(newService.Namespace).UpdateStatus(newService)
			if err != nil {
				return err
			}
		}

		return nil
	}

	// Service exists, update if desired state is not met
	newService := currentService.DeepCopy()
	newService.OwnerReferences = ownerReferences
	newService.Spec.Ports = []corev1.ServicePort{
		{
			Port:     int32(servicePort),
			Protocol: corev1.ProtocolTCP,
		},
	}

	var desiredStatus corev1.ServiceStatus
	globalVIP, exists := fs.Annotations[connectivityv1alpha1.GlobalVIPAnnotation]
	if exists {
		desiredStatus = corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{
						IP: globalVIP,
					},
				},
			},
		}
	}

	// actual ports don't match desired ports, or
	// actual ownerReferences don't match desired ownerReferences
	// => reconcile
	if !apiequality.Semantic.DeepEqual(currentService.Spec.Ports, newService.Spec.Ports) ||
		!apiequality.Semantic.DeepEqual(currentService.OwnerReferences, newService.OwnerReferences) {
		newService, err = s.kubeClient.CoreV1().Services(newService.Namespace).Update(newService)
		if err != nil {
			return err
		}
	}

	// actual status doesn't match desired status, reconcile
	if !apiequality.Semantic.DeepEqual(newService.Status, desiredStatus) {
		newService.Status = desiredStatus
		_, err = s.kubeClient.CoreV1().Services(newService.Namespace).UpdateStatus(newService)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *ServiceBindingController) syncEndpoints(fs *connectivityv1alpha1.ServiceRecord, ownerReferences []metav1.OwnerReference) error {
	// endpoints name must match the service name
	endpointsName := serviceNameForFQDN(fs.Spec.FQDN)

	// fetch the latest Endpoints from cache
	currentEndpoints, err := s.endpointsLister.Endpoints(connectivityv1alpha1.ConnectivityNamespace).Get(endpointsName)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error syncing Endpoints for imported ServiceRecord %s: %v", fs.Namespace+"/"+fs.Name, err)
	}

	desiredSubsets := convertToEndpointSubsets(fs)
	if apierrors.IsNotFound(err) {
		endpoints := &corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{
				Name:            endpointsName,
				Namespace:       connectivityv1alpha1.ConnectivityNamespace,
				OwnerReferences: ownerReferences,
			},
			Subsets: desiredSubsets,
		}

		if _, err = s.kubeClient.CoreV1().Endpoints(endpoints.Namespace).Create(endpoints); err != nil {
			return err
		}

		return nil
	}

	newEndpoints := currentEndpoints.DeepCopy()
	newEndpoints.OwnerReferences = ownerReferences
	newEndpoints.Subsets = desiredSubsets

	// skip reconcile if subset is equal and if ownerReferences are equal
	if apiequality.Semantic.DeepEqual(newEndpoints.Subsets, currentEndpoints.Subsets) &&
		apiequality.Semantic.DeepEqual(newEndpoints.OwnerReferences, currentEndpoints.OwnerReferences) {
		return nil
	}

	_, err = s.kubeClient.CoreV1().Endpoints(newEndpoints.Namespace).Update(newEndpoints)
	if err != nil {
		return fmt.Errorf("error updating Service: %v", err)
	}

	return nil
}

func convertToEndpointSubsets(fs *connectivityv1alpha1.ServiceRecord) []corev1.EndpointSubset {
	endpointAddresses := []corev1.EndpointAddress{}
	ports := map[uint32]struct{}{}
	for _, endpoint := range fs.Spec.Endpoints {
		endpointAddresses = append(endpointAddresses, corev1.EndpointAddress{
			IP: endpoint.Address,
		})

		if _, exists := ports[endpoint.Port]; !exists {
			ports[endpoint.Port] = struct{}{}
		}
	}

	// TODO: split out to different subsets based on ports
	endpointPorts := []corev1.EndpointPort{}
	for port := range ports {
		endpointPorts = append(endpointPorts, corev1.EndpointPort{
			Port: int32(port),
		})
	}

	return []corev1.EndpointSubset{
		{
			Addresses: endpointAddresses,
			Ports:     endpointPorts,
		},
	}
}

func serviceNameForFQDN(fqdn string) string {
	// service names cant contain "."
	fqdnNoDots := strings.ReplaceAll(fqdn, ".", "-")
	fqdnHash := createHash(fqdn)
	if len(fqdnNoDots) > 54 {
		fqdnNoDots = fqdnNoDots[:54]
	}
	return fmt.Sprintf("%s-%s", fqdnNoDots, fqdnHash)
}

func createHash(s string) string {
	hasher := fnv.New32a()
	// This never returns an error
	_, _ = hasher.Write([]byte(s))
	return fmt.Sprintf("%08x", hasher.Sum32())

}
