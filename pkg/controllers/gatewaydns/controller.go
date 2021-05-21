// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package gatewaydns

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	clusterv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

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

	// PollingInterval defaults to 30 seconds if not provided
	PollingInterval time.Duration
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . clientProvider
type clientProvider interface {
	GetClient(ctx context.Context, cluster client.ObjectKey) (client.Client, error)
}

// +kubebuilder:rbac:groups=connectivity.tanzu.vmware.com,resources=gatewaydns,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=connectivity.tanzu.vmware.com,resources=gatewaydns/status,verbs=get;update;patch

func (r *GatewayDNSReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("GatewayDNS", req.NamespacedName)
	log.Info("Start Reconciling")

	var gatewayDNS connectivityv1alpha1.GatewayDNS
	if err := r.Client.Get(ctx, req.NamespacedName, &gatewayDNS); err != nil {
		if k8serrors.IsNotFound(err) {
			err := r.convergeOnClustersForGatewayDNS(ctx, log, req.NamespacedName, nil)
			if err != nil {
				return ctrl.Result{}, err
			}
			log.Info("Finished Reconciling")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get GatewayDNS with name")
		return ctrl.Result{}, err
	}

	log.Info("Searching for Clusters", "ClusterSelector", gatewayDNS.Spec.ClusterSelector, "Service", gatewayDNS.Spec.Service)
	clustersWithEndpoints, err := r.ClusterSearcher.ListMatchingClusters(ctx, gatewayDNS)
	if err != nil {
		log.Error(err, "Failed to list matching Clusters")
		return ctrl.Result{}, err
	}
	log.Info("Found matching Clusters", "Total", len(clustersWithEndpoints), "Clusters", clustersToNames(clustersWithEndpoints))

	clusterGateways := r.ClusterGatewayCollector.GetGatewaysForClusters(ctx, gatewayDNS, clustersWithEndpoints)

	err = r.convergeOnClustersForGatewayDNS(ctx, log, req.NamespacedName, clusterGateways)
	if err != nil {
		return ctrl.Result{}, err
	}
	log.Info("Finished Reconciling")

	return ctrl.Result{}, nil
}

func (r *GatewayDNSReconciler) convergeOnClustersForGatewayDNS(ctx context.Context, log logr.Logger, namespacedName types.NamespacedName, clusterGateways []ClusterGateway) error {
	var clustersInGatewayDNSNamespace clusterv1alpha3.ClusterList
	err := r.Client.List(ctx, &clustersInGatewayDNSNamespace, client.InNamespace(namespacedName.Namespace))
	if err != nil {
		log.Error(err, "Failed to list clusters in gateway dns namespace")
		return err
	}

	errs := r.EndpointSliceReconciler.ConvergeToClusters(ctx, clustersInGatewayDNSNamespace.Items, namespacedName, clusterGateways)
	if len(errs) > 0 {
		return errors.New("Failed to converge EndpointSlices")
	}

	return nil
}

func (r *GatewayDNSReconciler) SetupWithManager(mgr ctrl.Manager) error {
	pollEventsCh := r.PollGatewayDNS()
	return ctrl.NewControllerManagedBy(mgr).
		For(&connectivityv1alpha1.GatewayDNS{}).
		Watches(
			&source.Channel{
				Source: pollEventsCh,
			},
			handler.Funcs{
				GenericFunc: func(e event.GenericEvent, q workqueue.RateLimitingInterface) {
					q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
						Name:      e.Object.GetName(),
						Namespace: e.Object.GetNamespace(),
					}})
				},
			},
		).
		Watches(
			&source.Kind{Type: &clusterv1alpha3.Cluster{}},
			handler.EnqueueRequestsFromMapFunc(r.ClusterToGatewayDNS),
		).
		Complete(r)
}

func (r *GatewayDNSReconciler) PollGatewayDNS() <-chan event.GenericEvent {
	log := r.Log.WithName("Poll")
	pollEventsCh := make(chan event.GenericEvent)

	pollingInterval := r.PollingInterval
	if pollingInterval == 0 {
		pollingInterval = 30 * time.Second
	}
	log.Info("Start", "PollingInterval", pollingInterval)

	go func() {
		for {
			<-time.After(pollingInterval)
			var gatewayDNSList connectivityv1alpha1.GatewayDNSList
			err := r.Client.List(context.Background(), &gatewayDNSList)
			if err != nil {
				log.Error(err, "Failed to list GatewayDNS")
				continue
			}

			for _, gatewayDNS := range gatewayDNSList.Items {
				gatewayDNSCopy := gatewayDNS
				pollEventsCh <- event.GenericEvent{
					Object: &gatewayDNSCopy,
				}
			}
		}
	}()

	return pollEventsCh
}

func (r *GatewayDNSReconciler) ClusterToGatewayDNS(o client.Object) []reconcile.Request {
	log := r.Log.WithName("ClusterToGatewayDNS")
	var gatewayDNSList connectivityv1alpha1.GatewayDNSList
	err := r.Client.List(
		context.Background(),
		&gatewayDNSList,
		client.InNamespace(o.GetNamespace()),
	)
	if err != nil {
		log.Error(err, "Failed to list GatewayDNS")
		return nil
	}

	matchingGatewayDNS := []reconcile.Request{}
	clusterLabels := labels.Set(o.GetLabels())

	for _, gatewayDNS := range gatewayDNSList.Items {
		selector, err := metav1.LabelSelectorAsSelector(&gatewayDNS.Spec.ClusterSelector)
		if err != nil {
			log.Error(err, "Encountered invalid Selector as LabelSelector", "GatewayDNS", fmt.Sprintf("%s/%s", gatewayDNS.Namespace, gatewayDNS.Name))
			continue
		}
		if selector.Matches(clusterLabels) {
			matchingGatewayDNS = append(matchingGatewayDNS, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      gatewayDNS.Name,
					Namespace: gatewayDNS.Namespace,
				},
			})
		}
	}

	return matchingGatewayDNS
}

func clustersToNames(clusters []clusterv1alpha3.Cluster) []string {
	var names []string
	for _, cluster := range clusters {
		names = append(names, fmt.Sprintf("%s/%s", cluster.Namespace, cluster.Name))
	}

	return names
}
