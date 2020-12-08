// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package crosscluster

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/request"
	"github.com/go-logr/logr"
	"github.com/miekg/dns"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/endpointslicedns"
)

type CrossCluster struct {
	Next plugin.Handler

	RecordsCache *endpointslicedns.DNSCache
	Zones        []string
	Log          logr.Logger
}

var errNotImplemented = errors.New("not implemented")

var errNameNotFound = errors.New("name not found")

// Services communicates with the backend to retrieve the service definitions. Exact indicates
// on exact match should be returned.
func (c *CrossCluster) Services(ctx context.Context, state request.Request, exact bool, opt plugin.Options) ([]msg.Service, error) {
	fqdn := strings.ToLower(state.QName())

	cacheEntry := c.RecordsCache.Lookup(fqdn)
	if cacheEntry == nil {
		return nil, errNameNotFound
	}

	services := []msg.Service{}
	for _, ip := range cacheEntry.IPs {
		services = append(services, msg.Service{
			Host: ip.String(),
			TTL:  30,
		})
	}

	return services, nil
}

// Reverse communicates with the backend to retrieve service definition based on a IP address
// instead of a name. I.e. a reverse DNS lookup.
func (c *CrossCluster) Reverse(ctx context.Context, state request.Request, exact bool, opt plugin.Options) ([]msg.Service, error) {
	c.Log.Info("not implemented: reverse")
	return nil, errNotImplemented
}

// Lookup is used to find records else where.
func (c *CrossCluster) Lookup(ctx context.Context, state request.Request, name string, typ uint16) (*dns.Msg, error) {
	c.Log.Info("not implemented: lookup")
	return nil, errNotImplemented
}

// Returns _all_ services that matches a certain name.
// Note: it does not implement a specific service.
func (c *CrossCluster) Records(ctx context.Context, state request.Request, exact bool) ([]msg.Service, error) {
	c.Log.Info("not implemented: records")
	return nil, errNotImplemented
}

// IsNameError returns true if err indicated a record not found condition
func (c *CrossCluster) IsNameError(err error) bool {
	return err == errNameNotFound
}

// Serial returns a SOA serial number to construct a SOA record.
func (c *CrossCluster) Serial(state request.Request) uint32 {
	return uint32(time.Now().Unix())
}

// MinTTL returns the minimum TTL to be used in the SOA record.
func (c *CrossCluster) MinTTL(state request.Request) uint32 {
	return 30
}

// Transfer handles a zone transfer it writes to the client just
// like any other handler.
func (c *CrossCluster) Transfer(ctx context.Context, state request.Request) (int, error) {
	return dns.RcodeServerFailure, nil
}
