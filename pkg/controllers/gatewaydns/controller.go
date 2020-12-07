// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package gatewaydns

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	discoveryv1beta1 "k8s.io/api/discovery/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
)

// GatewayDNSReconciler reconciles a GatewayDNS object
type GatewayDNSReconciler struct {
	client.Client
	Log            logr.Logger
	Scheme         *runtime.Scheme
	ClientProvider clientProvider
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
		log.Error(err, "Failed to get gatewayDNS with name", "namespacedName", req.NamespacedName.String())
		return ctrl.Result{}, err
	}

	matchingClusters, err := r.listMatchingClusters(ctx, gatewayDNS)
	if err != nil {
		log.Error(err, "Failed to list matching clusters")
		return ctrl.Result{}, err
	}
	log.Info("Found matching clusters", "matchingClusters", matchingClusters)

	endpointSlices, err := r.extractEndpointSlicesFromClusters(ctx, gatewayDNS, matchingClusters)
	if err != nil {
		log.Error(err, "Failed to extract endpoint slices from clusters")
		return ctrl.Result{}, err
	}
	log.Info("extracted endpoint slices: ", "endpointSlices", endpointSlices)

	err = r.writeEndpointSlicesToClusters(ctx, matchingClusters, endpointSlices)
	if err != nil {
		log.Error(err, "Failed to write endpoint slices to clusters")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *GatewayDNSReconciler) listMatchingClusters(ctx context.Context,
	gatewayDNS connectivityv1alpha1.GatewayDNS) ([]clusterv1alpha3.Cluster, error) {

	selector, err := metav1.LabelSelectorAsSelector(&gatewayDNS.Spec.ClusterSelector)
	if err != nil {
		return nil, err
	}

	var matchingClusters clusterv1alpha3.ClusterList
	err = r.Client.List(ctx, &matchingClusters, client.MatchingLabelsSelector{Selector: selector})
	if err != nil {
		return nil, err
	}
	return matchingClusters.Items, nil
}

func (r *GatewayDNSReconciler) extractEndpointSlicesFromClusters(ctx context.Context,
	gatewayDNS connectivityv1alpha1.GatewayDNS,
	clusters []clusterv1alpha3.Cluster) ([]discoveryv1beta1.EndpointSlice, error) {

	gatewayDNSSpecService := NewNamespacedNameFromString(gatewayDNS.Spec.Service)

	var endpointSlices []discoveryv1beta1.EndpointSlice
	for _, cluster := range clusters {
		services, err := r.extractLoadBalancedServicesFromCluster(ctx, gatewayDNSSpecService, cluster)
		if err != nil {
			return nil, err
		}
		endpointSlices = append(endpointSlices, convertServicesToEndpointSlices(services, cluster.ObjectMeta.Name, gatewayDNS.Namespace)...)
	}
	return endpointSlices, nil
}

func (r *GatewayDNSReconciler) writeEndpointSlicesToClusters(ctx context.Context,
	clusters []clusterv1alpha3.Cluster, endpointSlices []discoveryv1beta1.EndpointSlice) error {

	for _, cluster := range clusters {
		clusterClient, err := r.ClientProvider.GetClient(ctx, types.NamespacedName{
			Namespace: cluster.Namespace,
			Name:      cluster.Name,
		})
		if err != nil {
			return err
		}

		for _, endpointSlice := range endpointSlices {
			err := clusterClient.Create(ctx, &endpointSlice)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *GatewayDNSReconciler) extractLoadBalancedServicesFromCluster(ctx context.Context,
	serviceNamespacedName types.NamespacedName,
	cluster clusterv1alpha3.Cluster) ([]corev1.Service, error) {

	var services []corev1.Service
	clusterClient, err := r.ClientProvider.GetClient(ctx, types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      cluster.Name,
	})
	if err != nil {
		return nil, err
	}

	var service corev1.Service
	err = clusterClient.Get(ctx, serviceNamespacedName, &service)
	if err != nil {
		return nil, err
	}

	if service.Spec.Type == corev1.ServiceTypeLoadBalancer {
		services = append(services, service)
	}
	return services, nil
}

func convertServicesToEndpointSlices(services []corev1.Service, clusterName string, gatewayDNSNamespace string) []discoveryv1beta1.EndpointSlice {
	var endpointSlices []discoveryv1beta1.EndpointSlice
	for _, service := range services {
		endpointSlices = append(endpointSlices, convertServiceToEndpointSlice(service, clusterName, gatewayDNSNamespace))
	}
	return endpointSlices
}

func convertServiceToEndpointSlice(service corev1.Service, clusterName string, gatewayDNSNamespace string) discoveryv1beta1.EndpointSlice {

	hostname := fmt.Sprintf("*.gateway.%s.%s.clusters.xcc.test", clusterName, gatewayDNSNamespace)
	name := fmt.Sprintf("%s-%s-gateway", gatewayDNSNamespace, clusterName)

	return discoveryv1beta1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "capi-dns",
			Annotations: map[string]string{
				connectivityv1alpha1.DNSHostnameAnnotation: hostname,
			},
		},
		AddressType: discoveryv1beta1.AddressTypeIPv4,
		Endpoints: []discoveryv1beta1.Endpoint{
			{
				Addresses: service.Spec.ExternalIPs,
			},
		},
	}
}

func NewNamespacedNameFromString(s string) types.NamespacedName {
	namespacedName := types.NamespacedName{}
	result := strings.Split(s, string(types.Separator))
	if len(result) == 2 {
		namespacedName.Namespace = result[0]
		namespacedName.Name = result[1]
	}
	return namespacedName
}

func (r *GatewayDNSReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&connectivityv1alpha1.GatewayDNS{}).
		Complete(r)
}
