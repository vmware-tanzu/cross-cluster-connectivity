// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package endpointslicedns

import (
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/miekg/dns"
)

// DNSCacheEntry stores information on the resource associated with the FQDN
type DNSCacheEntry struct {
	ResourceKey string
	FQDN        string
	IPs         []net.IP
}

// DNSCache maps Domain Name -> DNSCacheEntry
type DNSCache struct {
	mutex             sync.RWMutex
	entries           map[string]DNSCacheEntry
	resourceKeyToFQDN map[string]string
	isPopulated       bool
}

// Upsert updates or inserts the DNSCacheEntry in the cache
func (d *DNSCache) Upsert(entry DNSCacheEntry) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.entries == nil || d.resourceKeyToFQDN == nil {
		d.entries = make(map[string]DNSCacheEntry)
		d.resourceKeyToFQDN = make(map[string]string)
	}
	fqdn := dns.Fqdn(entry.FQDN)
	if oldEntry, ok := d.entries[fqdn]; ok {
		if oldEntry.ResourceKey != entry.ResourceKey {
			delete(d.resourceKeyToFQDN, oldEntry.ResourceKey)
		}
	}
	if oldFQDN, ok := d.resourceKeyToFQDN[entry.ResourceKey]; ok {
		if oldFQDN != fqdn {
			delete(d.entries, oldFQDN)
		}
	}
	d.entries[fqdn] = entry
	d.resourceKeyToFQDN[entry.ResourceKey] = fqdn
}

// Delete removes the DNSCacheEntry associated with the provided FQDN
func (d *DNSCache) Delete(fqdn string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.entries == nil || d.resourceKeyToFQDN == nil {
		return
	}
	fqdn = dns.Fqdn(fqdn)
	if entryToDelete, ok := d.entries[fqdn]; ok {
		delete(d.entries, fqdn)
		delete(d.resourceKeyToFQDN, entryToDelete.ResourceKey)
	}
}

// DeleteByResourceKey removes the DNSCacheEntry associated with the resource key
func (d *DNSCache) DeleteByResourceKey(resourceKey string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.entries == nil || d.resourceKeyToFQDN == nil {
		return
	}
	if fqdnToDelete, ok := d.resourceKeyToFQDN[resourceKey]; ok {
		delete(d.entries, fqdnToDelete)
		delete(d.resourceKeyToFQDN, resourceKey)
	}
}

// Lookup retrieves the DNSCacheEntry associated with the provided FQDN
func (d *DNSCache) Lookup(fqdn string) *DNSCacheEntry {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	if d.entries == nil || d.resourceKeyToFQDN == nil {
		return nil
	}

	fqdn = dns.Fqdn(fqdn)
	if e, ok := d.entries[fqdn]; ok {
		return &e
	}

	labels := dns.SplitDomainName(fqdn)
	for len(labels) > 0 {
		nextLookup := fmt.Sprintf("*.%s.", strings.Join(labels[1:], "."))
		if e, ok := d.entries[nextLookup]; ok {
			return &e
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
		if e, ok := d.entries[fqdn]; ok {
			return &e
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
