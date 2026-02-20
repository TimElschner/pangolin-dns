package main

import (
	"sync"
	"testing"
)

func TestRecordStore_LookupMiss(t *testing.T) {
	s := NewRecordStore()
	_, ok := s.Lookup("missing.example.com.")
	if ok {
		t.Error("expected miss, got hit")
	}
}

func TestRecordStore_UpdateAndLookup(t *testing.T) {
	s := NewRecordStore()
	s.Update(map[string]string{
		"app.example.com.": "10.0.0.1",
	})

	ip, ok := s.Lookup("app.example.com.")
	if !ok {
		t.Fatal("expected hit, got miss")
	}
	if ip != "10.0.0.1" {
		t.Errorf("expected 10.0.0.1, got %q", ip)
	}
}

func TestRecordStore_UpdateReplacesAll(t *testing.T) {
	s := NewRecordStore()
	s.Update(map[string]string{"old.example.com.": "1.2.3.4"})
	s.Update(map[string]string{"new.example.com.": "5.6.7.8"})

	if _, ok := s.Lookup("old.example.com."); ok {
		t.Error("old record should have been removed after full update")
	}
	if _, ok := s.Lookup("new.example.com."); !ok {
		t.Error("new record should be present")
	}
}

func TestRecordStore_Count(t *testing.T) {
	s := NewRecordStore()
	if s.Count() != 0 {
		t.Errorf("empty store should have count 0, got %d", s.Count())
	}
	s.Update(map[string]string{
		"a.example.com.": "1.1.1.1",
		"b.example.com.": "2.2.2.2",
	})
	if s.Count() != 2 {
		t.Errorf("expected count 2, got %d", s.Count())
	}
}

func TestRecordStore_ConcurrentAccess(t *testing.T) {
	s := NewRecordStore()
	records := map[string]string{"x.example.com.": "9.9.9.9"}
	s.Update(records)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.Lookup("x.example.com.")
		}()
		go func() {
			defer wg.Done()
			s.Update(records)
		}()
	}
	wg.Wait()
}
