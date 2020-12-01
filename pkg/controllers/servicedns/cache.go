// Copyright (c) 2020 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package servicedns

import (
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/miekg/dns"
)

// DNSCacheEntry stores information on the service associated with the FQDN
type DNSCacheEntry struct {
	ServiceKey       string
	EndpointSliceKey string
	FQDN             string
	IPs              []net.IP
}

// DNSCache maps Domain Name -> DNSCacheEntry
type DNSCache struct {
	mutex       sync.RWMutex
	entries     map[string]DNSCacheEntry
	keyToFQDN   map[string]string
	isPopulated bool
}

func (e *DNSCacheEntry) GetKey() string {
	if len(e.ServiceKey) > 0 {
		return fmt.Sprintf("service/%s", e.ServiceKey)
	}
	if len(e.EndpointSliceKey) > 0 {
		return fmt.Sprintf("endpointslice/%s", e.EndpointSliceKey)
	}
	return ""
}

// Upsert updates or inserts the DNSCacheEntry in the cache
func (d *DNSCache) Upsert(entry DNSCacheEntry) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.entries == nil || d.keyToFQDN == nil {
		d.entries = make(map[string]DNSCacheEntry)
		d.keyToFQDN = make(map[string]string)
	}
	fqdn := dns.Fqdn(entry.FQDN)

	entryKey := entry.GetKey()

	if oldEntry, ok := d.entries[fqdn]; ok {
		oldEntryKey := oldEntry.GetKey()
		if oldEntryKey != entryKey {
			delete(d.keyToFQDN, oldEntryKey)
		}
	}
	if oldFQDN, ok := d.keyToFQDN[entryKey]; ok {
		if oldFQDN != fqdn {
			delete(d.entries, oldFQDN)
		}
	}
	d.entries[fqdn] = entry
	d.keyToFQDN[entryKey] = fqdn
}

// Delete removes the DNSCacheEntry associated with the provided FQDN
func (d *DNSCache) Delete(fqdn string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.entries == nil || d.keyToFQDN == nil {
		return
	}
	fqdn = dns.Fqdn(fqdn)
	if entryToDelete, ok := d.entries[fqdn]; ok {
		delete(d.entries, fqdn)
		delete(d.keyToFQDN, entryToDelete.GetKey())
	}
}

// DeleteByServiceKey removes the DNSCacheEntry associated with the service key
func (d *DNSCache) DeleteByServiceKey(serviceKey string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	tempEntry := DNSCacheEntry{ServiceKey: serviceKey}
	entryKey := tempEntry.GetKey()

	if d.entries == nil || d.keyToFQDN == nil {
		return
	}
	if fqdnToDelete, ok := d.keyToFQDN[entryKey]; ok {
		delete(d.entries, fqdnToDelete)
		delete(d.keyToFQDN, entryKey)
	}
}

// DeleteByEndpointSliceKey removes the DNSCacheEntry associated with the endpoint slice key
func (d *DNSCache) DeleteByEndpointSliceKey(endpointSliceKey string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	tempEntry := DNSCacheEntry{EndpointSliceKey: endpointSliceKey}
	entryKey := tempEntry.GetKey()

	if d.entries == nil || d.keyToFQDN == nil {
		return
	}
	if fqdnToDelete, ok := d.keyToFQDN[entryKey]; ok {
		delete(d.entries, fqdnToDelete)
		delete(d.keyToFQDN, entryKey)
	}
}

// Lookup retrieves the DNSCacheEntry associated with the provided FQDN
func (d *DNSCache) Lookup(fqdn string) *DNSCacheEntry {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	if d.entries == nil || d.keyToFQDN == nil {
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
