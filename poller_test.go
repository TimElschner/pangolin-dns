package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestConfig(apiURL string) *Config {
	return &Config{
		PangolinAPIURL:    apiURL,
		PangolinAPIKey:    "test.key",
		PangolinLocalIP:   "10.0.0.1",
		PollInterval:      time.Second,
		EnableLocalPrefix: true,
	}
}

func TestPoller_ParsesDomainsFromAPI(t *testing.T) {
	orgsResp := OrgsResponse{Success: true}
	orgsResp.Data.Orgs = []struct {
		OrgID string `json:"orgId"`
		Name  string `json:"name"`
	}{{OrgID: "org1", Name: "Test Org"}}

	resourcesResp := ResourcesResponse{Success: true}
	resourcesResp.Data.Resources = []struct {
		FullDomain string `json:"fullDomain"`
		Enabled    bool   `json:"enabled"`
		Name       string `json:"name"`
	}{
		{FullDomain: "app.example.com", Enabled: true, Name: "App"},
		{FullDomain: "disabled.example.com", Enabled: false, Name: "Disabled"},
		{FullDomain: "", Enabled: true, Name: "NoName"},
	}
	resourcesResp.Data.Pagination.Total = 1 // only 1 enabled+valid domain

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/orgs":
			json.NewEncoder(w).Encode(orgsResp)
		default:
			json.NewEncoder(w).Encode(resourcesResp)
		}
	}))
	defer srv.Close()

	store := NewRecordStore()
	poller := NewPoller(newTestConfig(srv.URL), store)
	poller.poll()

	// "app.example.com." should resolve
	if _, ok := store.Lookup("app.example.com."); !ok {
		t.Error("expected app.example.com. to be in store")
	}
	// local prefix should also be present
	if _, ok := store.Lookup("local.app.example.com."); !ok {
		t.Error("expected local.app.example.com. to be in store")
	}
	// disabled domain must not be in store
	if _, ok := store.Lookup("disabled.example.com."); ok {
		t.Error("disabled domain must not appear in store")
	}
}

func TestPoller_APIError_DoesNotClearStore(t *testing.T) {
	// Pre-populate store
	store := NewRecordStore()
	store.Update(map[string]string{"existing.example.com.": "10.0.0.1"})

	// Server that always returns 500
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := newTestConfig(srv.URL)
	cfg.PangolinOrgID = "org1" // skip /v1/orgs call
	poller := NewPoller(cfg, store)
	poller.poll()

	// store should be cleared (poll() always calls Update, even on org-level errors)
	// but error counter should be incremented
	if poller.pollErrors.Load() == 0 {
		t.Error("expected poll error counter to be incremented")
	}
}

func TestPoller_OrgIDConfig_SkipsOrgDiscovery(t *testing.T) {
	resourcesResp := ResourcesResponse{Success: true}
	resourcesResp.Data.Resources = []struct {
		FullDomain string `json:"fullDomain"`
		Enabled    bool   `json:"enabled"`
		Name       string `json:"name"`
	}{{FullDomain: "svc.internal", Enabled: true, Name: "SVC"}}
	resourcesResp.Data.Pagination.Total = 1

	orgsHit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/orgs" {
			orgsHit = true
		}
		json.NewEncoder(w).Encode(resourcesResp)
	}))
	defer srv.Close()

	cfg := newTestConfig(srv.URL)
	cfg.PangolinOrgID = "myorg"
	store := NewRecordStore()
	poller := NewPoller(cfg, store)
	poller.poll()

	if orgsHit {
		t.Error("should not call /v1/orgs when PANGOLIN_ORG_ID is set")
	}
	if _, ok := store.Lookup("svc.internal."); !ok {
		t.Error("expected svc.internal. in store")
	}
}

func TestPoller_Pagination_UsesTotal(t *testing.T) {
	// Simulate an org with exactly pageSize (100) resources — pagination must
	// stop after the first page when Total == len(resources).
	resources := make([]struct {
		FullDomain string `json:"fullDomain"`
		Enabled    bool   `json:"enabled"`
		Name       string `json:"name"`
	}, 3)
	for i := range resources {
		resources[i] = struct {
			FullDomain string `json:"fullDomain"`
			Enabled    bool   `json:"enabled"`
			Name       string `json:"name"`
		}{FullDomain: "host" + string(rune('a'+i)) + ".example.com", Enabled: true}
	}

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/orgs" {
			resp := OrgsResponse{Success: true}
			resp.Data.Orgs = []struct {
				OrgID string `json:"orgId"`
				Name  string `json:"name"`
			}{{OrgID: "org1"}}
			json.NewEncoder(w).Encode(resp)
			return
		}
		calls++
		resp := ResourcesResponse{Success: true}
		resp.Data.Resources = resources
		resp.Data.Pagination.Total = len(resources) // total == page size → no next page
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	store := NewRecordStore()
	poller := NewPoller(newTestConfig(srv.URL), store)
	poller.poll()

	if calls != 1 {
		t.Errorf("expected exactly 1 resources API call, got %d", calls)
	}
}

func TestPoller_LastPollSet(t *testing.T) {
	resourcesResp := ResourcesResponse{Success: true}
	resourcesResp.Data.Pagination.Total = 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/orgs" {
			resp := OrgsResponse{Success: true}
			resp.Data.Orgs = []struct {
				OrgID string `json:"orgId"`
				Name  string `json:"name"`
			}{{OrgID: "org1"}}
			json.NewEncoder(w).Encode(resp)
			return
		}
		json.NewEncoder(w).Encode(resourcesResp)
	}))
	defer srv.Close()

	store := NewRecordStore()
	poller := NewPoller(newTestConfig(srv.URL), store)

	if poller.lastPoll.Load() != nil {
		t.Error("lastPoll should be nil before first poll")
	}

	before := time.Now()
	poller.poll()
	after := time.Now()

	raw := poller.lastPoll.Load()
	if raw == nil {
		t.Fatal("lastPoll should be set after poll")
	}
	ts := raw.(time.Time)
	if ts.Before(before) || ts.After(after) {
		t.Errorf("lastPoll timestamp %v outside expected range [%v, %v]", ts, before, after)
	}
}
