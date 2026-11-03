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
  from GitHub Actions on pushes to `main` (every merge) and to
  `dev` / `fix/**` / `feat/**` for pre-merge checks.

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
