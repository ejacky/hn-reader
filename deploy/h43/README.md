# HN Reader Cloud on h43

Target: `https://hn-reader.whoareyou.ren` → `43.142.113.109` (Tencent CVM, Rocky Linux 9.4).

## Topology

```
Internet
  │  80/443
  ▼
Caddy (existing simpletimetracker-web stack, TLS + HSTS)
  │  reverse_proxy hn-reader-server:8080
  │  (Docker network simpletimetracker-web_default)
  ▼
hn-reader-server (Go, -tags cloud) ── hn-reader-db (PostgreSQL 16, no public port)
```

- Caddy remains the only process bound to 80/443.
- The cloud server is not published publicly; `127.0.0.1:8082` is a loopback-only
  debug port for smoke tests.
- The server joins the existing STT compose network so Caddy can resolve it by
  name. Do not run `docker compose down` in `simpletimetracker-web` while
  HN Reader is meant to stay online: that removes the shared network.

## Current deployment

- Image `hn-reader-cloud:1.2.0`; `/health` reports `1.2.0`.
- PostgreSQL 16 pinned by digest; data in volume `hn-reader_pgdata`.
- Caddy site added to `/root/simpletimetracker-web/Caddyfile` (backup kept as
  `Caddyfile.bak.*`), with `request_body` limit and `header_up` overwrites for
  `X-Forwarded-For` / `X-Real-IP`.
- The temporary IP allowlist was removed on 2026-10-22 after the P0 checks;
  anonymous signup, per-user rate limits, SSRF protection, and the global LLM
  budget are now the public-facing controls.
- Daily backup: `/etc/cron.d/hn-reader-backup` at 03:30 UTC, 14-day retention,
  dumps are mode 600.
- Fixed blocking bug: anonymous signup previously failed because `users.email`
  was NULL and scanned into a Go string; schema/queries now use `''`/`COALESCE`.

## Prerequisites

- DNS: `hn-reader.whoareyou.ren A 43.142.113.109`.
- Ports 80/443 reachable from the Internet (already used by STT).
- The STT stack and its `simpletimetracker-web_default` network are running.

## Build and upload

```bash
cd server
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags cloud -trimpath \
  -ldflags "-s -w" -o hn-reader-cloud ./cmd/hn-reader-cloud

ssh h43 'mkdir -p /root/hn-reader/backups'
scp deploy/h43/Dockerfile deploy/h43/docker-compose.yml \
    deploy/h43/Caddyfile.site deploy/h43/.env.example \
    h43:/root/hn-reader/
scp server/hn-reader-cloud h43:/root/hn-reader/
```

`-tags cloud` is mandatory. Without it the build fails by design (build
constraint in `cmd/hn-reader-cloud/main.go`), which prevents shipping the local
no-op authentication by accident.

## Environment

Copy `deploy/h43/.env.example` to `/root/hn-reader/.env`, set
`POSTGRES_PASSWORD`, `LLM_API_KEY`, and the URL/model, then `chmod 600 .env`.
The server reads all values through Compose `env_file`; no rebuild is needed
for limit/budget changes, but the container must be recreated:

```bash
cd /root/hn-reader
docker compose up -d --force-recreate server
```

Key values:

| Variable | Default | Purpose |
|---|---|---|
| `TRUSTED_PROXIES` | `127.0.0.1,172.16.0.0/12` | Gin trusted proxy CIDRs |
| `FREE_QUOTA_LIMIT` | `50` | New anonymous account monthly quota |
| `LLM_DAILY_BUDGET` | `2000` | Process-wide daily LLM call cap |
| `LLM_MAX_CONCURRENCY` | `4` | Concurrent LLM calls |
| `LLM_TIMEOUT_SECONDS` | `90` | Per-call timeout |
| `LLM_MAX_OUTPUT_TOKENS` | `600` | Output cost cap |
| `SIGNUP_RATE_PER_HOUR` / `_PER_DAY` | `5` / `20` | Anonymous signup per IP |
| `SUMMARIZE_RATE_PER_MIN` | `20` | Summary requests per user |
| `PRELOAD_MAX_PER_REQUEST` | `20` | Preload batch cap |
| `PRELOAD_MAX_PENDING_PER_USER` | `20` | Per-user queue cap |
| `PRELOAD_QUEUE_SIZE` | `200` | Global queue cap |

