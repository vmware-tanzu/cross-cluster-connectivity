// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package endpointslicedns

import (
	"fmt"
	"strings"
	"sync"

	"github.com/miekg/dns"
)

// DNSCacheEntry stores information on the resource associated with the FQDN
type DNSCacheEntry struct {
	ResourceKey string
	FQDN        string
	Addresses   []string
}

// DNSCache maps Domain Name -> DNSCacheEntry
type DNSCache struct {
	mutex             sync.RWMutex
	entries           map[string][]DNSCacheEntry
	resourceKeyToFQDN map[string]string
	isPopulated       bool
}

// Upsert updates or inserts the DNSCacheEntry in the cache
func (d *DNSCache) Upsert(entry DNSCacheEntry) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.entries == nil || d.resourceKeyToFQDN == nil {
		d.entries = make(map[string][]DNSCacheEntry)
		d.resourceKeyToFQDN = make(map[string]string)
	}
	fqdn := dns.Fqdn(entry.FQDN)

	if oldFQDN, ok := d.resourceKeyToFQDN[entry.ResourceKey]; ok {
		if oldFQDN != fqdn {
			for i, oldEntry := range d.entries[oldFQDN] {
				if oldEntry.ResourceKey == entry.ResourceKey {
					d.entries[oldFQDN] = append(d.entries[oldFQDN][:i], d.entries[oldFQDN][i+1:]...)
					break
				}
			}
			if len(d.entries[oldFQDN]) == 0 {
				delete(d.entries, oldFQDN)
			}
		}
	}

	updated := false
	for i, e := range d.entries[fqdn] {
		if e.ResourceKey == entry.ResourceKey {
			d.entries[fqdn][i] = entry
			updated = true
			break
		}
	}
	if !updated {
		d.entries[fqdn] = append(d.entries[fqdn], entry)
	}
	d.resourceKeyToFQDN[entry.ResourceKey] = fqdn
}

// Delete removes the DNSCacheEntries associated with the provided FQDN
func (d *DNSCache) Delete(fqdn string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.entries == nil || d.resourceKeyToFQDN == nil {
		return
	}
	fqdn = dns.Fqdn(fqdn)
	for _, entry := range d.entries[fqdn] {
		delete(d.resourceKeyToFQDN, entry.ResourceKey)
	}
	delete(d.entries, fqdn)
}

// DeleteByResourceKey removes the DNSCacheEntry associated with the resource key
func (d *DNSCache) DeleteByResourceKey(resourceKey string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.entries == nil || d.resourceKeyToFQDN == nil {
		return
	}
	if fqdnToUpdate, ok := d.resourceKeyToFQDN[resourceKey]; ok {
		for i, entry := range d.entries[fqdnToUpdate] {
			if entry.ResourceKey == resourceKey {
				d.entries[fqdnToUpdate] = append(d.entries[fqdnToUpdate][:i], d.entries[fqdnToUpdate][i+1:]...)
				break
			}

			if len(d.entries[fqdnToUpdate]) == 0 {
				delete(d.entries, fqdnToUpdate)
			}
		}
		delete(d.resourceKeyToFQDN, resourceKey)
	}
}

// Lookup retrieves the DNSCacheEntries associated with the provided FQDN
func (d *DNSCache) Lookup(fqdn string) []DNSCacheEntry {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	if d.entries == nil || d.resourceKeyToFQDN == nil {
		return nil
	}

	fqdn = dns.Fqdn(fqdn)
	if e, ok := d.entries[fqdn]; ok {
		return e
	}

	labels := dns.SplitDomainName(fqdn)
	for len(labels) > 0 {
		nextLookup := fmt.Sprintf("*.%s.", strings.Join(labels[1:], "."))
		if e, ok := d.entries[nextLookup]; ok {
			return e
		}
		labels = labels[1:]
	}

	return nil
}

// LookupByResourceKey retrieves the DNSCacheEntry associated with the resource key
func (d *DNSCache) LookupByResourceKey(resourceKey string) *DNSCacheEntry {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	if d.entries == nil || d.resourceKeyToFQDN == nil {
		return nil
	}

	if fqdn, ok := d.resourceKeyToFQDN[resourceKey]; ok {
		for _, e := range d.entries[fqdn] {
			if e.ResourceKey == resourceKey {
				return &e
			}
		}
	}
	return nil
}

// IsPopulated returns true when the cache is fully populated
func (d *DNSCache) IsPopulated() bool {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	return d.isPopulated
}

// SetPopulated marks the cache as populated
func (d *DNSCache) SetPopulated() {
	d.mutex.Lock()
	d.isPopulated = true
	d.mutex.Unlock()
}
