// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dnsconfig

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DNSServiceWatcher struct {
	Client client.Client

	Namespace   string
	ServiceName string

	PollingInterval time.Duration
}

func (d *DNSServiceWatcher) GetDNSServiceClusterIP(ctx context.Context) (string, error) {
	for {
		var dnsService corev1.Service
		err := d.Client.Get(ctx, client.ObjectKey{
			Namespace: d.Namespace,
			Name:      d.ServiceName,
		}, &dnsService)

		if err == nil && dnsService.Spec.ClusterIP != "" {
			return dnsService.Spec.ClusterIP, nil
		}
		var helpfulError error
		if err != nil {
			helpfulError = err
		} else if dnsService.Spec.ClusterIP == "" {
			helpfulError = fmt.Errorf(`service "%s/%s" does not have a ClusterIP`, d.Namespace, d.ServiceName)
		}

		select {
		case <-ctx.Done():
			return "", fmt.Errorf(`Timed out obtaining ClusterIP from service "%s/%s": %s`, d.Namespace, d.ServiceName, helpfulError)
		case <-time.After(d.PollingInterval):
		}
	}
}