## Start

```bash
cd /root/hn-reader
docker compose config >/dev/null          # syntax check
docker compose pull db
docker compose build server
docker compose up -d
docker compose ps
docker exec hn-reader-server wget -qO- http://127.0.0.1:8080/health
```

## Caddy

The site block must include the forwarded-header overwrites and the body limit:

```caddyfile
hn-reader.whoareyou.ren {
	encode zstd gzip

	request_body {
		max_size 256KB
	}

	# Temporary gate while validating the P0 hardening. Remove after acceptance.
	# @blocked not remote_ip <operator-ip> <proxy-egress-ip>
	# respond @blocked "HN Reader cloud is not public yet" 403

	reverse_proxy hn-reader-server:8080 {
		header_up X-Forwarded-For {remote_host}
		header_up X-Real-IP {remote_host}
	}

	header {
		Strict-Transport-Security "max-age=31536000"
		-Server
	}

	log {
		output stdout
		format console
	}
}
```

Apply:

```bash
cd /root/simpletimetracker-web
cp -p Caddyfile "Caddyfile.bak.$(date -u +%Y%m%d-%H%M%S)"
# append/replace the hn-reader block, then:
docker compose exec -T caddy caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
docker compose exec -T caddy caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile
```

Removing the gate is the same procedure with the `@blocked`/`respond` lines
deleted; validate and reload again.

> **Bind-mount gotcha:** `/root/simpletimetracker-web/Caddyfile` is mounted as a
> single file. If you replace it with `cp`/`mv` (a new inode), the running Caddy
> container keeps the old file and `caddy reload` will say "config is
> unchanged". Edit it in place (`cat new > Caddyfile`) and then reload. If the
> inode was already replaced, recreate only Caddy once:
> `docker compose up -d --force-recreate caddy` (brief STT downtime).

## Smoke tests

```bash
curl -sS http://127.0.0.1:8082/health

TOKEN=$(curl -sS -X POST http://127.0.0.1:8082/auth/anonymous \
  -H 'Content-Type: application/json' -d '{}' \
  | sed -E 's/.*"token":"([^"]+)".*/\1/')

curl -sS http://127.0.0.1:8082/auth/me -H "Authorization: Bearer $TOKEN"

# one real summary
curl -sS -m 120 -X POST http://127.0.0.1:8082/api/summarize/article \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"hn_id":1,"url":"https://example.com","title":"Example Domain","lang":"en"}'

# SSRF guard: expect 400 invalid_url
curl -sS -X POST http://127.0.0.1:8082/api/summarize/article \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"hn_id":1,"url":"http://169.254.169.254/latest/meta-data","title":"x","lang":"en"}'

# public TLS
curl -sSI https://hn-reader.whoareyou.ren/health
```

## Updating

```bash
# build on the workstation, upload, then:
cd /root/hn-reader
docker build -t hn-reader-cloud:1.2.1 .
docker compose up -d server
```

Keep the previous image tag for rollback.

## Rollback

- Server: change the image tag in `docker-compose.yml` back to the previous
  version, `docker compose up -d server`.
- Caddy: restore the most recent `Caddyfile.bak.*`, validate, reload.
- Database: migrations are additive (`global_articles`, email default/not-null);
  the previous binary ignores the new table and remains compatible.
- Limits/budget: change `.env`, then `docker compose up -d --force-recreate server`;
  no code rollback needed.

## Backups

```bash
#!/bin/sh
set -eu
umask 077
cd /root/hn-reader
install -d -m 700 backups
ts=$(date -u +%Y%m%d-%H%M%S)
docker exec hn-reader-db sh -c 'PGPASSWORD=$POSTGRES_PASSWORD pg_dump -U hn_reader -d hn_reader' \
  | gzip > "backups/hn-reader-$ts.sql.gz"
chmod 600 "backups/hn-reader-$ts.sql.gz"
find backups -type f -name '*.sql.gz' -mtime +14 -delete
```

Installed as `/etc/cron.d/hn-reader-backup` (03:30 UTC).

## Remaining non-P0 work

- Recommendation top-stories cache and profile ranking.
- Comment global cache.
- Multi-instance support (rate limits/budget would move to Redis).
- Metrics/alerting and a public status page.
- Payment/Pro implementation (upgrade CTAs are hidden for the free launch).
