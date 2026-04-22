# dyndns

A lightweight DynDNS bridge for Fritz!Box routers that keeps Cloudflare AAAA (IPv6) records in sync with your home prefix.

When your ISP rotates your IPv6 prefix, Fritz!Box calls this service via its built-in DynDNS hook. The service combines the new prefix with a static per-host suffix and upserts the corresponding Cloudflare DNS record — creating it if it doesn't exist, updating it only when the address has changed.

## How it works

```
Fritz!Box  →  GET /update?token=<secret>&ip6lanprefix=2001:db8:1234::/64
                         ↓
                   dyndns service
                         ↓
           prefix "2001:db8:1234::/64" + suffix "::1"
                         ↓
           Cloudflare AAAA "2001:db8:1234::1"
```

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
  - zone_id: "your-cloudflare-zone-id"
    name: "home.example.com"
    suffix: "::1"
  - zone_id: "your-cloudflare-zone-id"
    name: "nas.example.com"
    suffix: "::2"
```

`update_token` and `cloudflare.api_token` can be overridden via environment variables `UPDATE_TOKEN` and `CLOUDFLARE_API_TOKEN` respectively.

## Running with Docker Compose

```bash
docker compose up -d
```

The service binds to port `8080`. Pass secrets via env vars to avoid storing them in `config.yaml`:

```bash
UPDATE_TOKEN=secret CLOUDFLARE_API_TOKEN=cf-token docker compose up -d
```

## Running directly

```bash
go run ./cmd/dyndns -config config.yaml -addr :8080
```

## Fritz!Box setup

In Fritz!Box go to **Internet → Freigaben → DynDNS** and configure:

| Field        | Value                                               |
|--------------|-----------------------------------------------------|
| Provider     | Custom                                              |
| Update URL   | `http://<your-server>:8080/update?token=<DOMAIN>&ip6lanprefix=<IP6LANPREFIX>` |
| Domain name  | your secret token (Fritz!Box sends this as `token`) |
| Username     | _(any value)_                                       |
| Password     | _(any value)_                                       |

Fritz!Box substitutes `<DOMAIN>` and `<IP6LANPREFIX>` with the actual values on each update.

## Development

```bash
go test ./...
go vet ./...
```

CI runs on every push to `main` and on all pull requests.
