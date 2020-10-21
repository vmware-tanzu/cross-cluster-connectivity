// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package servicedns

import (
	"net"
	"sync"

	"github.com/miekg/dns"
)

// DNSCacheEntry stores information on the service associated with the FQDN
type DNSCacheEntry struct {
	ServiceKey string
	FQDN       string
	IP         net.IP
}

// DNSCache maps Domain Name -> DNSCacheEntry
type DNSCache struct {
	mutex            sync.RWMutex
	entries          map[string]DNSCacheEntry
	serviceKeyToFQDN map[string]string
	isPopulated      bool
}

// Upsert updates or inserts the DNSCacheEntry in the cache
func (d *DNSCache) Upsert(entry DNSCacheEntry) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.entries == nil || d.serviceKeyToFQDN == nil {
		d.entries = make(map[string]DNSCacheEntry)
		d.serviceKeyToFQDN = make(map[string]string)
	}
	fqdn := dns.Fqdn(entry.FQDN)
	if oldEntry, ok := d.entries[fqdn]; ok {
		if oldEntry.ServiceKey != entry.ServiceKey {
			delete(d.serviceKeyToFQDN, oldEntry.ServiceKey)
		}
	}
	if oldFQDN, ok := d.serviceKeyToFQDN[entry.ServiceKey]; ok {
		if oldFQDN != fqdn {
			delete(d.entries, oldFQDN)
		}
	}
	d.entries[fqdn] = entry
	d.serviceKeyToFQDN[entry.ServiceKey] = fqdn
}

// Delete removes the DNSCacheEntry associated with the provided FQDN
func (d *DNSCache) Delete(fqdn string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.entries == nil || d.serviceKeyToFQDN == nil {
		return
	}
	fqdn = dns.Fqdn(fqdn)
	if entryToDelete, ok := d.entries[fqdn]; ok {
		delete(d.entries, fqdn)
		delete(d.serviceKeyToFQDN, entryToDelete.ServiceKey)
	}
}

// DeleteByServiceKey removes the DNSCacheEntry associated with the service key
func (d *DNSCache) DeleteByServiceKey(serviceKey string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.entries == nil || d.serviceKeyToFQDN == nil {
		return
	}
	if fqdnToDelete, ok := d.serviceKeyToFQDN[serviceKey]; ok {
		delete(d.entries, fqdnToDelete)
		delete(d.serviceKeyToFQDN, serviceKey)
	}
}

// Lookup retrieves the DNSCacheEntry associated with the provided FQDN
func (d *DNSCache) Lookup(fqdn string) *DNSCacheEntry {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	if d.entries == nil || d.serviceKeyToFQDN == nil {
		return nil
	}

	fqdn = dns.Fqdn(fqdn)
	if e, ok := d.entries[fqdn]; ok {
		return &e
	}
	return nil
}

// LookupByServiceKey retrieves the DNSCacheEntry associated with the service key
func (d *DNSCache) LookupByServiceKey(serviceKey string) *DNSCacheEntry {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	if d.entries == nil || d.serviceKeyToFQDN == nil {
		return nil
	}

	if fqdn, ok := d.serviceKeyToFQDN[serviceKey]; ok {
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
