// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/servicebinding"
	connectivityclientset "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/clientset/versioned"
	connectivityinformers "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/informers/externalversions"
)

func main() {
	restConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Errorf("error creating rest config: %v", err)
		os.Exit(1)
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		log.Errorf("error creating kubernetes clientset: %v", err)
		os.Exit(1)
	}

	connectivityClientset, err := connectivityclientset.NewForConfig(restConfig)
	if err != nil {
		log.Errorf("error creating connectivity clientset: %v", err)
		os.Exit(1)
	}

	informerFactory := informers.NewSharedInformerFactory(kubeClient, 30*time.Second)
	connectivityInformerFactory := connectivityinformers.NewSharedInformerFactory(connectivityClientset, 30*time.Second)
	serviceRecordInformer := connectivityInformerFactory.Connectivity().V1alpha1().ServiceRecords()
	serviceInformer := informerFactory.Core().V1().Services()
	endpointsInformer := informerFactory.Core().V1().Endpoints()

	serviceBindingController := servicebinding.NewServiceBindingController(kubeClient, connectivityClientset,
		serviceRecordInformer, serviceInformer, endpointsInformer)

	stopCh := make(chan struct{})
	connectivityInformerFactory.Start(stopCh)
	informerFactory.Start(stopCh)

	connectivityInformerFactory.WaitForCacheSync(stopCh)
	informerFactory.WaitForCacheSync(stopCh)

	go serviceBindingController.Run(3, stopCh)

	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChannel
		close(stopCh)
		os.Exit(0)
	}()

	select {}
}
