// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package endpointslicedns_test

import (
	"net"

	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/endpointslicedns"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DNSCache", func() {
	Describe("Upsert", func() {
		It("inserts the cache entry if it does not exist", func() {
			cache := new(endpointslicedns.DNSCache)
			Expect(cache.Lookup("a.b.c")).To(BeNil())
			Expect(cache.LookupByResourceKey("12345-abc")).To(BeNil())

			dnsCacheEntry := endpointslicedns.DNSCacheEntry{
				ResourceKey: "12345-abc",
				FQDN:        "a.b.c",
				IPs:         []net.IP{net.ParseIP("1.2.3.4")},
			}
			cache.Upsert(dnsCacheEntry)

			Expect(cache.Lookup("a.b.c")).To(Equal(&dnsCacheEntry))
			Expect(cache.LookupByResourceKey("12345-abc")).To(Equal(&dnsCacheEntry))
		})

		Context("if the cache entry does exist and the resource key changes", func() {
			var (
				cache    *endpointslicedns.DNSCache
				oldEntry endpointslicedns.DNSCacheEntry
			)

			BeforeEach(func() {
				cache = new(endpointslicedns.DNSCache)
				oldEntry = endpointslicedns.DNSCacheEntry{
					ResourceKey: "12345-abc-old",
					FQDN:        "a.b.c",
					IPs:         []net.IP{net.ParseIP("1.2.3.4")},
				}
				cache.Upsert(oldEntry)
			})

			It("updates the cache entry", func() {
				Expect(cache.Lookup("a.b.c")).To(Equal(&oldEntry))
				Expect(cache.LookupByResourceKey("12345-abc")).To(BeNil())
				Expect(cache.LookupByResourceKey("12345-abc-old")).To(Equal(&oldEntry))

				newEntry := endpointslicedns.DNSCacheEntry{
					ResourceKey: "12345-abc",
					FQDN:        "a.b.c",
					IPs:         []net.IP{net.ParseIP("4.5.6.7")},
				}
				cache.Upsert(newEntry)
				Expect(cache.Lookup("a.b.c")).To(Equal(&newEntry))
				Expect(cache.LookupByResourceKey("12345-abc")).To(Equal(&newEntry))
				Expect(cache.LookupByResourceKey("12345-abc-old")).To(BeNil())
			})
		})

		Context("if the domain name has a wildcard", func() {
			It("can lookup any fqdn that matches the wildcard", func() {
				cache := new(endpointslicedns.DNSCache)
				dnsCacheEntry := endpointslicedns.DNSCacheEntry{
					ResourceKey: "12345-abc",
					FQDN:        "*.b.c",
					IPs:         []net.IP{net.ParseIP("1.2.3.4")},
				}
				cache.Upsert(dnsCacheEntry)

				Expect(cache.Lookup("*.b.c")).To(Equal(&dnsCacheEntry))
				Expect(cache.Lookup("foo.b.c")).To(Equal(&dnsCacheEntry))
				Expect(cache.Lookup("bar.b.c")).To(Equal(&dnsCacheEntry))
				Expect(cache.Lookup("foo.bar.b.c")).To(Equal(&dnsCacheEntry))
				Expect(cache.Lookup("foo.bar.baz.b.c")).To(Equal(&dnsCacheEntry))
			})
		})

		Context("if the cache entry does exist and the fqdn changes", func() {
			var (
				cache    *endpointslicedns.DNSCache
				oldEntry endpointslicedns.DNSCacheEntry
			)

			BeforeEach(func() {
				cache = new(endpointslicedns.DNSCache)
				oldEntry = endpointslicedns.DNSCacheEntry{
					ResourceKey: "12345-abc",
					FQDN:        "a.b.c",
					IPs:         []net.IP{net.ParseIP("1.2.3.4")},
				}
				cache.Upsert(oldEntry)
			})

			It("updates the cache entry", func() {
				Expect(cache.Lookup("a.b.c")).To(Equal(&oldEntry))
				Expect(cache.Lookup("b.c.d")).To(BeNil())
				Expect(cache.LookupByResourceKey("12345-abc")).To(Equal(&oldEntry))

				newEntry := endpointslicedns.DNSCacheEntry{
					ResourceKey: "12345-abc",
					FQDN:        "b.c.d",
					IPs:         []net.IP{net.ParseIP("4.5.6.7")},
				}
				cache.Upsert(newEntry)
				Expect(cache.Lookup("a.b.c")).To(BeNil())
				Expect(cache.Lookup("b.c.d")).To(Equal(&newEntry))
				Expect(cache.LookupByResourceKey("12345-abc")).To(Equal(&newEntry))
			})
		})
	})

	Describe("Delete", func() {
		Context("if the cache entry does exist", func() {
			var (
				cache    *endpointslicedns.DNSCache
				oldEntry endpointslicedns.DNSCacheEntry
			)

			BeforeEach(func() {
				cache = new(endpointslicedns.DNSCache)
				oldEntry = endpointslicedns.DNSCacheEntry{
					ResourceKey: "12345-abc-old",
					FQDN:        "a.b.c",
					IPs:         []net.IP{net.ParseIP("1.2.3.4")},
				}
				cache.Upsert(oldEntry)
			})

			It("deletes the cache entry if it does exist", func() {
				Expect(cache.Lookup("a.b.c")).To(Equal(&oldEntry))
				Expect(cache.LookupByResourceKey("12345-abc-old")).To(Equal(&oldEntry))

				cache.Delete("a.b.c")

				Expect(cache.Lookup("a.b.c")).To(BeNil())
				Expect(cache.LookupByResourceKey("12345-abc-old")).To(BeNil())
			})
		})

		It("does nothing if the entry does not exist", func() {
			cache := new(endpointslicedns.DNSCache)

			Expect(cache.Lookup("b.c.d")).To(BeNil())
			cache.Delete("b.c.d")
			Expect(cache.Lookup("b.c.d")).To(BeNil())
		})
	})

	Describe("DeleteByResourceKey", func() {
		Context("if the cache entry does exist", func() {
			var (
				cache    *endpointslicedns.DNSCache
				oldEntry endpointslicedns.DNSCacheEntry
			)

			BeforeEach(func() {
				cache = new(endpointslicedns.DNSCache)
				oldEntry = endpointslicedns.DNSCacheEntry{
					ResourceKey: "12345-abc-old",
					FQDN:        "a.b.c",
					IPs:         []net.IP{net.ParseIP("1.2.3.4")},
				}
				cache.Upsert(oldEntry)
			})

			It("deletes the cache entry if it does exist", func() {
				Expect(cache.Lookup("a.b.c")).To(Equal(&oldEntry))
				Expect(cache.LookupByResourceKey("12345-abc-old")).To(Equal(&oldEntry))

				cache.DeleteByResourceKey("12345-abc-old")

				Expect(cache.Lookup("a.b.c")).To(BeNil())
				Expect(cache.LookupByResourceKey("12345-abc-old")).To(BeNil())
			})
		})

		It("does nothing if the entry does not exist", func() {
			cache := new(endpointslicedns.DNSCache)

			Expect(cache.LookupByResourceKey("12345-abc-foo")).To(BeNil())
			cache.DeleteByResourceKey("12345-abc-foo")
			Expect(cache.LookupByResourceKey("12345-abc-foo")).To(BeNil())
		})
	})

	Describe("IsPopulated", func() {
		It("returns true when the populated flag is set", func() {
			cache := new(endpointslicedns.DNSCache)
			Expect(cache.IsPopulated()).To(BeFalse())
			cache.SetPopulated()
			Expect(cache.IsPopulated()).To(BeTrue())
		})
	})
})
