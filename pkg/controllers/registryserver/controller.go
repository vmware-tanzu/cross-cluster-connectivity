// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package registryserver

import (
	"crypto/tls"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	log "github.com/sirupsen/logrus"
	hamletv1alpha1 "github.com/vmware/hamlet/api/types/v1alpha1"
	"github.com/vmware/hamlet/pkg/server"
	"github.com/vmware/hamlet/pkg/server/state"
	hamlettls "github.com/vmware/hamlet/pkg/tls"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	connectivityinformers "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/informers/externalversions/connectivity/v1alpha1"
	connectivitylisters "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/listers/connectivity/v1alpha1"
)

type RegistryServerController struct {
	sync.Mutex

	state.StateProvider
	server.Server

	periodicCertLoader  *hamlettls.PeriodicCertLoader
	serviceRecordLister connectivitylisters.ServiceRecordLister

	workqueue workqueue.RateLimitingInterface
}

func NewRegistryServerController(port uint32, serverCertPath, serverKeyPath string,
	serviceRecordInformer connectivityinformers.ServiceRecordInformer) (*RegistryServerController, error) {
	controller := &RegistryServerController{
		serviceRecordLister: serviceRecordInformer.Lister(),
		workqueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ServiceRecords"),
	}

	var err error
	controller.periodicCertLoader, err = hamlettls.NewPeriodicCertLoader(serverCertPath, serverKeyPath, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("error creating periodic cert loader: %v", err)
	}

	tlsConfig := &tls.Config{
		ClientAuth: tls.VerifyClientCertIfGiven,
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			return controller.periodicCertLoader.Current(), nil
		},
	}

	s, err := server.NewServer(uint32(port), tlsConfig, controller)
	if err != nil {
		return nil, fmt.Errorf("error creating Hamlet server: %v", err)
	}

	controller.Server = s

	serviceRecordInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueServiceRecord,
		UpdateFunc: func(oldObj, newObj interface{}) {
			controller.enqueueServiceRecord(newObj)
		},
		DeleteFunc: controller.handleDelete,
	})

	return controller, nil
}

func (r *RegistryServerController) Run(threads int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer r.workqueue.ShutDown()

	log.Info("Starting PeriodicCertLoader for RegistryServer controller")
	go r.periodicCertLoader.Start()

	log.Info("Starting RegistryServer controller")
	for i := 0; i < threads; i++ {
		go wait.Until(r.runWorker, time.Second, stopCh)
	}

	log.Info("Started workers")
	<-stopCh
	log.Info("Shutting down workers")

	return nil
}

func (r *RegistryServerController) GetState(string) ([]proto.Message, error) {
	log.Info("Getting Service State")

	// only serve ServiceRecord resources with the export label
	exportLabelRequirement, err := labels.NewRequirement(connectivityv1alpha1.ExportLabel, selection.Exists, []string{})
	if err != nil {
		return nil, err
	}

	serviceRecords, err := r.serviceRecordLister.List(labels.NewSelector().Add(*exportLabelRequirement))
	if err != nil {
		return nil, err
	}

	services := []proto.Message{}
	for _, sr := range serviceRecords {
		hamletFederatedService := convertToHamletFederatedService(sr)
		services = append(services, hamletFederatedService)
	}

	return services, nil
}

func (r *RegistryServerController) runWorker() {
	for r.processNextWorkItem() {
	}
}

func (r *RegistryServerController) processNextWorkItem() bool {
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
		log.Infof("Successfully synced ServiceRecord '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (r *RegistryServerController) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// fetch the latest ServiceRecord from cache
	federatedService, err := r.serviceRecordLister.ServiceRecords(namespace).Get(name)
	if err != nil {
		return err
	}

	// Create and Update notifications on the hamlet stream are effectively the same
	// since connected clients perform the same action for both notificaions
	hamletFederatedService := convertToHamletFederatedService(federatedService)
	return r.Resources().Update(hamletFederatedService)
}

func (r *RegistryServerController) handleDelete(obj interface{}) {
	var fs *connectivityv1alpha1.ServiceRecord
	var ok bool
	if fs, ok = obj.(*connectivityv1alpha1.ServiceRecord); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		fs, ok = tombstone.Obj.(*connectivityv1alpha1.ServiceRecord)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
	}

	if _, exists := fs.Labels[connectivityv1alpha1.ExportLabel]; !exists {
		return
	}

	hamletFederatedService := convertToHamletFederatedService(fs)
	if err := r.Resources().Delete(hamletFederatedService); err != nil {
		log.Errorf("error sending delete notifiation to hamlet stream: %v", err)
	}
}

func (r *RegistryServerController) enqueueServiceRecord(obj interface{}) {
	meta, err := meta.Accessor(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	// we only care about exported ServiceRecords on the server
	labels := meta.GetLabels()
	if _, exists := labels[connectivityv1alpha1.ExportLabel]; !exists {
		return
	}

	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}

	r.workqueue.Add(key)
}

func convertToHamletFederatedService(fs *connectivityv1alpha1.ServiceRecord) *hamletv1alpha1.FederatedService {
	endpoints := []*hamletv1alpha1.FederatedService_Endpoint{}
	for _, endpoint := range fs.Spec.Endpoints {
		endpoints = append(endpoints, &hamletv1alpha1.FederatedService_Endpoint{
			Address: endpoint.Address,
			Port:    endpoint.Port,
		})
	}

	labels := map[string]string{}
	for key, value := range fs.Annotations {
		// translate annotations with the connectivity prefix as labels on the federated service
		if !strings.HasPrefix(key, connectivityv1alpha1.ConnectivityLabelPrefix) {
			continue
		}

		labels[key] = value
	}

	federatedService := &hamletv1alpha1.FederatedService{
		Name:      fs.Spec.FQDN,
		Fqdn:      fs.Spec.FQDN,
		Id:        fs.Spec.FQDN, // TODO: use a different value here?
		Labels:    labels,
		Protocols: []string{"https"},
		Endpoints: endpoints,
	}

	return federatedService
}
