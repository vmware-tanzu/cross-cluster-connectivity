// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package crosscluster

import (
	"fmt"
	"time"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/prometheus/common/log"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/servicedns"
)

func init() {
	plugin.Register("crosscluster", setup)
}

func setup(c *caddy.Controller) error {
	log.Debug("Setting up crosscluster dns controller")

	dnsPlugin, err := setupController(c)
	if err != nil {
		return plugin.Error("crosscluster", err)
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		dnsPlugin.Next = next
		return dnsPlugin
	})

	return nil
}

func setupController(c *caddy.Controller) (*CrossCluster, error) {
	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("error creating rest config: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating kubernetes clientset: %v", err)
	}

	informerFactory := informers.NewSharedInformerFactory(kubeClient, 30*time.Second)
	serviceInformer := informerFactory.Core().V1().Services()

	dnsRecordsCache := new(servicedns.DNSCache)

	serviceDNSController := servicedns.NewServiceDNSController(serviceInformer, dnsRecordsCache)

	stopCh := make(chan struct{})
	informerFactory.WaitForCacheSync(stopCh)
	informerFactory.Start(stopCh)

	go serviceDNSController.Run(3, stopCh)

	c.OnShutdown(func() error {
		close(stopCh)
		return nil
	})

	dnsPlugin := &CrossCluster{
		RecordsCache: dnsRecordsCache,
	}

	// Consume the token "crosscluster" and get next token
	if c.Next() {
		dnsPlugin.Zones = c.RemainingArgs()
		if len(dnsPlugin.Zones) == 0 {
			dnsPlugin.Zones = make([]string, len(c.ServerBlockKeys))
			copy(dnsPlugin.Zones, c.ServerBlockKeys)
		}
		for i, str := range dnsPlugin.Zones {
			dnsPlugin.Zones[i] = plugin.Host(str).Normalize()
		}
	}

	return dnsPlugin, nil
}
