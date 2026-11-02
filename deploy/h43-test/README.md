# HN Reader staging on h43

Target: `https://test.hn-reader.whoareyou.ren` → `43.142.113.109` (Tencent CVM, Rocky Linux 9.4).

Purpose: validate a change end to end before it reaches production — the preload
fix, the English build, anything that only shows up against the real LLM provider
and a real Postgres. It is disposable by design: losing its database costs
nothing.

## Topology

```
Internet ──80/443──▶ edge (independent project, /root/edge)
                      │  sites/hn-reader-test.caddy  ← installed by deploy.sh
                      │  reverse_proxy hn-reader-test-server:8080
                      ▼
                    hn-reader-test-server ── hn-reader-test-db (PostgreSQL 16)
                      │  shared `edge` network (so the edge can resolve it)
                      │  loopback 127.0.0.1:8083 (smoke tests, no allowlist)
```

| | production | staging |
|---|---|---|
| compose project | `hn-reader` | `hn-reader-test` |
| container | `hn-reader-server` / `hn-reader-db` | `hn-reader-test-server` / `hn-reader-test-db` |
| volume | `hn-reader_pgdata` | `hn-reader-test_pgdata` |
| loopback port | `127.0.0.1:8082` | `127.0.0.1:8083` |
| site | `hn-reader.whoareyou.ren` | `test.hn-reader.whoareyou.ren` |
| access | public (rate limits + quota) | IP allowlist |
| lifecycle | manual deploy | `push` to `dev`/`fix/**`/`feat/**` deploys automatically |

They share exactly one thing: the `edge` network. A staging deploy cannot
restart, replace or reconfigure a production container.

The compose service is named `web`, not `server`: a service name becomes a DNS
alias on every network it joins, and production's `hn-reader-server` already
answers to `server` there. The edge proxies to the container name instead.

## Prerequisites

- DNS: `test.hn-reader.whoareyou.ren A 43.142.113.109` (already in place).
- The edge is installed and owns 80/443 (`/root/edge`, container `edge-caddy`).
- Ports 80/443 reachable; the host key of h43 pinned for the CI deploy key.

## One-time bootstrap

CI does not create the environment, and it never writes `.env` (that is where the
secrets live). Run this once, on the workstation:

```bash
ssh h43 'install -d -m 755 /root/hn-reader-test'

scp deploy/h43-test/{docker-compose.yml,Caddyfile.site,.env.example,.dockerignore,deploy.sh,smoke.sh} \
    h43:/root/hn-reader-test/
scp deploy/h43/Dockerfile h43:/root/hn-reader-test/Dockerfile

# .env: random DB password, LLM settings copied from the production .env
ssh h43 'set -e; cd /root/hn-reader-test; umask 077
  pw=$(openssl rand -hex 24)
  key=$(grep -m1 "^LLM_API_KEY=" /root/hn-reader/.env | cut -d= -f2-)
  base=$(grep -m1 "^LLM_BASE_URL=" /root/hn-reader/.env | cut -d= -f2-)
  model=$(grep -m1 "^LLM_MODEL=" /root/hn-reader/.env | cut -d= -f2-)
  sed -e "s|^POSTGRES_PASSWORD=.*|POSTGRES_PASSWORD=$pw|" \
      -e "s|^LLM_API_KEY=.*|LLM_API_KEY=$key|" \
      -e "s|^LLM_BASE_URL=.*|LLM_BASE_URL=$base|" \
      -e "s|^LLM_MODEL=.*|LLM_MODEL=$model|" \
      .env.example > .env
  chmod 600 .env'
```

Then deploy once:

```bash
# CI path (preferred: it also proves the whole pipeline)
gh workflow run deploy-staging.yml --ref <branch>
gh run watch
```

## Deploying

### Automatic (GitHub Actions)

`.github/workflows/deploy-staging.yml` deploys on:

- `push` to `dev`, `fix/**`, `feat/**` that touches `server/**`,
  `deploy/h43-test/**`, `deploy/h43/Dockerfile` or the workflow itself;
- `workflow_dispatch` on any **pushed** branch or tag (the workflow file must
  exist on that ref; `--ref` does not accept a raw SHA).

Pipeline: `go test` (both build tags) → cross-compile for linux/amd64 with
`-X main.buildSuffix=-staging.<sha>` → upload binary and deploy files → run
`deploy.sh` on the host → assert `/health` reports that sha. Anything red stops
before the host is touched; a failed health gate rolls the image back.

Concurrency is serialised (`cancel-in-progress: false`), so two quick pushes
deploy one after the other instead of fighting.

Required repository secrets/variables:

| Name | Kind | Value |
|---|---|---|
| `H43_SSH_KEY` | secret | private ed25519 key whose public half is in h43's `authorized_keys` (with `restrict`) |
| `H43_SSH_KNOWN_HOSTS` | secret | `ssh-keyscan -t ed25519 43.142.113.109` output, so the runner pins the host key |
| `H43_HOST` | variable | `43.142.113.109` |

The deploy key gets root on a public host; treat a leak as an incident (remove the
line from `authorized_keys`, rotate the secret). Note as pre-existing debt: h43's
sshd still allows password authentication.

### Manual

```bash
# build (on the workstation)
cd server
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags cloud -trimpath \
  -ldflags "-s -w -X main.buildSuffix=-staging.$(git rev-parse --short HEAD)" \
  -o ../deploy/h43-test/hn-reader-cloud ./cmd/hn-reader-cloud

scp deploy/h43-test/hn-reader-cloud h43:/root/hn-reader-test/hn-reader-cloud.new
ssh h43 'cd /root/hn-reader-test && ./deploy.sh --sha '"$(git rev-parse --short HEAD)" \
        "' --sha256 '"$(sha256sum deploy/h43-test/hn-reader-cloud | cut -d' ' -f1)"'"
```

