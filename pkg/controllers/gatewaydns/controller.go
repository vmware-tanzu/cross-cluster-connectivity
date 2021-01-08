// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package gatewaydns

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
)

// GatewayDNSReconciler reconciles a GatewayDNS object
type GatewayDNSReconciler struct {
	client.Client
	Log                     logr.Logger
	Scheme                  *runtime.Scheme
	ClientProvider          clientProvider
	ClusterSearcher         *ClusterSearcher
	EndpointSliceReconciler *EndpointSliceReconciler
	ClusterGatewayCollector *ClusterGatewayCollector
	Namespace               string
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . clientProvider
type clientProvider interface {
	GetClient(ctx context.Context, cluster client.ObjectKey) (client.Client, error)
}

// +kubebuilder:rbac:groups=connectivity.tanzu.vmware.com,resources=gatewaydns,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=connectivity.tanzu.vmware.com,resources=gatewaydns/status,verbs=get;update;patch

func (r *GatewayDNSReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("gatewaydns", req.NamespacedName)

	var gatewayDNS connectivityv1alpha1.GatewayDNS
	if err := r.Client.Get(ctx, req.NamespacedName, &gatewayDNS); err != nil {
		log.Error(err, "Failed to get gatewayDNS with name")
		return ctrl.Result{}, err
	}

	clustersWithEndpoints, err := r.ClusterSearcher.ListMatchingClusters(ctx, gatewayDNS)
	if err != nil {
		log.Error(err, "Failed to list matching clusters")
		return ctrl.Result{}, err
	}
	log.Info("Found matching clusters", "total", len(clustersWithEndpoints), "matchingClusters", clustersWithEndpoints)

	clusterGateways, err := r.ClusterGatewayCollector.GetGatewaysForClusters(ctx, gatewayDNS, clustersWithEndpoints)
	if err != nil {
		log.Error(err, "Failed to get gateways for clusters")
		return ctrl.Result{}, err
	}

	endpointSlices := ConvertGatewaysToEndpointSlices(clusterGateways, gatewayDNS.Namespace, r.Namespace)

	var clustersInGatewayDNSNamespace clusterv1alpha3.ClusterList
	err = r.Client.List(ctx, &clustersInGatewayDNSNamespace, client.InNamespace(gatewayDNS.Namespace))
	if err != nil {
		log.Error(err, "Failed to list clusters in gateway dns namespace")
		return ctrl.Result{}, err
	}

	err = r.EndpointSliceReconciler.WriteEndpointSlicesToClusters(ctx, clustersInGatewayDNSNamespace.Items, endpointSlices)
	if err != nil {
		log.Error(err, "Failed to write endpoint slices to clusters")
		return ctrl.Result{}, err
	}
	log.Info("created endpoint slices: ", "endpointSlices", endpointSlices)

	return ctrl.Result{}, nil
}

func (r *GatewayDNSReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&connectivityv1alpha1.GatewayDNS{}).
		Complete(r)
}
