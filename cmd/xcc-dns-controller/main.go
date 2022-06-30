// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"flag"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/gatewaydns"
	discoveryv1 "k8s.io/api/discovery/v1"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = connectivityv1alpha1.AddToScheme(scheme)
	_ = clusterv1beta1.AddToScheme(scheme)
	_ = discoveryv1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	namespace, ok := os.LookupEnv("NAMESPACE")
	if !ok {
		setupLog.Error(errors.New("NAMESPACE environment variable unset. Must be set to the namespace that should be watched."), "unable to get NAMESPACE environment variable")
		os.Exit(1)
	}

	domainSuffix, ok := os.LookupEnv("DOMAIN_SUFFIX")
	if !ok {
		setupLog.Error(errors.New("DOMAIN_SUFFIX environment variable unset. Sets the domain suffix on generated domain names."), "unable to get DOMAIN_SUFFIX environment variable")
		os.Exit(1)
	}

	var gatewayDNSPollingIntervalDuration time.Duration
	gatewayDNSPollingInterval, ok := os.LookupEnv("GATEWAY_DNS_POLLING_INTERVAL")
	if ok {
		var err error
		gatewayDNSPollingIntervalDuration, err = time.ParseDuration(gatewayDNSPollingInterval)
		if err != nil {
			setupLog.Error(err, "GATEWAY_DNS_POLLING_INTERVAL environment variable malformed.")
			os.Exit(1)
		}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "d6f90f8c.tanzu.vmware.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	reconcilerLog := ctrl.Log.WithName("controllers").WithName("GatewayDNS")
	cacheTrackerLog := reconcilerLog.WithName("clustercachetracker")
	clusterCacheTrackerOptions := remote.ClusterCacheTrackerOptions{Log: &cacheTrackerLog}
	clusterCacheTracker, err := remote.NewClusterCacheTracker(mgr, clusterCacheTrackerOptions)
	if err != nil {
		setupLog.Error(err, "unable to create clusterCacheTracker", "clusterCacheTracker", "GatewayDNS")
		os.Exit(1)
	}

	client := mgr.GetClient()
	if err = (&gatewaydns.GatewayDNSReconciler{
		Client:          client,
		Log:             reconcilerLog,
		PollingInterval: gatewayDNSPollingIntervalDuration,
		Scheme:          mgr.GetScheme(),
		ClientProvider:  clusterCacheTracker,
		ClusterSearcher: &gatewaydns.ClusterSearcher{Client: client},
		EndpointSliceReconciler: &gatewaydns.EndpointSliceReconciler{
			ClientProvider: clusterCacheTracker,
			Namespace:      namespace,
			Log:            reconcilerLog.WithName("EndpointSliceReconciler"),
		},
		ClusterGatewayCollector: &gatewaydns.ClusterGatewayCollector{
			Log:            reconcilerLog.WithName("EndpointSliceCollector"),
			ClientProvider: clusterCacheTracker,
			Namespace:      namespace,
			DomainSuffix:   domainSuffix,
		},
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "GatewayDNS")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
