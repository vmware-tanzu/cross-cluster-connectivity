// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package endpointslicedns

import (
	"context"
	"fmt"
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
			entry := r.RecordsCache.LookupByResourceKey(req.String())
			r.RecordsCache.DeleteByResourceKey(req.String())
			log.WithValues("dns-hostname", entry.FQDN).Info("Successfully deleted")
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

	addresses := []string{}

	if endpointSlice.AddressType == discoveryv1beta1.AddressTypeIPv4 {
		for _, endpoint := range endpointSlice.Endpoints {
			for _, address := range endpoint.Addresses {
				ip := net.ParseIP(address)
				if ip == nil {
					log.Error(fmt.Errorf("Invalid IP with AddressType IPv4: %s", address), "")
				} else {
					addresses = append(addresses, ip.String())
				}
			}
		}
	} else if endpointSlice.AddressType == discoveryv1beta1.AddressTypeFQDN {
		for _, endpoint := range endpointSlice.Endpoints {
			addresses = append(addresses, endpoint.Addresses...)
		}
	} else {
		log.Info("Skipping EndpointSlice with unhandled AddressType")
		return ctrl.Result{}, nil
	}

	r.RecordsCache.Upsert(DNSCacheEntry{
		ResourceKey: req.String(),
		FQDN:        fqdn,
		Addresses:   addresses,
	})
	log.WithValues("dns-hostname", fqdn).Info("Successfully synced")

	if !r.RecordsCache.IsValid(fqdn) {
		log.Error(
			fmt.Errorf(`DNS entry for "%s" is in an invalid state.`, fqdn),
			"If this FQDN is to resolve to a CNAME record, check to ensure any "+
				" FQDN EndpointSlice associated with this FQDN is the only "+
				"EndpointSlice annotated with this FQDN. Otherwise, if the FQDN is to "+
				"resolve to an A record, then ensure there are no FQDN EndpointSlices "+
				"annotated with this FQDN.",
		)
	}

	return ctrl.Result{}, nil
}

func (r *EndpointSliceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1beta1.EndpointSlice{}).
		Complete(r)
}
