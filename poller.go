package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type Poller struct {
	cfg    *Config
	store  *RecordStore
	client *http.Client
}

// API response types

type OrgsResponse struct {
	Data struct {
		Orgs []struct {
			OrgID string `json:"orgId"`
			Name  string `json:"name"`
		} `json:"orgs"`
	} `json:"data"`
	Success bool `json:"success"`
}

type ResourcesResponse struct {
	Data struct {
		Resources []struct {
			FullDomain string `json:"fullDomain"`
			Enabled    bool   `json:"enabled"`
			Name       string `json:"name"`
		} `json:"resources"`
		Pagination struct {
			Total    int `json:"total"`
			Page     int `json:"page"`
			PageSize int `json:"pageSize"`
		} `json:"pagination"`
	} `json:"data"`
	Success bool `json:"success"`
}

func NewPoller(cfg *Config, store *RecordStore) *Poller {
	return &Poller{
		cfg:   cfg,
		store: store,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Run starts the polling loop. It polls immediately on start, then every PollInterval.
func (p *Poller) Run(ctx context.Context) {
	p.poll()

	ticker := time.NewTicker(p.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("poller: shutting down")
			return
		case <-ticker.C:
			p.poll()
		}
	}
}

func (p *Poller) poll() {
	orgIDs, err := p.getOrgIDs()
	if err != nil {
		log.Printf("poller: failed to get org IDs: %v", err)
		return
	}

	records := make(map[string]string)

	for _, orgID := range orgIDs {
		domains, err := p.getDomainsForOrg(orgID)
		if err != nil {
			log.Printf("poller: failed to get resources for org %s: %v", orgID, err)
			continue
		}

		for _, domain := range domains {
			fqdn := strings.ToLower(domain)
			if !strings.HasSuffix(fqdn, ".") {
				fqdn += "."
			}
			records[fqdn] = p.cfg.PangolinLocalIP

			if p.cfg.EnableLocalPrefix {
				localFQDN := "local." + fqdn
				records[localFQDN] = p.cfg.PangolinLocalIP
			}
		}
	}

	p.store.Update(records)
	log.Printf("poller: updated %d DNS records from %d org(s)", len(records), len(orgIDs))
}

func (p *Poller) getOrgIDs() ([]string, error) {
	// If org ID is configured, use it directly
	if p.cfg.PangolinOrgID != "" {
		return []string{p.cfg.PangolinOrgID}, nil
	}

	// Auto-discover orgs via API (requires root API key)
	body, err := p.apiGet("/v1/orgs?limit=1000&offset=0")
	if err != nil {
		return nil, fmt.Errorf("GET /v1/orgs: %w", err)
	}

	var resp OrgsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse orgs response: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("API returned success=false for /v1/orgs")
	}

	ids := make([]string, 0, len(resp.Data.Orgs))
	for _, org := range resp.Data.Orgs {
		ids = append(ids, org.OrgID)
		log.Printf("poller: discovered org %q (%s)", org.Name, org.OrgID)
	}

	if len(ids) == 0 {
		return nil, fmt.Errorf("no organizations found")
	}

	return ids, nil
}

func (p *Poller) getDomainsForOrg(orgID string) ([]string, error) {
	var allDomains []string
	page := 1
	pageSize := 100

	for {
		path := fmt.Sprintf("/v1/org/%s/resources?page=%d&pageSize=%d", orgID, page, pageSize)
		body, err := p.apiGet(path)
		if err != nil {
			return nil, fmt.Errorf("GET %s: %w", path, err)
		}

		var resp ResourcesResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("parse resources response: %w", err)
		}

		if !resp.Success {
			return nil, fmt.Errorf("API returned success=false for %s", path)
		}

		for _, r := range resp.Data.Resources {
			if r.FullDomain != "" && r.Enabled {
				allDomains = append(allDomains, r.FullDomain)
			}
		}

		// Check if there are more pages
		if len(resp.Data.Resources) < pageSize {
			break
		}
		page++
	}

	return allDomains, nil
}

func (p *Poller) apiGet(path string) ([]byte, error) {
	url := strings.TrimRight(p.cfg.PangolinAPIURL, "/") + path

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.cfg.PangolinAPIKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}
