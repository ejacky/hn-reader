# HN Reader v1.3.4 — per-user local server

## Release Title

HN Reader v1.3.4 — per-user local server, safe upgrades, and version visibility

## Release Notes

### Extension

- **Local mode now shows the running server version.** The popup displays
  `Local server vX.Y.Z` when it is connected to a local server.
- **Older local servers get an update link.** If the server version is lower than
  the extension version, the same row links to the latest GitHub release, so a
  user who upgraded the extension but not the installer can see why.

### Local server

- **Local data moves to the user profile.** The database, `.env` and log now live
  in `%LOCALAPPDATA%\HN Reader` instead of the program directory. The executable
  is installed under `%LOCALAPPDATA%\Programs\HN Reader`.
- **The startup log identifies the build.** It now prints
  `HN Reader server vX.Y.Z ...`, plus the data directory and database path, so
  troubleshooting no longer requires `curl /health` first.
- **Manual binary upgrades keep their data.** If the new data directory has no
  database but the old executable directory does, the server copies the database,
  WAL/SHM files and `.env` across; if that copy fails it keeps using the old path
  and logs why.

### Installer

- **Per-user installation, no service.** New installs do not create a Windows
  service and do not ask for administrator rights. The server starts at install
  time and again at every user logon through the HKCU Run entry.
- **One-time migration from the old service install.** If a machine-wide 1.3.3 or
  older install is detected, a small elevated helper stops and removes the
  `HNReader` service, copies the database and `.env` to the per-user data
  directory, and only then deletes the old files. This is the only UAC prompt;
  later upgrades run with `/S` and no elevation.
- **Uninstall keeps data by default.** The uninstaller asks before deleting the
  database and `.env`; silent uninstall keeps them, and
  `uninstall.exe /S /DELETE_DATA=1` is the explicit delete path.
- **Upgrades back up the database first.** The installer stops the server, waits
  for `hn-reader.exe` to really exit, copies the database (including WAL/SHM) and
  `.env` to `%LOCALAPPDATA%\HN Reader\backups\<timestamp>\`, and aborts if the
  backup fails. The five newest backups are kept.
- **Service stop waits for the process.** The old code treated `sc query` exit
  code 0 as "stopped"; the new code polls the process for up to 30 seconds and
  aborts with a clear message instead of overwriting a binary that is still in
  use.

### Cloud service

- No behaviour change. Production `/health` stays at `1.3.2`; this release ships
  the local server and extension only.

### Upgrade notes

- From `1.3.3` or older with the Windows service: expect **one** UAC prompt during
  the first v1.3.4 install, for the service removal/data migration. Every later
  upgrade is user-level.
- Local data remains compatible: the SQLite schema is unchanged; only its
  directory moves.
- The local server no longer runs before a user logs in. The extension only works
  in a logged-in browser session, so normal use is unchanged.

---

# HN Reader v1.3.3 — popup first screen and Chinese variants

## Release Title

HN Reader v1.3.3 — first-screen popup, Traditional Chinese, and archived stats

## Release Notes

### Extension

- The popup header now has a one-click **Open Hacker News** link. It opens
  `https://news.ycombinator.com/` in a new tab and works in both cloud and local
  modes, including when the service is unreachable or the quota is exhausted.
- **Language settings are collapsed by default.** The collapsed row shows the
  effective interface and summary language; the expanded state is remembered, and the
  auto-load quota warning stays inside the section with a small badge when it applies.
- **Traditional Chinese browsers get Chinese.** `zh-TW`, `zh-HK` and `zh-Hant` now
  load the `zh_CN` catalog explicitly, because Chrome does not match those locales to
  `_locales/zh_CN` and would otherwise fall back to English for both UI and summaries.
- **Stats are archived.** The stats section is hidden behind a low-weight **Stats**
  toggle at the bottom; no stats request is made when the popup opens, and the section
  reuses its result for the rest of the popup session.

### Infrastructure

- **Staging now deploys on every merge to `main`.** Before this, staging only followed
  feature branches, so the result of a merge — including how a conflict was resolved —
  was never executed anywhere before it reached production. Feature branches still
  deploy for pre-merge checks; tags do not deploy automatically (use
  `gh workflow run deploy-staging.yml --ref v<tag>` when a specific tag needs to be
  verified).
- **Issue closing now covers extension-only pushes.** `close-issues` was extracted into
  a reusable workflow called by both the cloud-server and extension-i18n pipelines after
  their own checks; previously it only ran for server/deploy changes, so `Done #N` in an
  extension commit never closed anything.

### Cloud service

- No server change in this release. Production `/health` stays at `1.3.2`; the `1.3.3`
  extension works against the existing API.

### Upgrade notes

- Extension `1.3.3`; cloud `/health` reports `1.3.2` (unchanged).

---

# HN Reader v1.3.2 — store metadata alignment

## Release Title

HN Reader v1.3.2 — description and locale metadata aligned with the package

## Release Notes

### Extension

- The **package description** — which Chrome Web Store reads as the listing summary and
  the browser shows on the extensions page — now says what the product is today:
  `AI summaries and personalized highlights for Hacker News. Free monthly quota, no setup required.`. The previous text still advertised cloud mode as a new thing and
  never mentioned the free monthly quota.
- `_locales/zh_CN` now carries a Chinese description instead of a copy of the English
  one, so the 中文（中国）listing no longer pairs an English summary with a Chinese
  description. That combination is what triggered "metadata mismatch" on 1.3.1.
- No functional change: the interface and behaviour are identical to 1.3.1.

### Cloud service

- Rebuilt at `1.3.2` so `/health` matches the tag. Behaviour identical to 1.3.1.

### Upgrade notes

- Extension `1.3.2`; cloud `/health` reports `1.3.2`.

