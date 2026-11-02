#!/bin/bash
# Fast end-to-end smoke test for staging.
#
# Run on h43 (defaults to the container's loopback port, so the site's IP
# allowlist cannot interfere):
#   /root/hn-reader-test/smoke.sh
#
# Or against the public URL from an allowlisted machine:
#   BASE=https://test.hn-reader.whoareyou.ren ./smoke.sh
#
# It spends a handful of LLM calls (a few preloads + one summarize), which is why
# the CI workflow only runs it on request.
#
# What it can and cannot assert: the sum of the checks below is "the API is
# wired up and the preload pipeline moves". It is NOT the 15-article acceptance
# test - article fetching depends on real pages (YouTube, IPv6-only hosts) and
# some of them legitimately never produce a summary. That acceptance belongs to
# the extension running against the real front page.
set -uo pipefail

BASE=${BASE:-http://127.0.0.1:8083}
HN_BASE=${HN_BASE:-https://hacker-news.firebaseio.com/v0}
LANG_CODE=${LANG_CODE:-en}
POLL_TIMEOUT=${POLL_TIMEOUT:-180}
BATCH=${BATCH:-3}

pass=0
ok()  { echo "OK   $*"; pass=$((pass + 1)); }
bad() { echo "FAIL $*" >&2; exit 1; }

code() { curl -sS -m 30 -o /tmp/smoke.body -w '%{http_code}' "$@" 2>/dev/null; }

echo "smoke: $BASE"

# 1. Health. A 200 here also proves the database answers (handleHealth pings it).
c=$(code "$BASE/health")
[ "$c" = "200" ] || bad "GET /health returned $c (body: $(cat /tmp/smoke.body))"
grep -q '"status":"ok"' /tmp/smoke.body || bad "GET /health body is not ok: $(cat /tmp/smoke.body)"
version=$(sed -n 's/.*"version":"\([^"]*\)".*/\1/p' /tmp/smoke.body)
ok "health 200, version=$version"

# 2. Anonymous signup.
c=$(code -X POST "$BASE/auth/anonymous" -H 'Content-Type: application/json' -d '{}')
[ "$c" = "200" ] || bad "POST /auth/anonymous returned $c (body: $(cat /tmp/smoke.body))"
token=$(sed -n 's/.*"token":"\([^"]*\)".*/\1/p' /tmp/smoke.body)
[ -n "$token" ] || bad "no token in the signup response"
ok "anonymous account created"

# 3. Authenticated read, plus the quota fields the client relies on.
c=$(code "$BASE/auth/me" -H "Authorization: Bearer $token")
[ "$c" = "200" ] || bad "GET /auth/me returned $c"
grep -q '"quota_limit"' /tmp/smoke.body || bad "GET /auth/me has no quota_limit: $(cat /tmp/smoke.body)"
ok "auth/me 200"

# 4. SSRF guard. The summarizer resolves the story through the HN API and uses
#    THAT canonical URL, so a client-supplied one is ignored for a real id. The
#    interesting case is an id the API does not know: then there is no canonical
#    URL to fall back on, and a link-local metadata address must be refused
#    outright (invalid_hn_id here, invalid_url if the lookup fails transiently).
c=$(code -X POST "$BASE/api/summarize/article" \
	-H "Authorization: Bearer $token" -H 'Content-Type: application/json' \
	-d '{"hn_id":999999999,"url":"http://169.254.169.254/latest/meta-data","title":"x","lang":"en"}')
[ "$c" = "400" ] || bad "metadata-URL attempt returned $c, expected 400 (body: $(cat /tmp/smoke.body))"
grep -Eq 'invalid_hn_id|invalid_url' /tmp/smoke.body \
	|| bad "metadata-URL attempt was not rejected as invalid_hn_id/invalid_url: $(cat /tmp/smoke.body)"
ok "metadata URL never fetched (400 on an unresolvable story id)"

# 5. Preload a few of the CURRENT top stories. The ids have to be real: the
#    server resolves each id back to its canonical story and drops anything the
#    HN API does not know ("is not a story"), so hand-made ids never become
#    ready. Titles are ignored too - only the id matters.
ids=$(curl -fsS -m 20 "$HN_BASE/topstories.json" 2>/dev/null | tr -d '[]' | tr ',' '\n' | tr -d ' ' | head -n "$BATCH")
count=$(printf '%s\n' "$ids" | grep -c '[0-9]')
[ "$count" = "$BATCH" ] || bad "could not read $BATCH story ids from the HN API (got: $ids)"
payload='{"articles":['
first=1
for id in $ids; do
	[ $first = 1 ] || payload="$payload,"
	first=0
	payload="$payload{\"hn_id\":$id,\"url\":\"\",\"title\":\"HN item $id\",\"lang\":\"$LANG_CODE\"}"
done
payload="$payload]}"

c=$(code -X POST "$BASE/api/summarize/article/preload" \
	-H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "$payload")
[ "$c" = "200" ] || bad "POST preload returned $c (body: $(cat /tmp/smoke.body))"
queued=$(sed -n 's/.*"queued":\([0-9]*\).*/\1/p' /tmp/smoke.body)
skipped=$(sed -n 's/.*"skipped":\([0-9]*\).*/\1/p' /tmp/smoke.body)
[ -n "$queued" ] && [ -n "$skipped" ] || bad "preload response lacks queued/skipped: $(cat /tmp/smoke.body)"
total=$((queued + skipped))
[ "$total" = "$BATCH" ] || bad "queued($queued)+skipped($skipped) != $BATCH - some articles were not classified: $(cat /tmp/smoke.body)"
ok "preload classified all $BATCH (queued=$queued skipped=$skipped)"

# 6. Poll until the pipeline produces something. Requiring every article would
#    be dishonest: unfetchable pages (YouTube over IPv6, 403s) legitimately never
#    produce a summary. What must hold is that the queue moves and the status
#    endpoint answers.
ids_csv=$(echo $ids | tr ' ' ',')
deadline=$(( $(date +%s) + POLL_TIMEOUT ))
ready=0
while [ "$(date +%s)" -lt "$deadline" ]; do
	c=$(code "$BASE/api/summarize/article/preload-status?ids=$ids_csv&lang=$LANG_CODE" \
		-H "Authorization: Bearer $token")
	[ "$c" = "200" ] || bad "GET preload-status returned $c"
	list=$(sed -n 's/.*"ready_ids":\[//p' /tmp/smoke.body | sed 's/\].*//')
	ready=0
	if [ -n "$list" ]; then
		ready=$(printf '%s' "$list" | tr ',' '\n' | grep -c '[0-9]')
	fi
	[ "$ready" -ge 1 ] && break
	sleep 5
done
[ "$ready" -ge 1 ] || bad "0/$BATCH preloads became ready in ${POLL_TIMEOUT}s (check docker logs hn-reader-test-server)"
[ "$ready" -ge "$BATCH" ] && ok "preload ready: $ready/$BATCH" || ok "preload moved: $ready/$BATCH ready (the rest may legitimately be unfetchable - see server logs)"

# 7. One real summary of the first story, to prove the stored text is usable.
first_id=$(echo $ids | cut -d' ' -f1)
c=$(code -m 120 -X POST "$BASE/api/summarize/article" \
	-H "Authorization: Bearer $token" -H 'Content-Type: application/json' \
	-d "{\"hn_id\":$first_id,\"url\":\"\",\"title\":\"HN item $first_id\",\"lang\":\"$LANG_CODE\"}")
[ "$c" = "200" ] || bad "POST summarize returned $c (body: $(cat /tmp/smoke.body))"
summary=$(sed -n 's/.*"summary":"\([^"]\{0,60\}\).*/\1/p' /tmp/smoke.body)
[ -n "$summary" ] || bad "summarize returned an empty summary: $(cat /tmp/smoke.body)"
ok "summary: ${summary}..."

rm -f /tmp/smoke.body
echo
echo "smoke passed ($pass checks) against $BASE"
