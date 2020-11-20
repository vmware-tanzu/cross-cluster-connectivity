// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package orphandeleter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/common/log"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"

	connectivityclientset "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/clientset/versioned"
	connectivitylisters "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/listers/connectivity/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type OrphanDeleter struct {
	serviceRecordLister  connectivitylisters.ServiceRecordLister
	connClientSet        connectivityclientset.Interface
	namespace            string
	deleteOrphanTimer    *time.Timer
	deleteOrphanDelay    time.Duration
	remoteServiceRecords map[types.NamespacedName]struct{}

	mutex sync.RWMutex
}

func NewOrphanDeleter(
	serviceRecordLister connectivitylisters.ServiceRecordLister,
	connClientSet connectivityclientset.Interface,
	namespace string,
	deleteOrphanDelay time.Duration,
) *OrphanDeleter {
	return &OrphanDeleter{
		serviceRecordLister:  serviceRecordLister,
		connClientSet:        connClientSet,
		namespace:            namespace,
		remoteServiceRecords: map[types.NamespacedName]struct{}{},
		deleteOrphanDelay:    deleteOrphanDelay,
	}
}

func (o *OrphanDeleter) Reset() {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	if o.deleteOrphanTimer != nil {
		o.deleteOrphanTimer.Stop()
		o.deleteOrphanTimer = nil
	}
	o.remoteServiceRecords = map[types.NamespacedName]struct{}{}
}

func (o *OrphanDeleter) AddRemoteServiceRecord(namespacedName types.NamespacedName) {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	if o.deleteOrphanTimer == nil {
		o.deleteOrphanTimer = time.AfterFunc(o.deleteOrphanDelay, o.deleteOrphans)
	}

	o.remoteServiceRecords[namespacedName] = struct{}{}
}

func (o *OrphanDeleter) copyOfRemoteServiceRecords() map[types.NamespacedName]struct{} {
	remoteServiceRecords := map[types.NamespacedName]struct{}{}
	o.mutex.RLock()
	defer o.mutex.RUnlock()
	for k, v := range o.remoteServiceRecords {
		remoteServiceRecords[k] = v
	}
	return remoteServiceRecords
}

func (o *OrphanDeleter) deleteOrphans() {
	log.Infof("Cleaning up orphaned service records")

	importedServiceRecords, err := o.listServiceRecords()
	if err != nil {
		log.Errorf("error listing service records: %s", err)
	}

	remoteServiceRecords := o.copyOfRemoteServiceRecords()
	for namespacedName, _ := range importedServiceRecords {
		if _, ok := remoteServiceRecords[namespacedName]; ok {
			continue
		}

		log.Infof("Deleting Service Record: %s", namespacedName)
		err := o.deleteServiceRecord(namespacedName)
		if err != nil {
			log.Errorf("error deleting orphaned service record: %s", err)
		}
	}
}

func (o *OrphanDeleter) listServiceRecords() (map[types.NamespacedName]struct{}, error) {
	serviceRecords, err := o.serviceRecordLister.ServiceRecords(o.namespace).
		List(labels.Everything())
	if err != nil {
		return nil, err
	}

	serviceRecordsNamespacedName := map[types.NamespacedName]struct{}{}
	for _, serviceRecord := range serviceRecords {
		namespacedName := types.NamespacedName{
			Namespace: serviceRecord.Namespace,
			Name:      serviceRecord.Name,
		}
		serviceRecordsNamespacedName[namespacedName] = struct{}{}
	}

	return serviceRecordsNamespacedName, nil
}

func (o *OrphanDeleter) deleteServiceRecord(namespacedName types.NamespacedName) error {
	currentServiceRecord, err := o.serviceRecordLister.ServiceRecords(namespacedName.Namespace).Get(namespacedName.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// already deleted
			return nil
		}

		return fmt.Errorf("error getting current ServiceRecord: %v", err)
	}

	return o.connClientSet.ConnectivityV1alpha1().ServiceRecords(currentServiceRecord.Namespace).Delete(context.Background(), currentServiceRecord.Name, metav1.DeleteOptions{})
}
