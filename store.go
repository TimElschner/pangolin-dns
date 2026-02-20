package main

import (
	"sync"
)

// RecordStore holds DNS records in memory with thread-safe access.
// Records are swapped atomically on each poll cycle.
type RecordStore struct {
	mu      sync.RWMutex
	records map[string]string // FQDN (with trailing dot) → IP
}

func NewRecordStore() *RecordStore {
	return &RecordStore{
		records: make(map[string]string),
	}
}

// Update replaces all records atomically.
func (s *RecordStore) Update(records map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = records
}

// Lookup returns the IP for a given FQDN (with trailing dot).
func (s *RecordStore) Lookup(fqdn string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ip, ok := s.records[fqdn]
	return ip, ok
}

// Count returns the number of records.
func (s *RecordStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

// Domains returns all registered FQDNs for logging.
func (s *RecordStore) Domains() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	domains := make([]string, 0, len(s.records))
	for d := range s.records {
		domains = append(domains, d)
	}
	return domains
}
