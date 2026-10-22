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
