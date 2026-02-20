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
- **Health endpoint** — `GET /healthz` on port 8080 reports record count, last poll time and error count

## Architecture

```
┌──────────────────────────────────────────┐
│  pangolin-dns                            │
│                                          │
│  ┌─────────────┐   ┌──────────────────┐  │
│  │ DNS Server  │   │ Pangolin Poller  │  │
│  │ (miekg/dns) │   │ (HTTP client)    │  │
│  │ :53 UDP/TCP │   │ every 60s        │  │
│  └──────┬──────┘   └────────┬─────────┘  │
│         │  ┌────────────┐   │            │
│         └──│Record Store│───┘            │
│            │ (in-memory)│                │
│            └────────────┘                │
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
| `HEALTH_PORT` | `8080` | HTTP health endpoint port |
| `ENABLE_LOCAL_PREFIX` | `true` | Create `local.{domain}` entries |

## Installation

pangolin-dns runs as a Docker container on any host that is reachable from your LAN — typically the same machine as Pangolin itself.

### 1. Create a `.env` file

```
PANGOLIN_API_KEY=your_api_key_id.your_api_key_secret
```

The API key is created in the Pangolin admin UI under **Settings → API Keys**. Use a root key to allow auto-discovery of all organizations, or set `PANGOLIN_ORG_ID` and use an org-scoped key.

### 2. Adjust `docker-compose.yml` if needed

The defaults assume Pangolin runs at `10.1.100.2`. If your setup differs, override the relevant variables:

```yaml
environment:
  - PANGOLIN_API_URL=http://<your-pangolin-ip>:3004
  - PANGOLIN_LOCAL_IP=<your-pangolin-ip>
```

### 3. Start the container

```bash
docker compose up -d
```

### 4. Verify it works

```bash
# Check the health endpoint
curl http://<host-ip>:8080/healthz

# Test DNS resolution (replace with one of your actual Pangolin domains)
nslookup app.example.com <host-ip>
```

The health response looks like:
```json
{"status":"ok","records":12,"last_poll":"2026-02-20T19:00:00Z","poll_errors":0}
```

`records` should be > 0 after the first poll (within a few seconds of startup).

---

## Network Setup

pangolin-dns only does something useful if your devices actually use it as their DNS resolver. You have two options:

### Option A: Router / DHCP (recommended — affects all LAN clients automatically)

Configure your router to hand out the pangolin-dns host IP as the DNS server via DHCP. New DHCP leases will pick it up immediately; existing clients will update on their next lease renewal (or after a reconnect).

**UniFi (UDM / UDM Pro / USG):**

1. Go to **Settings → Networks → [your LAN network] → Advanced**
2. Under **DHCP Name Server**, select **Manual**
3. Enter the IP of the host running pangolin-dns as DNS Server 1
4. Optionally add `1.1.1.1` as DNS Server 2 as a fallback (pangolin-dns already forwards unknown queries upstream, so this is only needed if pangolin-dns itself goes down)
5. Save — clients will receive the new DNS on their next DHCP renewal

**pfSense / OPNsense:**

1. Go to **Services → DHCP Server → [your LAN interface]**
2. Set **DNS Servers** to the pangolin-dns host IP
3. Save and apply

**Generic router (most home routers):**

Look for **LAN / DHCP Settings** and set the **Primary DNS** to the pangolin-dns host IP. The exact location varies by router brand.

---

### Option B: Per-device (useful for testing or single machines)

Set the DNS server manually on the device you want to configure. pangolin-dns forwards all non-Pangolin queries upstream, so it is safe to use as your only DNS resolver.

**Windows:**

1. Open **Settings → Network & Internet → [your adapter] → Edit DNS**
2. Switch to **Manual**, enable IPv4
3. Set **Preferred DNS** to the pangolin-dns host IP
4. Set **Alternate DNS** to `1.1.1.1` (fallback if pangolin-dns is unreachable)

Or via PowerShell (replace `Ethernet` and the IP as needed):
```powershell
Set-DnsClientServerAddress -InterfaceAlias "Ethernet" -ServerAddresses "10.1.100.2","1.1.1.1"
```

**macOS:**

1. Open **System Settings → Network → [your connection] → Details → DNS**
2. Click **+** and add the pangolin-dns host IP
3. Click OK and Apply

Or via terminal:
```bash
# Replace "Wi-Fi" with your interface name from `networksetup -listallnetworkservices`
networksetup -setdnsservers "Wi-Fi" 10.1.100.2 1.1.1.1
```

**Linux (systemd-resolved):**

Edit `/etc/systemd/resolved.conf`:
```ini
[Resolve]
DNS=10.1.100.2
FallbackDNS=1.1.1.1
```
Then restart: `sudo systemctl restart systemd-resolved`

Or per-interface via NetworkManager:
```bash
nmcli connection modify "your-connection" ipv4.dns "10.1.100.2 1.1.1.1"
nmcli connection up "your-connection"
```

---

### Verify resolution after setup

```bash
# Should return the local Pangolin IP, not a public IP
nslookup app.example.com

# Check which DNS server answered
nslookup app.example.com 10.1.100.2
```

If `nslookup` returns the `PANGOLIN_LOCAL_IP` you configured, everything is working.

## License

MIT
