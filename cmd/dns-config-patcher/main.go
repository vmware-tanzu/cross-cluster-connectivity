// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/dnsconfig"
)

var (
	scheme = runtime.NewScheme()
	log    = ctrl.Log.WithName("dns-config-patcher")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
}

func getEnvVarOrDie(name string, description string) string {
	envVar, ok := os.LookupEnv(name)
	if !ok {
		log.Error(fmt.Errorf("%s environment variable unset. %s", name, description), fmt.Sprintf("unable to get %s environment variable", name))
		os.Exit(1)
	}
	return envVar
}

func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	dnsServiceNamespace := getEnvVarOrDie(
		"DNS_SERVICE_NAMESPACE",
		"Must be set to the namespace of the DNS service that will handle DNS lookups for the provided DOMAIN_SUFFIX.",
	)

	dnsServiceName := getEnvVarOrDie(
		"DNS_SERVICE_NAME",
		"Must be set to the name of the DNS service that will handle DNS lookups for the provided DOMAIN_SUFFIX.",
	)

	corefileConfigMapNamespace := getEnvVarOrDie(
		"COREFILE_CONFIGMAP_NAMESPACE",
		"Must be set to the namespace of the ConfigMap containing the Corefile to be patched.",
	)

	corefileConfigMapName := getEnvVarOrDie(
		"COREFILE_CONFIGMAP_NAME",
		"Must be set to the name of the ConfigMap containing the Corefile to be patched.",
	)

	domainSuffix := getEnvVarOrDie(
		"DOMAIN_SUFFIX",
		"Must be set to the domain suffix of the zone that is handled by another DNS service.",
	)

	client, err := client.New(ctrl.GetConfigOrDie(), client.Options{
		Scheme: scheme,
	})
	if err != nil {
		log.Error(err, "unable to get client")
		os.Exit(1)
	}

	dnsServiceWatcher := dnsconfig.DNSServiceWatcher{
		Client:      client,
		Namespace:   dnsServiceNamespace,
		ServiceName: dnsServiceName,

		PollingInterval: 500 * time.Millisecond,
	}

	dnsServiceWatcherCtx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	dnsServiceClusterIP, err := dnsServiceWatcher.GetDNSServiceClusterIP(dnsServiceWatcherCtx)
	if err != nil {
		log.Error(err, "unable to get DNS service ClusterIP")
		os.Exit(1)
	}

	patcher := dnsconfig.CorefilePatcher{
		Client:       client,
		Log:          log.WithName("CorefilePatcher"),
		DomainSuffix: domainSuffix,

		Namespace:     corefileConfigMapNamespace,
		ConfigMapName: corefileConfigMapName,
	}

	if err = patcher.AppendStubDomainBlock(dnsServiceClusterIP); err != nil {
		log.Error(err, "unable to append stub domain block")
		os.Exit(1)
	}

	log.Info("successfully patched Corefile")
}