---

# HN Reader v1.3.1 — Store package cleanup

## Release Title

HN Reader v1.3.1 — permission cleanup for the store package

## Release Notes

### Extension

- The published package no longer declares `https://test.hn-reader.whoareyou.ren/*`.
  That host existed only for local staging work, but Chrome Web Store would have
  shown it as a new host permission on the listing. The extension now declares
  exactly what it uses for users: Hacker News pages and the cloud API.
- Version `1.3.1`. No other functional change; everything in 1.3.0 still applies.

### Cloud service

- Rebuilt and redeployed at `1.3.1` so `/health` matches the release tag. Behaviour
  is identical to 1.3.0.

### Upgrade notes

- Extension `1.3.1`; cloud `/health` reports `1.3.1`.

---

# HN Reader v1.3.0 — English UI & Reliable Preloading

## Release Title

HN Reader v1.3.0 — English interface, i18n groundwork, and the preload fix

## Release Notes

### Extension

- **English by default.** The interface now follows the browser language through
  `_locales`: `en` is the default catalog and `zh_CN` is kept complete. A new
  setting overrides the browser (Auto / English / 中文) without changing Chrome's
  own language.
- **Summary language follows the interface** unless the user has chosen one.
- **Preload state is explicit.** Each story shows preloaded, or failed *with a
  reason*, instead of silently spinning; entries that never arrive expire after
  five minutes rather than hanging forever.
- Summary and preload requests give up after 20 seconds instead of waiting on a
  dead connection.

### Cloud service

- Reasoning models no longer return empty content: the output budget can be
  raised when a response is cut off by `finish_reason=length`, and
  `LLM_REASONING_EFFORT=none` is the default so chain-of-thought does not eat the
  whole budget.
- The preload endpoint answers with a status per article (`queued`, `cached`,
  `invalid`, `duplicate`, `user_limit`, `queue_full`, `quota_exceeded`), so the
  client can tell "still coming" from "never coming".
- `/health` reports the build's commit suffix, so a non-production image is
  distinguishable from the release one.

### Infrastructure

- New staging environment `https://test.hn-reader.whoareyou.ren` on h43: its own
  database, container, volume and port, an IP allowlist, and automatic deployment
  from GitHub Actions on pushes to `dev` / `fix/**` / `feat/**`.

### Upgrade notes

- Extension `1.3.0`; cloud `/health` reports `1.3.0`.
- Production environment: `LLM_MAX_OUTPUT_TOKENS=1500`, `LLM_REASONING_EFFORT=none`.

### Known issues

- Articles whose pages cannot be fetched from the h43 host (GitHub is
  unreachable, IPv6-only hosts, CDNs that block China or require a JS challenge)
  never produce a summary; the client now says so instead of waiting. Tracked in
  #11.

---

# HN Reader v1.2.0 — Cloud Hardening & Launch Readiness

## Release Title

HN Reader v1.2.0 — Cloud Hardening, Rate Limits, and Data Controls

## Release Notes

### Cloud service

- **SSRF protection**: article URLs are validated before fetching; private, loopback, link-local, CGNAT, and cloud-metadata addresses are rejected, including redirects and DNS rebinding.
- **Rate limits**: anonymous signup is limited per IP (5/hour, 20/day); API, summary, preload, signal, and account operations have per-user limits.
- **Cost guards**: global daily LLM budget (default 2000 calls), global concurrency limit (default 4), request timeout, and `max_tokens` cap.
- **Shared summary cache**: article summaries are cached by `(hn_id, lang)` and shared across anonymous users, so the same article is summarized once. Cache hits do not consume quota.
- **Bounded preload queue**: fixed workers, queue size, and per-user pending limit; preload reserves quota and refunds it on cache hits or failures.
- **Account controls**: export your data as JSON and delete the cloud account from the extension popup.
- **Server baseline**: HTTP timeouts, graceful shutdown, request-body limit, DB-backed `/health`, connection pool settings, and trusted-proxy handling for Caddy.
- **Correctness fixes**: anonymous signup no longer fails on `NULL` email; the cloud binary now requires `-tags cloud`; stale `X-Forwarded-For` cannot bypass rate limits.
- **Version**: cloud `/health` reports `1.2.0`.

### Extension

- Points at `https://hn-reader.whoareyou.ren`.
- Distinguishes quota, rate-limit, and service-busy errors.
- Removes the Pro/upgrade dead link for the free launch.
- Adds “导出我的数据” and “删除云端数据”.
- Fixes the permanent “cloud account not ready” state after a failed first signup; 401 responses now rebuild the anonymous token once.
- Version `1.2.0`.

### Cloud deployment

```bash
# on the build machine
cd server
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags cloud -trimpath \
  -ldflags "-s -w" -o hn-reader-cloud ./cmd/hn-reader-cloud

# on h43 (see deploy/h43/README.md for the full procedure)
cd /root/hn-reader
docker build -t hn-reader-cloud:1.2.0 .
docker compose up -d server
```

### Local mode

Local mode remains available for privacy-conscious users. The Windows installer and local server are unchanged; the extension can switch modes from the popup.

### Upgrade notes

- The database migration is additive: a `global_articles` table plus an idempotent `users.email` fix. The previous `1.1.1` image can be used for rollback.
- Keep the temporary Caddy IP allowlist until the P0 acceptance checks pass, then remove it and reload Caddy.
- Existing anonymous tokens remain valid. Tokens invalidated by a database reset are recreated by the extension on the next 401.

### Privacy

Cloud mode stores anonymous reading signals and generated summaries to provide recommendations and enforce quota. Article/comment text is sent to the configured LLM provider only to generate the requested summary. See [PRIVACY.md](PRIVACY.md).
