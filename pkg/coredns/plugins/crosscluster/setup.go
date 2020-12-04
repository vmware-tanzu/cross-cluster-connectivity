// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package crosscluster

import (
	"context"
	"errors"
	"os"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/prometheus/common/log"
	"github.com/vmware-tanzu/cross-cluster-connectivity/v2/pkg/controllers/endpointslicedns"
	discoveryv1beta1 "k8s.io/api/discovery/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"

	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("crosscluster-coredns-plugin")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = discoveryv1beta1.AddToScheme(scheme)

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
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	dnsRecordsCache := new(endpointslicedns.DNSCache)

	namespace, ok := os.LookupEnv("NAMESPACE")
	if !ok {
		setupLog.Error(errors.New("NAMESPACE environment variable unset. Must be set to the namespace that should be watched."), "unable to get NAMESPACE environment variable")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		Port:               9443,
		MetricsBindAddress: "0",
		Namespace:          namespace,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&endpointslicedns.EndpointSliceReconciler{
		Client:       mgr.GetClient(),
		Log:          ctrl.Log.WithName("controllers").WithName("EndpointSlice"),
		Scheme:       mgr.GetScheme(),
		RecordsCache: dnsRecordsCache,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "EndpointSlice")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		setupLog.Info("starting manager")

		if err := mgr.Start(ctx); err != nil {
			setupLog.Error(err, "problem running manager")
			os.Exit(1)
		}
	}()

	c.OnShutdown(func() error {
		cancel()
		return nil
	})

	dnsPlugin := &CrossCluster{
		RecordsCache: dnsRecordsCache,
		Log:          ctrl.Log.WithName("dnsserver"),
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
