// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package gatewaydns

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	clusterv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
)

type ClusterGatewayCollector struct {
	Log            logr.Logger
	ClientProvider clientProvider
}

type ClusterGateway struct {
	ClusterName string
	Gateway     corev1.Service
}

func (e *ClusterGatewayCollector) GetGatewaysForClusters(ctx context.Context,
	gatewayDNS connectivityv1alpha1.GatewayDNS,
	clusters []clusterv1alpha3.Cluster) ([]ClusterGateway, error) {

	gatewayDNSSpecService := newNamespacedNameFromString(gatewayDNS.Spec.Service)

	var clusterGateways []ClusterGateway
	for _, cluster := range clusters {
		service, err := e.getLoadBalancerServiceForCluster(ctx, gatewayDNSSpecService, cluster)
		if err != nil {
			return []ClusterGateway{}, err
		}
		if service != nil {
			clusterGateways = append(clusterGateways, ClusterGateway{
				ClusterName: cluster.ObjectMeta.Name,
				Gateway:     *service,
			})
		}
	}
	return clusterGateways, nil
}

func (e *ClusterGatewayCollector) getLoadBalancerServiceForCluster(ctx context.Context,
	serviceNamespacedName types.NamespacedName,
	cluster clusterv1alpha3.Cluster) (*corev1.Service, error) {
	log := e.Log.WithValues("Cluster", fmt.Sprintf("%s/%s", cluster.Namespace, cluster.Name))

	clusterClient, err := e.ClientProvider.GetClient(ctx, types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      cluster.Name,
	})
	if err != nil {
		return nil, err
	}

	var service corev1.Service
	err = clusterClient.Get(ctx, serviceNamespacedName, &service)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			log.Error(err, "Expected Service not found", "Service", serviceNamespacedName.String())
			return nil, nil
		}
		return nil, err // not tested
	}

	if isLoadBalancerWithExternalIP(service) {
		log.Info("Found Service", "Service", serviceNamespacedName.String(), "ExternalIP", getExternalIPsFromStatus(service))
		return &service, nil
	}
	log.Info("Ignoring Service without type LoadBalancer or without ExternalIP", "Service", serviceNamespacedName.String())

	return nil, nil
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

func isLoadBalancerWithExternalIP(service corev1.Service) bool {
	if service.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return false
	}
	if len(service.Status.LoadBalancer.Ingress) == 0 {
		return false
	}
	return true
}

func getExternalIPsFromStatus(service corev1.Service) []string {
	var addresses []string
	for _, ingress := range service.Status.LoadBalancer.Ingress {
		addresses = append(addresses, ingress.IP)
	}

	return addresses
}
