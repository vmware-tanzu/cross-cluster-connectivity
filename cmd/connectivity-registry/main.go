// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"k8s.io/client-go/rest"

	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/registryclient"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/registryserver"
	connectivityclientset "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/clientset/versioned"
	connectivityinformers "github.com/vmware-tanzu/cross-cluster-connectivity/pkg/generated/informers/externalversions"
)

func main() {
	var (
		tlsCertPath                                   string
		tlsKeyPath                                    string
		port                                          int
		orphanImportedServiceRecordDeleteDelaySeconds int
	)

	flag.StringVar(&tlsCertPath, "tls-cert", "", "the path to the server TLS certificate")
	flag.StringVar(&tlsKeyPath, "tls-key", "", "the path to the server TLS key")
	flag.IntVar(&port, "port", 8000, "the serving port for the hamlet server")
	flag.IntVar(
		&orphanImportedServiceRecordDeleteDelaySeconds,
		"orphan-imported-service-record-delete-delay-seconds",
		5 * 60,
		"delay in seconds before an orphan imported service record is deleted"
	)
	flag.Parse()

	restConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Errorf("error creating rest config: %v", err)
		os.Exit(1)
	}

	connectivityClientset, err := connectivityclientset.NewForConfig(restConfig)
	if err != nil {
		log.Errorf("error creating connectivity clientset: %v", err)
		os.Exit(1)
	}

	connectivityInformerFactory := connectivityinformers.NewSharedInformerFactory(connectivityClientset, 30*time.Second)
	serviceRecordInformer := connectivityInformerFactory.Connectivity().V1alpha1().ServiceRecords()
	remoteRegistryInformer := connectivityInformerFactory.Connectivity().V1alpha1().RemoteRegistries()

	namespace, exists := os.LookupEnv("NAMESPACE")
	if !exists {
		log.Error("NAMESPACE environment variable has not been set")
		os.Exit(1)
	}

	registryServerController, err := registryserver.NewRegistryServerController(uint32(port), tlsCertPath, tlsKeyPath, serviceRecordInformer)
	if err != nil {
		log.Errorf("error creating RegistryServer controller: %v", err)
		os.Exit(1)
	}

	registryClientController := registryclient.NewRegistryClientController(
		connectivityClientset,
		remoteRegistryInformer,
		serviceRecordInformer,
		namespace,
		orphanImportedServiceRecordDeleteDelaySeconds*time.Second,
	)

	stopCh := make(chan struct{})
	connectivityInformerFactory.Start(stopCh)
	connectivityInformerFactory.WaitForCacheSync(stopCh)

	go registryServerController.Run(3, stopCh)
	go registryClientController.Run(3, stopCh)

	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChannel
		close(stopCh)
		if err := registryServerController.Server.Stop(); err != nil {
			log.Errorf("error occured while stopping server: %v", err)
			os.Exit(1)
		}
		os.Exit(0)
	}()

	if err := registryServerController.Server.Start(); err != nil {
		log.Errorf("error occured while starting server: %v", err)
	}
}
