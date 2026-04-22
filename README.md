# dyndns

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![CI](https://github.com/aivus/dyndns/actions/workflows/ci.yml/badge.svg)](https://github.com/aivus/dyndns/actions/workflows/ci.yml)

A lightweight DynDNS bridge for Fritz!Box routers that keeps Cloudflare AAAA (IPv6) records in sync with your home network.

When your ISP rotates your IPv6 prefix (or reassigns your router's WAN address), Fritz!Box calls this service via its built-in DynDNS hook. The service upserts Cloudflare DNS records — creating them if they don't exist, updating them only when the address has changed.

Two record modes are supported:

- **Prefix + suffix** — combines the delegated LAN prefix with a static per-host suffix. Use this for hosts behind the router (NAS, server, etc.).
- **Router IP** — uses the router's WAN IPv6 address directly. Use this when you want a DNS record for the router itself and the ISP assigns the full address (no fixed suffix).

## How it works

**Prefix + suffix mode** (hosts behind the router):
```
Fritz!Box  →  GET /update?token=<secret>&ip6lanprefix=2001:db8:1234::/64
                         ↓
           prefix "2001:db8:1234::/64" + suffix "::1"
                         ↓
           Cloudflare AAAA "2001:db8:1234::1"
```

**Router IP mode** (the router itself):
```
Fritz!Box  →  GET /update?token=<secret>&ip6addr=2001:db8:1234:5601:abcd:ef01:2345:6789
                         ↓
           Cloudflare AAAA "2001:db8:1234:5601:abcd:ef01:2345:6789"
```

Both parameters can be sent in the same request to update all record types at once.

## Requirements

- A Cloudflare account with DNS zones managed by Cloudflare
- A Cloudflare API token with **DNS:Edit** permission for the target zone(s)
- A Fritz!Box router with DynDNS support

## Configuration

Copy `config.yaml` and fill in your values:

```yaml
# Secret token Fritz!Box sends with every update request.
update_token: "change-me"

cloudflare:
  api_token: "your-cloudflare-api-token"  # or set CLOUDFLARE_API_TOKEN env var

records:
  # Prefix + suffix: full address = ip6lanprefix + suffix
  - zone_id: "your-cloudflare-zone-id"
    name: "home.example.com"
    suffix: "::1"
  - zone_id: "your-cloudflare-zone-id"
    name: "nas.example.com"
    suffix: "::2"
  # Router IP: no suffix → uses ip6addr (the router's WAN IPv6 address) directly
  - zone_id: "your-cloudflare-zone-id"
    name: "router.example.com"
```

`update_token` and `cloudflare.api_token` can be overridden via environment variables `UPDATE_TOKEN` and `CLOUDFLARE_API_TOKEN` respectively.

## Running with Docker Compose

The image is published to the GitHub Container Registry. Pull and start with:

```bash
docker compose up -d
```

The service binds to port `8080`. Pass secrets via env vars to avoid storing them in `config.yaml`:

```bash
UPDATE_TOKEN=secret CLOUDFLARE_API_TOKEN=cf-token docker compose up -d
```

To pin a specific release instead of `latest`, edit `docker-compose.yml` and change the image tag:

```yaml
image: ghcr.io/aivus/dyndns:1.0.0
```

## Running directly

```bash
go run ./cmd/dyndns -config config.yaml -addr :8080
```

To enable debug logging:

```bash
go run ./cmd/dyndns -config config.yaml -addr :8080 -debug
```

With Docker Compose, add `-debug` to the `command` in `docker-compose.yml`:

```yaml
services:
  dyndns:
    image: ghcr.io/aivus/dyndns:latest
    command: ["-config", "/config/config.yaml", "-addr", ":8080", "-debug"]
```

## Fritz!Box setup

In Fritz!Box go to **Internet → Freigaben → DynDNS** and configure:

| Field        | Value                                               |
|--------------|-----------------------------------------------------|
| Provider     | Custom                                              |
| Update URL   | `http://<your-server>:8080/update?token=<DOMAIN>&ip6lanprefix=<IP6LANPREFIX>&ip6addr=<IP6ADDR>` |
| Domain name  | your secret token (Fritz!Box sends this as `token`) |
| Username     | _(any value)_                                       |
| Password     | _(any value)_                                       |

Fritz!Box substitutes `<DOMAIN>`, `<IP6LANPREFIX>`, and `<IP6ADDR>` with the actual values on each update. Including both parameters lets you update prefix+suffix records and router IP records in a single request. You can omit either parameter if you only have one record type.

## Development

```bash
go test ./...
go vet ./...
```

CI runs on every push to `main` and on all pull requests.

## License

This project is licensed under the [MIT License](LICENSE).
