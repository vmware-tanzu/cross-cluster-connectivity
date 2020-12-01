// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package servicedns_test

import (
	"net"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vmware-tanzu/cross-cluster-connectivity/pkg/controllers/servicedns"
)

var _ = Describe("DNSCache", func() {
	Describe("Upsert", func() {
		It("inserts the cache entry if it does not exist", func() {
			cache := new(servicedns.DNSCache)
			Expect(cache.Lookup("a.b.c")).To(BeNil())

			dnsCacheEntry := servicedns.DNSCacheEntry{
				ServiceKey: "12345-abc",
				FQDN:       "a.b.c",
				IPs:        []net.IP{net.ParseIP("1.2.3.4")},
			}
			cache.Upsert(dnsCacheEntry)

			Expect(cache.Lookup("a.b.c")).To(Equal(&dnsCacheEntry))
		})

		Context("if the cache entry does exist and the key changes", func() {
			var (
				cache    *servicedns.DNSCache
				oldEntry servicedns.DNSCacheEntry
			)

			BeforeEach(func() {
				cache = new(servicedns.DNSCache)
				oldEntry = servicedns.DNSCacheEntry{
					ServiceKey: "12345-abc-old",
					FQDN:       "a.b.c",
					IPs:        []net.IP{net.ParseIP("1.2.3.4")},
				}
				cache.Upsert(oldEntry)
			})

			It("updates the cache entry", func() {
				Expect(cache.Lookup("a.b.c")).To(Equal(&oldEntry))

				newEntry := servicedns.DNSCacheEntry{
					ServiceKey: "12345-abc",
					FQDN:       "a.b.c",
					IPs:        []net.IP{net.ParseIP("4.5.6.7")},
				}
				cache.Upsert(newEntry)
				Expect(cache.Lookup("a.b.c")).To(Equal(&newEntry))
			})
		})

		Context("if the cache entry does exist and the fqdn changes", func() {
			var (
				cache    *servicedns.DNSCache
				oldEntry servicedns.DNSCacheEntry
			)

			BeforeEach(func() {
				cache = new(servicedns.DNSCache)
				oldEntry = servicedns.DNSCacheEntry{
					ServiceKey: "12345-abc",
					FQDN:       "a.b.c",
					IPs:        []net.IP{net.ParseIP("1.2.3.4")},
				}
				cache.Upsert(oldEntry)
			})

			It("updates the cache entry", func() {
				Expect(cache.Lookup("a.b.c")).To(Equal(&oldEntry))
				Expect(cache.Lookup("b.c.d")).To(BeNil())

				newEntry := servicedns.DNSCacheEntry{
					ServiceKey: "12345-abc",
					FQDN:       "b.c.d",
					IPs:        []net.IP{net.ParseIP("4.5.6.7")},
				}
				cache.Upsert(newEntry)
				Expect(cache.Lookup("a.b.c")).To(BeNil())
				Expect(cache.Lookup("b.c.d")).To(Equal(&newEntry))
			})
		})

		Context("if the domain name has a wildcard", func() {
			It("can lookup any fqdn that matches the wildcard", func() {
				cache := new(servicedns.DNSCache)
				dnsCacheEntry := servicedns.DNSCacheEntry{
					EndpointSliceKey: "12345-abc",
					FQDN:             "*.b.c",
					IPs:              []net.IP{net.ParseIP("1.2.3.4")},
				}
				cache.Upsert(dnsCacheEntry)

				Expect(cache.Lookup("*.b.c")).To(Equal(&dnsCacheEntry))
				Expect(cache.Lookup("foo.b.c")).To(Equal(&dnsCacheEntry))
				Expect(cache.Lookup("bar.b.c")).To(Equal(&dnsCacheEntry))
				Expect(cache.Lookup("foo.bar.b.c")).To(Equal(&dnsCacheEntry))
				Expect(cache.Lookup("foo.bar.baz.b.c")).To(Equal(&dnsCacheEntry))
			})
		})
	})

	Describe("Delete", func() {
		Context("if the cache entry does exist", func() {
			var (
				cache    *servicedns.DNSCache
				oldEntry servicedns.DNSCacheEntry
			)

			BeforeEach(func() {
				cache = new(servicedns.DNSCache)
				oldEntry = servicedns.DNSCacheEntry{
					ServiceKey: "12345-abc-old",
					FQDN:       "a.b.c",
					IPs:        []net.IP{net.ParseIP("1.2.3.4")},
				}
				cache.Upsert(oldEntry)
			})

			It("deletes the cache entry if it does exist", func() {
				Expect(cache.Lookup("a.b.c")).To(Equal(&oldEntry))

				cache.Delete("a.b.c")

				Expect(cache.Lookup("a.b.c")).To(BeNil())
			})
		})

		It("does nothing if the entry does not exist", func() {
			cache := new(servicedns.DNSCache)

			Expect(cache.Lookup("b.c.d")).To(BeNil())
			cache.Delete("b.c.d")
			Expect(cache.Lookup("b.c.d")).To(BeNil())
		})
	})

	Describe("DeleteByServiceKey", func() {
		Context("if the cache entry does exist", func() {
			var (
				cache *servicedns.DNSCache
				entry servicedns.DNSCacheEntry
			)

			BeforeEach(func() {
				cache = new(servicedns.DNSCache)
				entry = servicedns.DNSCacheEntry{
					ServiceKey: "12345-abc",
					FQDN:       "a.b.c",
					IPs:        []net.IP{net.ParseIP("1.2.3.4")},
				}
				cache.Upsert(entry)
			})

			It("deletes the cache entry if it does exist", func() {
				Expect(cache.Lookup("a.b.c")).To(Equal(&entry))

				cache.DeleteByServiceKey("12345-abc")

				Expect(cache.Lookup("a.b.c")).To(BeNil())
			})

			When("the entry does not have an service key", func() {
				var otherEntry servicedns.DNSCacheEntry

				BeforeEach(func() {
					otherEntry = servicedns.DNSCacheEntry{
						EndpointSliceKey: entry.ServiceKey,
						FQDN:             "a.b.d",
						IPs:              []net.IP{net.ParseIP("1.2.3.5")},
					}
					cache.Upsert(otherEntry)
				})

				It("does not delete the cache entry by endpoint slice key", func() {
					Expect(cache.Lookup("a.b.d")).To(Equal(&otherEntry))

					cache.DeleteByServiceKey("12345-abc")

					Expect(cache.Lookup("a.b.d")).To(Equal(&otherEntry))
				})
			})
		})

		It("does nothing if the entry does not exist", func() {
			cache := new(servicedns.DNSCache)

			Expect(cache.Lookup("a.b.c")).To(BeNil())
			cache.DeleteByServiceKey("12345-abc")
			Expect(cache.Lookup("a.b.c")).To(BeNil())
		})
	})

	Describe("DeleteByEndpointSliceKey", func() {
		Context("if the cache entry does exist", func() {
			var (
				cache *servicedns.DNSCache
				entry servicedns.DNSCacheEntry
			)

			BeforeEach(func() {
				cache = new(servicedns.DNSCache)
				entry = servicedns.DNSCacheEntry{
					EndpointSliceKey: "12345-abc",
					FQDN:             "a.b.c",
					IPs:              []net.IP{net.ParseIP("1.2.3.4")},
				}
				cache.Upsert(entry)
			})

			It("deletes the cache entry if it does exist", func() {
				Expect(cache.Lookup("a.b.c")).To(Equal(&entry))

				cache.DeleteByEndpointSliceKey("12345-abc")

				Expect(cache.Lookup("a.b.c")).To(BeNil())
			})

			When("the entry does not have an endpoint slice key", func() {
				var otherEntry servicedns.DNSCacheEntry

				BeforeEach(func() {
					otherEntry = servicedns.DNSCacheEntry{
						ServiceKey: entry.EndpointSliceKey,
						FQDN:       "a.b.d",
						IPs:        []net.IP{net.ParseIP("1.2.3.5")},
					}
					cache.Upsert(otherEntry)
				})

				It("does not delete the cache entry by endpoint slice key", func() {
					Expect(cache.Lookup("a.b.d")).To(Equal(&otherEntry))

					cache.DeleteByEndpointSliceKey("12345-abc")

					Expect(cache.Lookup("a.b.d")).To(Equal(&otherEntry))
				})
			})
		})

		It("does nothing if the entry does not exist", func() {
			cache := new(servicedns.DNSCache)

			Expect(cache.Lookup("a.b.c")).To(BeNil())
			cache.DeleteByEndpointSliceKey("12345-abc-foo")
			Expect(cache.Lookup("a.b.c")).To(BeNil())
		})
	})

	Describe("IsPopulated", func() {
		It("returns true when the populated flag is set", func() {
			cache := new(servicedns.DNSCache)
			Expect(cache.IsPopulated()).To(BeFalse())
			cache.SetPopulated()
			Expect(cache.IsPopulated()).To(BeTrue())
		})
	})
})

