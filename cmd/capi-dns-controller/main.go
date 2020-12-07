// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/gatewaydns"
	discoveryv1beta1 "k8s.io/api/discovery/v1beta1"
	clusterv1alpha4 "sigs.k8s.io/cluster-api/api/v1alpha3"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = connectivityv1alpha1.AddToScheme(scheme)
	_ = clusterv1alpha4.AddToScheme(scheme)
	_ = discoveryv1beta1.AddToScheme(scheme)
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
	clusterCacheTrackerLog := reconcilerLog.WithName("clustercachetracker")
	clusterCacheTracker, err := remote.NewClusterCacheTracker(clusterCacheTrackerLog, mgr)
	if err != nil {
		setupLog.Error(err, "unable to create clusterCacheTracker", "clusterCacheTracker", "GatewayDNS")
		os.Exit(1)
	}

	if err = (&gatewaydns.GatewayDNSReconciler{
		Client:         mgr.GetClient(),
		Log:            reconcilerLog,
		Scheme:         mgr.GetScheme(),
		ClientProvider: clusterCacheTracker,
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
