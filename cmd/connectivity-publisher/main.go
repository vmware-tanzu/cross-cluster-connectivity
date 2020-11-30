// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	contourv1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	log "github.com/sirupsen/logrus"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/httpproxypublish"
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

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		log.Errorf("error creating dynamic clientset: %v", err)
		os.Exit(1)
	}

	connectivityClientset, err := connectivityclientset.NewForConfig(restConfig)
	if err != nil {
		log.Errorf("error creating connectivity clientset: %v", err)
		os.Exit(1)
	}

	coreInformerFactory := informers.NewSharedInformerFactory(kubeClient, 30*time.Second)
	nodeInformer := coreInformerFactory.Core().V1().Nodes()

	contourInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 30*time.Second)
	contourInformer := contourInformerFactory.ForResource(contourv1.HTTPProxyGVR)

	connectivityInformerFactory := connectivityinformers.NewSharedInformerFactoryWithOptions(connectivityClientset, 30*time.Second)
	serviceRecordInformer := connectivityInformerFactory.Connectivity().V1alpha1().ServiceRecords()

	httpProxyPublishController, err := httpproxypublish.NewHTTPProxyPublishController(
		nodeInformer, contourInformer, serviceRecordInformer, connectivityClientset)
	if err != nil {
		log.Errorf("error creating HTTPProxyPublish controller: %v", err)
		os.Exit(1)
	}

	stopCh := make(chan struct{})

	contourInformerFactory.Start(stopCh)
	coreInformerFactory.Start(stopCh)
	connectivityInformerFactory.Start(stopCh)

	contourInformerFactory.WaitForCacheSync(stopCh)
	coreInformerFactory.WaitForCacheSync(stopCh)
	connectivityInformerFactory.WaitForCacheSync(stopCh)

	go httpProxyPublishController.Run(3, stopCh)

	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChannel
		close(stopCh)
		os.Exit(0)
	}()

	select {}
}