var _ = Describe("DNSCacheEntry", func() {
	var entry servicedns.DNSCacheEntry

	Describe("GetKey", func() {
		When("only a service key is provided", func() {
			BeforeEach(func() {
				entry.ServiceKey = "namespace/service-name"
				entry.EndpointSliceKey = ""
			})

			It("returns the service key", func() {
				Expect(entry.GetKey()).To(Equal("service/namespace/service-name"))
			})
		})
		When("only an endpoint slice key is provided", func() {
			BeforeEach(func() {
				entry.ServiceKey = ""
				entry.EndpointSliceKey = "namespace/endpoint-slice-name"
			})

			It("returns the endpoint slice key", func() {
				Expect(entry.GetKey()).To(Equal("endpointslice/namespace/endpoint-slice-name"))
			})
		})
		When("both a service key and an endpoint slice key are provided", func() {
			BeforeEach(func() {
				entry.ServiceKey = "namespace/service-name"
				entry.EndpointSliceKey = "namespace/endpoint-slice-name"
			})

			It("returns the service key", func() {
				Expect(entry.GetKey()).To(Equal("service/namespace/service-name"))
			})
		})

		When("neither a service key nor an endpoint slice key are provided", func() {
			BeforeEach(func() {
				entry.ServiceKey = ""
				entry.EndpointSliceKey = ""
			})

			It("returns an empty string", func() {
				Expect(entry.GetKey()).To(Equal(""))
			})
		})
	})
})
