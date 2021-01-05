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
	log.Info("Found matching clusters", "total", len(matchingClusters), "matchingClusters", matchingClusters)

	endpointSlices, err := r.createEndpointSlicesForClusters(ctx, gatewayDNS, matchingClusters)
	if err != nil {
		log.Error(err, "Failed to create endpoint slices for clusters")
		return ctrl.Result{}, err
	}
	log.Info("created endpoint slices: ", "endpointSlices", endpointSlices)

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

func (r *GatewayDNSReconciler) createEndpointSlicesForClusters(ctx context.Context,
	gatewayDNS connectivityv1alpha1.GatewayDNS,
	clusters []clusterv1alpha3.Cluster) ([]discoveryv1beta1.EndpointSlice, error) {

	gatewayDNSSpecService := newNamespacedNameFromString(gatewayDNS.Spec.Service)

	var endpointSlices []discoveryv1beta1.EndpointSlice
	for _, cluster := range clusters {
		service, err := r.getLoadBalancerServiceForCluster(ctx, gatewayDNSSpecService, cluster)
		if err != nil {
			return nil, err
		}
		if service != nil {
			r.Log.Info("Get Load Balancer Service: ", "cluster", cluster.ClusterName, "service", service)
			endpointSlices = append(endpointSlices, convertServiceToEndpointSlice(service, cluster.ObjectMeta.Name, gatewayDNS.Namespace))
		}
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

func (r *GatewayDNSReconciler) getLoadBalancerServiceForCluster(ctx context.Context,
	serviceNamespacedName types.NamespacedName,
	cluster clusterv1alpha3.Cluster) (*corev1.Service, error) {

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

	r.Log.Info("Get Service: ", "cluster", cluster.Name, "serviceNamespacedName", serviceNamespacedName, "service", service)
	if isLoadBalancerWithExternalIP(service) {
		return &service, nil
	}
	return nil, nil
}

func isLoadBalancerWithExternalIP(service corev1.Service) bool {
	if service.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return false
	}
	if len(service.Status.LoadBalancer.Ingress) == 0 {
		return false
	}
	return true
}

func convertServiceToEndpointSlice(service *corev1.Service, clusterName string, gatewayDNSNamespace string) discoveryv1beta1.EndpointSlice {
	// TODO: xcc.test TLD should be a configuration option
	hostname := fmt.Sprintf("*.gateway.%s.%s.clusters.xcc.test", clusterName, gatewayDNSNamespace)
	name := fmt.Sprintf("%s-%s-gateway", gatewayDNSNamespace, clusterName)
	addresses := []string{}

	for _, ingress := range service.Status.LoadBalancer.Ingress {
		addresses = append(addresses, ingress.IP)
	}

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
				Addresses: addresses,
			},
		},
	}
}

func newNamespacedNameFromString(s string) types.NamespacedName {
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
