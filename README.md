# pangolin-dns

A lightweight DNS resolver that automatically discovers services from a [Pangolin](https://github.com/fosrl/pangolin) reverse proxy and resolves their domains to a local IP address — avoiding hairpin NAT when clients and services are on the same LAN.

## Problem

When you run Pangolin as a reverse proxy in your homelab, all traffic to your services (e.g. `app.example.com`) goes through the public internet and back via hairpin NAT, even if both the client and Pangolin are on the same local network. This adds unnecessary latency and external bandwidth usage.

## Solution

pangolin-dns is a single Go binary that acts as a DNS server. It polls the Pangolin Integration API to discover all configured resources and their domains, then resolves those domains to the local Pangolin IP. All other queries are forwarded to an upstream DNS server (e.g. 1.1.1.1).

```
Client DNS query: app.example.com
  → pangolin-dns checks local store
  → Found! Returns 10.1.100.2 (local Pangolin IP)

Client DNS query: google.com
  → pangolin-dns checks local store
  → Not found, forwards to upstream (1.1.1.1)
```

## Features

- **Auto-discovery** — polls the Pangolin Integration API and picks up new resources automatically
- **Local prefix** — optionally creates `local.{domain}` entries as an explicit local alternative
- **Upstream forwarding** — non-Pangolin domains are forwarded to a configurable upstream DNS
- **Lightweight** — single static Go binary, ~10MB Docker image
- **Zero config for domains** — no manual domain list needed, everything comes from Pangolin

## Architecture

```
┌──────────────────────────────────────────┐
│  pangolin-dns                            │
│                                          │
│  ┌─────────────┐   ┌──────────────────┐  │
│  │ DNS Server   │   │ Pangolin Poller  │  │
│  │ (miekg/dns)  │   │ (HTTP client)    │  │
│  │ :53 UDP/TCP  │   │ every 60s        │  │
│  └──────┬───────┘   └────────┬─────────┘  │
│         │  ┌─────────────┐   │             │
│         └──│ Record Store │───┘             │
│            │ (in-memory)  │                │
│            └─────────────┘                 │
└──────────────────────────────────────────┘
```

## Prerequisites

- Pangolin with the Integration API enabled (port 3004)
- An API key (root key for org auto-discovery, or org-scoped key with `PANGOLIN_ORG_ID` set)

## Quick Start

### Docker Compose (recommended)

1. Create a `.env` file:
   ```
   PANGOLIN_API_KEY=your_api_key_id.your_api_key_secret
   ```

2. Start the container:
   ```bash
   docker compose up -d
   ```

3. Test it:
   ```bash
   nslookup app.example.com localhost
   ```

### Build from Source

```bash
go build -o pangolin-dns .
PANGOLIN_API_KEY=your_key ./pangolin-dns
```

## Configuration

All configuration is done via environment variables:

| Variable | Default | Description |
|---|---|---|
| `PANGOLIN_API_URL` | `http://10.1.100.2:3004` | Pangolin Integration API URL |
| `PANGOLIN_API_KEY` | *(required)* | API key (`keyId.keySecret`) |
| `PANGOLIN_LOCAL_IP` | `10.1.100.2` | IP to resolve Pangolin domains to |
| `PANGOLIN_ORG_ID` | *(auto-discover)* | Specific org ID (skip auto-discovery) |
| `UPSTREAM_DNS` | `1.1.1.1:53` | Upstream DNS server for non-local queries |
| `POLL_INTERVAL` | `60s` | How often to poll the Pangolin API |
| `DNS_PORT` | `53` | DNS server listen port |
| `ENABLE_LOCAL_PREFIX` | `true` | Create `local.{domain}` entries |

## Network Setup

To use pangolin-dns for your entire LAN, point your router's DNS setting to the IP of the machine running pangolin-dns. For example, in a UniFi setup, set the LAN DNS server to the Pangolin host IP.

## License

MIT
