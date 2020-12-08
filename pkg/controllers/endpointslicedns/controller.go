// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package endpointslicedns

import (
	"context"
	"net"

	"github.com/go-logr/logr"
	connectivityv1alpha1 "github.com/vmware-tanzu/cross-cluster-connectivity/apis/connectivity/v1alpha1"
	discoveryv1beta1 "k8s.io/api/discovery/v1beta1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EndpointSliceReconciler reconciles a EndpointSlice object
type EndpointSliceReconciler struct {
	client.Client
	Log          logr.Logger
	Scheme       *runtime.Scheme
	RecordsCache *DNSCache
}

// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslice,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslice/status,verbs=get;update;patch

func (r *EndpointSliceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("endpointslice", req.NamespacedName)

	var endpointSlice discoveryv1beta1.EndpointSlice
	if err := r.Client.Get(ctx, req.NamespacedName, &endpointSlice); err != nil {
		if k8serrors.IsNotFound(err) {
			r.RecordsCache.DeleteByResourceKey(req.String())
			return ctrl.Result{}, nil
		} else {
			log.Error(err, "Failed to get")
			return ctrl.Result{}, err
		}
	}

	fqdn, ok := endpointSlice.Annotations[connectivityv1alpha1.DNSHostnameAnnotation]
	if !ok {
		return ctrl.Result{}, nil
	}

	if endpointSlice.AddressType != discoveryv1beta1.AddressTypeIPv4 {
		log.Info("Skipping non-IPv4 EndpointSlice")
		return ctrl.Result{}, nil
	}

	ips := []net.IP{}
	for _, endpoint := range endpointSlice.Endpoints {
		for _, address := range endpoint.Addresses {
			ip := net.ParseIP(address)
			ips = append(ips, ip)
		}
	}

	r.RecordsCache.Upsert(DNSCacheEntry{
		ResourceKey: req.String(),
		FQDN:        fqdn,
		IPs:         ips,
	})
	log.WithValues("dns-hostname", fqdn).Info("Successfully synced")

	return ctrl.Result{}, nil
}

func (r *EndpointSliceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1beta1.EndpointSlice{}).
		Complete(r)
}
