package main

import (
	"fmt"
	"net"
	"os"
	"time"
)

type Config struct {
	PangolinAPIURL    string
	PangolinAPIKey    string
	PangolinLocalIP   string
	PangolinOrgID     string // optional: if empty, auto-discover via /v1/orgs
	UpstreamDNS       string
	PollInterval      time.Duration
	DNSPort           string
	HealthPort        string
	EnableLocalPrefix bool
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		PangolinAPIURL:    envOrDefault("PANGOLIN_API_URL", "http://10.1.100.2:3004"),
		PangolinAPIKey:    os.Getenv("PANGOLIN_API_KEY"),
		PangolinLocalIP:   envOrDefault("PANGOLIN_LOCAL_IP", "10.1.100.2"),
		PangolinOrgID:     os.Getenv("PANGOLIN_ORG_ID"),
		UpstreamDNS:       envOrDefault("UPSTREAM_DNS", "1.1.1.1:53"),
		DNSPort:           envOrDefault("DNS_PORT", "53"),
		HealthPort:        envOrDefault("HEALTH_PORT", "8080"),
		EnableLocalPrefix: envOrDefault("ENABLE_LOCAL_PREFIX", "true") == "true",
	}

	if cfg.PangolinAPIKey == "" {
		return nil, fmt.Errorf("PANGOLIN_API_KEY is required")
	}

	if net.ParseIP(cfg.PangolinLocalIP) == nil {
		return nil, fmt.Errorf("invalid PANGOLIN_LOCAL_IP: %q", cfg.PangolinLocalIP)
	}

	interval := envOrDefault("POLL_INTERVAL", "60s")
	d, err := time.ParseDuration(interval)
	if err != nil {
		return nil, fmt.Errorf("invalid POLL_INTERVAL %q: %w", interval, err)
	}
	cfg.PollInterval = d

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
