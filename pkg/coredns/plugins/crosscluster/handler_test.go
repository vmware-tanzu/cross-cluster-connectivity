// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package crosscluster_test

import (
	"context"
	"net"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"
	"github.com/miekg/dns"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/servicedns"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/coredns/plugins/crosscluster"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("CrossCluster", func() {
	Describe("ServeDNS", func() {
		var (
			dnsCache  *servicedns.DNSCache
			dnsPlugin *crosscluster.CrossCluster
		)

		BeforeEach(func() {
			dnsCache = &servicedns.DNSCache{}
			dnsPlugin = &crosscluster.CrossCluster{
				RecordsCache: dnsCache,
				Zones:        []string{"some.domain."},
			}

			dnsCache.Upsert(servicedns.DNSCacheEntry{
				ServiceKey: "some-namespace/some-service",
				FQDN:       "some-service.some.domain",
				IP:         net.ParseIP("1.2.3.4"),
			})

			dnsCache.Upsert(servicedns.DNSCacheEntry{
				ServiceKey: "some-namespace/another-service",
				FQDN:       "another-service.some.domain",
				IP:         net.ParseIP("2.3.4.5"),
			})
		})

		DescribeTable("returns an appropriate DNS response given an A record dns request", func(fqdn string, expectedIP net.IP) {
			r := new(dns.Msg)
			r.SetQuestion(dns.Fqdn(fqdn), dns.TypeA)
			w := dnstest.NewRecorder(&test.ResponseWriter{})

			dnsPlugin.ServeDNS(context.Background(), w, r)
			Expect(w.Msg.Answer).To(HaveLen(1))
			Expect(w.Msg.Answer[0].(*dns.A).Hdr).To(Equal(dns.RR_Header{
				Name:   dns.Fqdn(fqdn),
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    30,
			}))
			Expect(w.Msg.Answer[0].(*dns.A).A).To(Equal(expectedIP))
		},
			Entry("returns an A record with the correct IP for some-service", "some-service.some.domain", net.ParseIP("1.2.3.4").To4()),
			Entry("returns an A record with the correct IP for another-service", "another-service.some.domain", net.ParseIP("2.3.4.5").To4()),
			Entry("handles case-insensitivity", "ANOTHER-SERVICE.some.domain", net.ParseIP("2.3.4.5").To4()),
		)

		Context("when the FQDN provided is not in the cache", func() {
			It("returns a DNS message NXDOMAIN", func() {
				r := new(dns.Msg)
				r.SetQuestion(dns.Fqdn("not-exists.some.domain"), dns.TypeA)
				w := dnstest.NewRecorder(&test.ResponseWriter{})
				dnsPlugin.ServeDNS(context.Background(), w, r)

				Expect(w.Msg.Rcode).To(Equal(dns.RcodeNameError))
			})
		})

		Context("when the dns request asks for non A record", func() {
			It("returns a DNS message NXDOMAIN", func() {
				r := new(dns.Msg)
				r.SetQuestion(dns.Fqdn("some-service.some.domain"), dns.TypeAAAA)
				w := dnstest.NewRecorder(&test.ResponseWriter{})
				dnsPlugin.ServeDNS(context.Background(), w, r)

				Expect(w.Msg.Rcode).To(Equal(dns.RcodeNameError))
			})
		})
	})
})
