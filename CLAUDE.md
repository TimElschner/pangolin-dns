# pangolin-dns — CLAUDE.md

## What this project does

A lightweight DNS server (Go) that solves hairpin NAT in homelabs. It polls the
[Pangolin](https://github.com/fosrl/pangolin) reverse proxy Integration API to
discover configured service domains and resolves them to a local IP. Everything
else is forwarded to an upstream DNS server (default: 1.1.1.1).

## Architecture

```
main.go        — wires up components, handles OS signals, starts goroutines
config.go      — loads all config from environment variables
store.go       — thread-safe in-memory DNS record map (FQDN → IP)
poller.go      — polls Pangolin API every POLL_INTERVAL, updates store
dns.go         — miekg/dns server on :53 UDP+TCP, answers A queries from store
health.go      — HTTP server on :8080 with GET /healthz endpoint
```

All code lives in `package main` — intentionally flat, no sub-packages.

## Build & Run

```bash
# Build
go build -o pangolin-dns .

# Run (minimum required env var)
PANGOLIN_API_KEY=keyId.keySecret ./pangolin-dns

# Run tests
go test -race -v ./...

# Docker
docker compose up -d
```

## Environment Variables

| Variable              | Default                   | Required | Description                                      |
|-----------------------|---------------------------|----------|--------------------------------------------------|
| `PANGOLIN_API_KEY`    | —                         | Yes      | API key (`keyId.keySecret`)                      |
| `PANGOLIN_API_URL`    | `http://10.1.100.2:3004`  | No       | Pangolin Integration API base URL                |
| `PANGOLIN_LOCAL_IP`   | `10.1.100.2`              | No       | IP to resolve Pangolin domains to (must be valid)|
| `PANGOLIN_ORG_ID`     | *(auto-discover)*         | No       | Skip org auto-discovery if set                   |
| `UPSTREAM_DNS`        | `1.1.1.1:53`              | No       | Upstream DNS for non-local queries               |
| `POLL_INTERVAL`       | `60s`                     | No       | Go duration string (e.g. `30s`, `5m`)            |
| `DNS_PORT`            | `53`                      | No       | UDP/TCP listen port                              |
| `HEALTH_PORT`         | `8080`                    | No       | HTTP health endpoint port                        |
| `ENABLE_LOCAL_PREFIX` | `true`                    | No       | Also register `local.{domain}` entries           |

## Code Conventions

- Logger: stdlib `log` only — prefix set to `[pangolin-dns]` in main.go
- No external frameworks beyond `github.com/miekg/dns`
- Errors always wrapped with `fmt.Errorf("context: %w", err)`
- DNS records stored as lowercased FQDN with trailing dot: `"app.example.com."`
- Config validation happens entirely in `LoadConfig()` — fail fast at startup

## Known Limitations / Future Work

- Single upstream DNS server (no fallback)
- No retry/backoff on API poll failures
- No metrics endpoint (Prometheus)
- DNS record TTL is hardcoded to 60s