`deploy.sh` refuses to run outside `/root/hn-reader-test`, verifies the uploaded
binary's checksum, installs the site fragment only when it changed (validate
before reload), swaps the binary, builds the image, recreates **only** `web`, and
then polls `http://127.0.0.1:8083/health` for the new version. On failure it
recreates the previously recorded image and says plainly whether that restored
service.

## The IP allowlist

`Caddyfile.site` is the only gate in front of a real LLM key:

```caddyfile
@blocked not remote_ip 171.88.165.161
respond @blocked "hn-reader staging: source IP not allowlisted" 403
```

Adding or replacing an address:

1. edit `deploy/h43-test/Caddyfile.site`;
2. deploy (CI) or `scp` it plus `./deploy.sh --sha <current>` — the fragment step
   compares, validates and reloads the edge;
3. if a browser still gets 403, read the address that actually connected:

   ```bash
   docker logs edge-caddy --since 10m 2>&1 | grep test.hn-reader.whoareyou.ren | tail -5
   ```

   and add that `remote_ip`. A browser behind `127.0.0.1:7897` leaves from the
   proxy's address, not the local one; either allowlist the proxy's egress or add
   a direct-connection rule for `*.whoareyou.ren`.

Keep single addresses. A `/24` would let strangers spend the staging budget.

## Why these values differ from production

| Variable | staging | production | Why |
|---|---|---|---|
| `LLM_DAILY_BUDGET` | 500 | 2000 | measured cost is ≈ $0.00003 per call, so 500/day ≈ $0.015; the cap exists to bound abuse, not to save money |
| `LLM_MAX_CONCURRENCY` | 2 | 4 | do not contend with production for the same provider account |
| `FREE_QUOTA_LIMIT` | 200 | 50 | repeated 15-article preload runs; set it to 5 and recreate to exercise the quota branch |
| `SIGNUP_RATE_*` | 20/h, 100/day | 5/h, 20/day | the allowlist already gates signup |
| `LLM_MAX_OUTPUT_TOKENS` | 1500 | 1500 (pending deploy) | staging is the evidence for that change, so the value must match |
| `LLM_REASONING_EFFORT` | `none` | `none` (pending deploy) | same reason |
| `PRELOAD_*`, `API_RATE_PER_MIN`, `MAX_BODY_BYTES`, `TRUSTED_PROXIES`, `HN_BASE_URL` | identical | identical | 15-of-20 pending with a 200-item queue is the configuration that exposed the "some articles never load" bug |

The LLM key is production's; the budget counter is per process, so staging spend
cannot exhaust production's allowance. To separate billing, change `LLM_API_KEY`
here only.

## Operating

```bash
# health, bypassing the allowlist and the edge
curl -sS http://127.0.0.1:8083/health

# smoke test (5 preloads + 1 summary; spends a little LLM budget)
/root/hn-reader-test/smoke.sh

# logs
docker logs -f hn-reader-test-server
docker logs --since 10m edge-caddy | grep test.hn-reader.whoareyou.ren
```

Every `docker compose` command in this directory needs `IMAGE_TAG`, which
`deploy.sh` exports and nothing stores in `.env`. Either read it back from
`.last-good-image` or export it once per shell:

```bash
cd /root/hn-reader-test
export IMAGE_TAG="$(cat .last-good-image)"
docker compose ps

# recreate only the app, keeping the database (also resets the in-process
# daily LLM counter)
docker compose up -d --force-recreate web

# wipe the database and start clean (schema is created on boot)
docker compose down -v && docker compose up -d
```

Rollback:

- automatic on a failed health gate (the image recorded in `.last-good-image`);
- `./deploy.sh --rollback` on the host, no CI needed;
- to a specific commit: `gh run list --workflow deploy-staging.yml`, then
  `gh run rerun <id>` of the last good run.

Tear down completely:

```bash
cd /root/hn-reader-test
IMAGE_TAG="$(cat .last-good-image)" docker compose down -v
rm -f /root/edge/sites/hn-reader-test.caddy
docker compose -f /root/edge/docker-compose.yml exec caddy \
  caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile
cd / && rm -rf /root/hn-reader-test
```

## Things that will confuse you

| Symptom | Cause |
|---|---|
| `https://...` handshake fails (`000`) | certificate not issued yet, or DNS points elsewhere: `docker logs edge-caddy | grep -i acme` |
| 403 with "not allowlisted" | your current egress IP is not in the fragment |
| The site works but `/health` says a version you did not deploy | you are looking at production's `X-Env` — staging always sends `X-Env: staging` and a `-staging-` version suffix |
| `docker compose` complains about `IMAGE_TAG` | it is exported by `deploy.sh`, not stored in `.env` |
| Push to `dev` deployed nothing | the workflow file is not on that ref (see the trigger rules above) |
| Preloads fail with `daily LLM budget exceeded` | the 500-call cap: `IMAGE_TAG=$(cat .last-good-image) docker compose up -d --force-recreate web` resets the in-process counter, or wait for the UTC day to roll over |
| `gh workflow run --ref <sha>` says no workflow | `--ref` takes a branch or tag, not a commit id |

## Not implemented here

No backups, no monitoring, no alerting, no automatic database snapshots — staging
data is disposable. Production keeps its own cron backup and is deployed by hand.
