package main

import (
	"testing"
)

func TestLoadConfig_MissingAPIKey(t *testing.T) {
	t.Setenv("PANGOLIN_API_KEY", "")
	_, err := LoadConfig()
	if err == nil {
		t.Error("expected error when PANGOLIN_API_KEY is empty")
	}
}

func TestLoadConfig_InvalidPollInterval(t *testing.T) {
	t.Setenv("PANGOLIN_API_KEY", "test.key")
	t.Setenv("POLL_INTERVAL", "not-a-duration")
	_, err := LoadConfig()
	if err == nil {
		t.Error("expected error for invalid POLL_INTERVAL")
	}
}

func TestLoadConfig_InvalidLocalIP(t *testing.T) {
	t.Setenv("PANGOLIN_API_KEY", "test.key")
	t.Setenv("PANGOLIN_LOCAL_IP", "not-an-ip")
	_, err := LoadConfig()
	if err == nil {
		t.Error("expected error for invalid PANGOLIN_LOCAL_IP")
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	t.Setenv("PANGOLIN_API_KEY", "test.key")
	t.Setenv("PANGOLIN_LOCAL_IP", "")   // force default
	t.Setenv("POLL_INTERVAL", "")       // force default
	t.Setenv("DNS_PORT", "")            // force default
	t.Setenv("HEALTH_PORT", "")         // force default
	t.Setenv("UPSTREAM_DNS", "")        // force default
	t.Setenv("ENABLE_LOCAL_PREFIX", "") // force default

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.PangolinLocalIP != "10.1.100.2" {
		t.Errorf("expected default local IP, got %q", cfg.PangolinLocalIP)
	}
	if cfg.DNSPort != "53" {
		t.Errorf("expected default DNS port 53, got %q", cfg.DNSPort)
	}
	if cfg.HealthPort != "8080" {
		t.Errorf("expected default health port 8080, got %q", cfg.HealthPort)
	}
	if cfg.UpstreamDNS != "1.1.1.1:53" {
		t.Errorf("expected default upstream DNS, got %q", cfg.UpstreamDNS)
	}
	if !cfg.EnableLocalPrefix {
		t.Error("expected EnableLocalPrefix=true by default")
	}
	if cfg.PollInterval.String() != "1m0s" {
		t.Errorf("expected default poll interval 60s, got %s", cfg.PollInterval)
	}
}

func TestLoadConfig_CustomValues(t *testing.T) {
	t.Setenv("PANGOLIN_API_KEY", "myid.mysecret")
	t.Setenv("PANGOLIN_LOCAL_IP", "192.168.1.100")
	t.Setenv("POLL_INTERVAL", "30s")
	t.Setenv("ENABLE_LOCAL_PREFIX", "false")
	t.Setenv("HEALTH_PORT", "9090")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.PangolinLocalIP != "192.168.1.100" {
		t.Errorf("expected 192.168.1.100, got %q", cfg.PangolinLocalIP)
	}
	if cfg.PollInterval.Seconds() != 30 {
		t.Errorf("expected 30s, got %s", cfg.PollInterval)
	}
	if cfg.EnableLocalPrefix {
		t.Error("expected EnableLocalPrefix=false")
	}
	if cfg.HealthPort != "9090" {
		t.Errorf("expected health port 9090, got %q", cfg.HealthPort)
	}
}
