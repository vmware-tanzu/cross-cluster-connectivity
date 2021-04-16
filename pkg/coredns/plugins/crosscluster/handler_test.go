// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package crosscluster_test

import (
	"context"
	"fmt"
	"net"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"
	"github.com/miekg/dns"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/endpointslicedns"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/coredns/plugins/crosscluster"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("CrossCluster", func() {
	Describe("ServeDNS", func() {
		var (
			dnsCache  *endpointslicedns.DNSCache
			dnsPlugin *crosscluster.CrossCluster
		)

		BeforeEach(func() {
			ctrl.SetLogger(zap.New(
				zap.UseDevMode(true),
				zap.WriteTo(GinkgoWriter),
			))

			dnsCache = &endpointslicedns.DNSCache{}
			dnsPlugin = &crosscluster.CrossCluster{
				RecordsCache: dnsCache,
				Zones:        []string{"some.domain.", "other.domain."},
				Log:          ctrl.Log.WithName("dnsserver"),
			}

			dnsCache.Upsert(endpointslicedns.DNSCacheEntry{
				ResourceKey: "some-namespace/some-service",
				FQDN:        "some-service.some.domain",
				Addresses:   []string{"1.2.3.4", "1.2.3.5"},
			})

			dnsCache.Upsert(endpointslicedns.DNSCacheEntry{
				ResourceKey: "some-namespace/another-service",
				FQDN:        "another-service.some.domain",
				Addresses:   []string{"2.3.4.5"},
			})

			dnsCache.Upsert(endpointslicedns.DNSCacheEntry{
				ResourceKey: "other-namespace/some-service",
				FQDN:        "some-service.other.domain",
				Addresses:   []string{"foo.com", "bar.com"},
			})

			dnsCache.Upsert(endpointslicedns.DNSCacheEntry{
				ResourceKey: "other-namespace/another-service",
				FQDN:        "another-service.other.domain",
				Addresses:   []string{"baz.com"},
			})
		})

		DescribeTable("returns an appropriate DNS response given an A record dns request", func(fqdn string, expectedIPs ...net.IP) {
			r := new(dns.Msg)
			r.SetQuestion(dns.Fqdn(fqdn), dns.TypeA)
			w := dnstest.NewRecorder(&test.ResponseWriter{})

			dnsPlugin.ServeDNS(context.Background(), w, r)

			Expect(w.Msg).ToNot(BeNil())

			var answerIPs []net.IP
			for i, answer := range w.Msg.Answer {
				aRecord := answer.(*dns.A)
				Expect(aRecord.Hdr).To(Equal(dns.RR_Header{
					Name:   dns.Fqdn(fqdn),
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    30,
				}), fmt.Sprintf("Mismatch at index %d", i))
				answerIPs = append(answerIPs, aRecord.A)
			}
			Expect(answerIPs).To(ConsistOf(expectedIPs))
		},
			Entry("returns an A record with the correct IP for some-service", "some-service.some.domain", net.ParseIP("1.2.3.4").To4(), net.ParseIP("1.2.3.5").To4()),
			Entry("returns an A record with the correct IP for another-service", "another-service.some.domain", net.ParseIP("2.3.4.5").To4()),
			Entry("handles case-insensitivity", "ANOTHER-SERVICE.some.domain", net.ParseIP("2.3.4.5").To4()),
		)

		DescribeTable("returns an appropriate DNS response given a CNAME record dns request", func(fqdn string, expectedTarget string) {
			r := new(dns.Msg)
			r.SetQuestion(dns.Fqdn(fqdn), dns.TypeCNAME)
			w := dnstest.NewRecorder(&test.ResponseWriter{})

			dnsPlugin.ServeDNS(context.Background(), w, r)

			Expect(w.Msg).ToNot(BeNil())

			var answerTargets []string
			for i, answer := range w.Msg.Answer {
				cnameRecord := answer.(*dns.CNAME)
				Expect(cnameRecord.Hdr).To(Equal(dns.RR_Header{
					Name:   dns.Fqdn(fqdn),
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
					Ttl:    30,
				}), fmt.Sprintf("Mismatch at index %d", i))
				answerTargets = append(answerTargets, cnameRecord.Target)
			}
			// CNAME records can only map to one host (RFC 1034, Section 3.6.2)
			Expect(answerTargets).To(ConsistOf(expectedTarget))
		},

			Entry("returns a CNAME record with the correct targets for some-service", "some-service.other.domain", "foo.com."),
			Entry("returns a CNAME record with the correct target for another-service", "another-service.other.domain", "baz.com."),
			Entry("handles case-insensitivity", "ANOTHER-SERVICE.other.domain", "baz.com."),
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

		Context("when the dns request asks for record that is not type A or CNAME", func() {
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
