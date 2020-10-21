// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package crosscluster

import (
	"context"
	"errors"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	log "github.com/sirupsen/logrus"
)

func (c *CrossCluster) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	opt := plugin.Options{}
	zone := plugin.Zones(c.Zones).Matches(state.QName())
	if zone == "" {
		return dns.RcodeServerFailure, errors.New("unknown zone")
	}

	var records []dns.RR
	var err error
	switch state.QType() {
	case dns.TypeSOA:
		records, err = plugin.SOA(ctx, c, zone, state, opt)
	case dns.TypeA:
		records, err = plugin.A(ctx, c, zone, state, nil, opt)
	default:
		return plugin.BackendError(ctx, c, zone, dns.RcodeNameError, state, nil, opt)
	}
	if err != nil {
		if c.IsNameError(err) {
			log.Info("couldn't find record in cache: ", state.Name())
			return plugin.BackendError(ctx, c, zone, dns.RcodeNameError, state, nil, opt)
		}
		log.Info("record lookup failed: ", err)
		return plugin.BackendError(ctx, c, zone, dns.RcodeServerFailure, state, err, opt)
	}

	if len(records) == 0 {
		return plugin.BackendError(ctx, c, zone, dns.RcodeNameError, state, nil, opt)
	}

	response := new(dns.Msg)
	response.SetReply(r)
	response.Authoritative = true
	response.Answer = append(response.Answer, records...)
	w.WriteMsg(response)

	return dns.RcodeSuccess, nil
}

func (c *CrossCluster) Name() string {
	return "crosscluster"
}
