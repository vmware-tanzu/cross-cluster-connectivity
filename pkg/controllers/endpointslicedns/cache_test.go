// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package endpointslicedns_test

import (
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/endpointslicedns"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("DNSCache", func() {
	Describe("Upsert", func() {
		It("inserts the cache entry if it does not exist", func() {
			cache := new(endpointslicedns.DNSCache)
			Expect(cache.Lookup("a.b.c")).To(BeEmpty())
			Expect(cache.LookupByResourceKey("12345-abc")).To(BeNil())

			dnsCacheEntry := endpointslicedns.DNSCacheEntry{
				ResourceKey: "12345-abc",
				FQDN:        "a.b.c.",
				Addresses:   []string{"1.2.3.4"},
			}
			cache.Upsert(dnsCacheEntry)

			Expect(cache.Lookup("a.b.c")).To(ConsistOf(dnsCacheEntry))
			Expect(cache.LookupByResourceKey("12345-abc")).To(Equal(&dnsCacheEntry))
		})

		It("canonicalizes non-ip addresses of the cache entry", func() {
			cache := new(endpointslicedns.DNSCache)
			Expect(cache.Lookup("a.b.c")).To(BeEmpty())
			Expect(cache.LookupByResourceKey("12345-abc")).To(BeNil())

			dnsCacheEntry := endpointslicedns.DNSCacheEntry{
				ResourceKey: "12345-abc",
				FQDN:        "a.b.c",
				Addresses:   []string{"FOO.com"},
			}
			cache.Upsert(dnsCacheEntry)

			expectedDNSCacheEntry := endpointslicedns.DNSCacheEntry{
				ResourceKey: "12345-abc",
				FQDN:        "a.b.c.",
				Addresses:   []string{"foo.com."},
			}

			Expect(cache.Lookup("a.b.c")).To(ConsistOf(expectedDNSCacheEntry))
			Expect(cache.LookupByResourceKey("12345-abc")).To(Equal(&expectedDNSCacheEntry))
		})

		Context("if cache entry has the FQDN which already exists in the cache but with different resource key", func() {
			var (
				cache     *endpointslicedns.DNSCache
				someEntry endpointslicedns.DNSCacheEntry
			)

			BeforeEach(func() {
				cache = new(endpointslicedns.DNSCache)
				someEntry = endpointslicedns.DNSCacheEntry{
					ResourceKey: "12345-some",
					FQDN:        "a.b.c.",
					Addresses:   []string{"1.2.3.4"},
				}
				cache.Upsert(someEntry)
			})

			It("returns both entries on lookup", func() {
				Expect(cache.Lookup("a.b.c")).To(ConsistOf(someEntry))
				Expect(cache.LookupByResourceKey("12345-some")).To(Equal(&someEntry))

				anotherEntry := endpointslicedns.DNSCacheEntry{
					ResourceKey: "12345-another",
					FQDN:        "a.b.c.",
					Addresses:   []string{"4.5.6.7"},
				}
				cache.Upsert(anotherEntry)
				Expect(cache.Lookup("a.b.c")).To(ConsistOf(someEntry, anotherEntry))
				Expect(cache.LookupByResourceKey("12345-some")).To(Equal(&someEntry))
				Expect(cache.LookupByResourceKey("12345-another")).To(Equal(&anotherEntry))
			})

			Context("when another entry is added that is impropery cased", func() {
				It("canonicalizes fqdn of the cache entry", func() {
					anotherDNSCacheEntry := endpointslicedns.DNSCacheEntry{
						ResourceKey: "12345-another",
						FQDN:        "A.B.C",
						Addresses:   []string{"2.3.4.5"},
					}
					cache.Upsert(anotherDNSCacheEntry)

					expectedAnotherDNSCacheEntry := endpointslicedns.DNSCacheEntry{
						ResourceKey: "12345-another",
						FQDN:        "a.b.c.",
						Addresses:   []string{"2.3.4.5"},
					}

					Expect(cache.Lookup("a.b.c")).To(ConsistOf(someEntry, expectedAnotherDNSCacheEntry))
					Expect(cache.LookupByResourceKey("12345-some")).To(Equal(&someEntry))
					Expect(cache.LookupByResourceKey("12345-another")).To(Equal(&expectedAnotherDNSCacheEntry))
				})
			})
		})

		Context("if the domain name has a wildcard", func() {
			It("can lookup any fqdn that matches the wildcard", func() {
				cache := new(endpointslicedns.DNSCache)
				dnsCacheEntry := endpointslicedns.DNSCacheEntry{
					ResourceKey: "12345-abc",
					FQDN:        "*.b.c.",
					Addresses:   []string{"1.2.3.4"},
				}
				cache.Upsert(dnsCacheEntry)

				Expect(cache.Lookup("*.b.c")).To(ConsistOf(dnsCacheEntry))
				Expect(cache.Lookup("foo.b.c")).To(ConsistOf(dnsCacheEntry))
				Expect(cache.Lookup("bar.b.c")).To(ConsistOf(dnsCacheEntry))
				Expect(cache.Lookup("foo.bar.b.c")).To(ConsistOf(dnsCacheEntry))
				Expect(cache.Lookup("foo.bar.baz.b.c")).To(ConsistOf(dnsCacheEntry))
			})
		})

		Context("if the cache entry does exist with the same resource key and the fqdn and ip changes", func() {
			var (
				cache    *endpointslicedns.DNSCache
				oldEntry endpointslicedns.DNSCacheEntry
			)

			BeforeEach(func() {
				cache = new(endpointslicedns.DNSCache)
				oldEntry = endpointslicedns.DNSCacheEntry{
					ResourceKey: "12345-abc",
					FQDN:        "a.b.c.",
					Addresses:   []string{"1.2.3.4"},
				}
				cache.Upsert(oldEntry)
			})

			It("updates the cache entry", func() {
				Expect(cache.Lookup("a.b.c")).To(ConsistOf(oldEntry))
				Expect(cache.Lookup("b.c.d")).To(BeEmpty())
				Expect(cache.LookupByResourceKey("12345-abc")).To(Equal(&oldEntry))

				newEntry := endpointslicedns.DNSCacheEntry{
					ResourceKey: "12345-abc",
					FQDN:        "b.c.d.",
					Addresses:   []string{"4.5.6.7"},
				}
				cache.Upsert(newEntry)
				Expect(cache.Lookup("a.b.c")).To(BeEmpty())
				Expect(cache.Lookup("b.c.d")).To(ConsistOf(newEntry))
				Expect(cache.LookupByResourceKey("12345-abc")).To(Equal(&newEntry))
			})
		})
	})

	Describe("Delete", func() {
		Context("if the cache entries do exist", func() {
			var (
				cache     *endpointslicedns.DNSCache
				oldEntry  endpointslicedns.DNSCacheEntry
				oldEntry2 endpointslicedns.DNSCacheEntry
			)

			BeforeEach(func() {
				cache = new(endpointslicedns.DNSCache)
				oldEntry = endpointslicedns.DNSCacheEntry{
					ResourceKey: "12345-abc-old",
					FQDN:        "a.b.c.",
					Addresses:   []string{"1.2.3.4"},
				}
				cache.Upsert(oldEntry)

				oldEntry2 = endpointslicedns.DNSCacheEntry{
					ResourceKey: "12345-abc-old-2",
					FQDN:        "a.b.c.",
					Addresses:   []string{"2.3.4.5"},
				}
				cache.Upsert(oldEntry2)
			})

			It("deletes the cache entries for a given fqdn", func() {
				Expect(cache.Lookup("a.b.c")).To(ConsistOf(oldEntry, oldEntry2))
				Expect(cache.LookupByResourceKey("12345-abc-old")).To(Equal(&oldEntry))
				Expect(cache.LookupByResourceKey("12345-abc-old-2")).To(Equal(&oldEntry2))

				cache.Delete("a.b.c")

				Expect(cache.Lookup("a.b.c")).To(BeEmpty())
				Expect(cache.LookupByResourceKey("12345-abc-old")).To(BeNil())
				Expect(cache.LookupByResourceKey("12345-abc-old-2")).To(BeNil())
			})
		})

		It("does nothing if the entry does not exist", func() {
			cache := new(endpointslicedns.DNSCache)

			Expect(cache.Lookup("b.c.d")).To(BeEmpty())
			cache.Delete("b.c.d")
			Expect(cache.Lookup("b.c.d")).To(BeEmpty())
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
					FQDN:        "a.b.c.",
					Addresses:   []string{"1.2.3.4"},
				}
				cache.Upsert(oldEntry)
			})

			It("deletes the cache entry", func() {
				Expect(cache.Lookup("a.b.c")).To(ConsistOf(oldEntry))
				Expect(cache.LookupByResourceKey("12345-abc-old")).To(Equal(&oldEntry))

				cache.DeleteByResourceKey("12345-abc-old")

				Expect(cache.Lookup("a.b.c")).To(BeEmpty())
				Expect(cache.LookupByResourceKey("12345-abc-old")).To(BeNil())
			})
		})

		Context("if there are multiple cache entries for the fqdn", func() {
			var (
				cache        *endpointslicedns.DNSCache
				oldEntry     endpointslicedns.DNSCacheEntry
				anotherEntry endpointslicedns.DNSCacheEntry
			)

			BeforeEach(func() {
				cache = new(endpointslicedns.DNSCache)
				oldEntry = endpointslicedns.DNSCacheEntry{
					ResourceKey: "12345-abc-old",
					FQDN:        "a.b.c.",
					Addresses:   []string{"1.2.3.4"},
				}
				cache.Upsert(oldEntry)

				anotherEntry = endpointslicedns.DNSCacheEntry{
					ResourceKey: "12345-abc-another",
					FQDN:        "a.b.c.",
					Addresses:   []string{"2.3.4.5"},
				}
				cache.Upsert(anotherEntry)
			})

			It("does not delete all the cache entries for the fqdn", func() {
				Expect(cache.Lookup("a.b.c")).To(ConsistOf(oldEntry, anotherEntry))
				Expect(cache.LookupByResourceKey("12345-abc-old")).To(Equal(&oldEntry))
				Expect(cache.LookupByResourceKey("12345-abc-another")).To(Equal(&anotherEntry))

				cache.DeleteByResourceKey("12345-abc-old")

				Expect(cache.Lookup("a.b.c")).To(ConsistOf(anotherEntry))
				Expect(cache.LookupByResourceKey("12345-abc-old")).To(BeNil())
				Expect(cache.LookupByResourceKey("12345-abc-another")).To(Equal(&anotherEntry))
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

	Describe("IsValid", func() {
		var (
			cache       *endpointslicedns.DNSCache
			ipEntry1    endpointslicedns.DNSCacheEntry
			ipEntry2    endpointslicedns.DNSCacheEntry
			cnameEntry1 endpointslicedns.DNSCacheEntry
			cnameEntry2 endpointslicedns.DNSCacheEntry
		)
		BeforeEach(func() {
			cache = new(endpointslicedns.DNSCache)
			ipEntry1 = endpointslicedns.DNSCacheEntry{
				ResourceKey: "12345-abc-one",
				FQDN:        "a.b.c.",
				Addresses:   []string{"1.2.3.4"},
			}

			ipEntry2 = endpointslicedns.DNSCacheEntry{
				ResourceKey: "12345-abc-two",
				FQDN:        "a.b.c.",
				Addresses:   []string{"2.3.4.5"},
			}

			cnameEntry1 = endpointslicedns.DNSCacheEntry{
				ResourceKey: "12345-abc-three",
				FQDN:        "a.b.c.",
				Addresses:   []string{"foo.com"},
			}

			cnameEntry2 = endpointslicedns.DNSCacheEntry{
				ResourceKey: "12345-abc-four",
				FQDN:        "a.b.c.",
				Addresses:   []string{"bar.com"},
			}
		})

		It("returns true when the DNS cache entry has only IP addresses", func() {
			cache.Upsert(ipEntry1)
			cache.Upsert(ipEntry2)
			Expect(cache.IsValid("a.b.c")).To(BeTrue())
		})

		It("returns true when the DNS cache entry has only one CNAME entry", func() {
			cache.Upsert(cnameEntry1)
			Expect(cache.IsValid("a.b.c")).To(BeTrue())
		})

		It("returns false when the DNS cache entry has both IP address and CNAME entries", func() {
			cache.Upsert(ipEntry1)
			cache.Upsert(cnameEntry1)
			Expect(cache.IsValid("a.b.c")).To(BeFalse())
		})

		It("returns false when the DNS cache entry has multiple CNAME entries", func() {
			cache.Upsert(cnameEntry1)
			cache.Upsert(cnameEntry2)
			Expect(cache.IsValid("a.b.c")).To(BeFalse())
		})

		It("returns false when the FQDN doesn't exist", func() {
			Expect(cache.IsValid("a.b.d")).To(BeFalse())
		})
	})
})
