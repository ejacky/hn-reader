#!/bin/bash
# Deploy a staging build on h43. Runs ON h43, from /root/hn-reader-test.
#
#   ./deploy.sh --sha <short sha> [--sha256 <hex>]   deploy the uploaded binary
#   ./deploy.sh --rollback                           go back to .last-good-image
#
# The binary is cross-compiled elsewhere (CGO_ENABLED=0 GOOS=linux GOARCH=amd64
# -tags cloud) and uploaded as hn-reader-cloud.new; this script never compiles Go
# and never downloads anything.
#
# It touches exactly three places: this directory, one fragment under
# /root/edge/sites/, and the `edge` project's Caddy (graceful reload). It never
# references /root/hn-reader — production is not this script's business.
set -uo pipefail

DIR=/root/hn-reader-test
EDGE=/root/edge
SITES=$EDGE/sites
FRAGMENT=hn-reader-test.caddy
BINARY=hn-reader-cloud
NEW=$BINARY.new
PREV=$BINARY.prev
STATE=.last-good-image
HEALTH=http://127.0.0.1:8083/health
KEEP_IMAGES=3

SHA=""
SUM=""
ROLLBACK=0
while [ $# -gt 0 ]; do
	case "$1" in
		--sha)      SHA="${2:-}"; shift 2 ;;
		--sha256)   SUM="${2:-}"; shift 2 ;;
		--rollback) ROLLBACK=1; shift ;;
		*) echo "unknown argument: $1" >&2; exit 2 ;;
	esac
done

fail() { echo "FAIL $*" >&2; exit 1; }
ok()   { echo "OK   $*"; }

# The working directory decides which compose project is addressed. Refuse to
# guess: running this from /root/hn-reader would deploy staging over production.
cd "$DIR" 2>/dev/null || fail "cannot cd to $DIR"
[ "$(pwd)" = "$DIR" ] || fail "must run from $DIR (got $(pwd))"
[ -s .env ] || fail ".env is missing - run the bootstrap in README.md first"
command -v docker >/dev/null 2>&1 || fail "docker not on PATH"
command -v curl >/dev/null 2>&1 || fail "curl not on PATH"

health_version() {
	curl -fsS -m 5 "$HEALTH" 2>/dev/null | sed -n 's/.*"version":"\([^"]*\)".*/\1/p'
}

# wait_healthy <version substring, empty for any> - polls the container's own
# health endpoint over loopback, so the site's IP allowlist cannot make this
# check lie.
wait_healthy() {
	local want="$1" i got
	for i in $(seq 1 30); do
		got="$(health_version)"
		if [ -n "$got" ] && { [ -z "$want" ] || [[ "$got" == *"$want"* ]]; }; then
			ok "health answers version=$got"
			return 0
		fi
		sleep 2
	done
	return 1
}

edge_caddy() {
	docker compose -f "$EDGE/docker-compose.yml" exec -T caddy \
		caddy "$@" --config /etc/caddy/Caddyfile --adapter caddyfile
}

if [ "$ROLLBACK" = 1 ]; then
	[ -s "$STATE" ] || fail "no $STATE yet - nothing to roll back to"
	tag="$(cat "$STATE")"
	docker image inspect "$tag" >/dev/null 2>&1 || fail "the recorded image $tag is gone; redeploy the commit instead"
	ok "rolling back to $tag"
	IMAGE_TAG="$tag" docker compose up -d --force-recreate web >/dev/null 2>&1 \
		|| fail "compose up failed while rolling back"
	wait_healthy "" || fail "rolled back to $tag but health still fails - this needs a human"
	ok "rollback complete: staging answers on $tag"
	exit 0
fi

[ -n "$SHA" ] || fail "--sha is required"
[ ${#SHA} -ge 7 ] || fail "--sha looks too short: $SHA"
[ -s "$NEW" ] || fail "$NEW is missing or empty (upload the binary first)"

if [ -n "$SUM" ]; then
	got="$(sha256sum "$NEW" | cut -d' ' -f1)"
	[ "$got" = "$SUM" ] || fail "sha256 mismatch: got $got, expected $SUM (truncated upload?)"
	ok "sha256 verified"
fi

# 1. Site fragment. Installed under its real name before validating, because the
#    edge imports sites/*.caddy: a *.new file would simply not be part of the
#    config that caddy validate reads, so the check would pass for the wrong
#    reason. A bad fragment is put back and the reload never happens.
if [ -s Caddyfile.site ]; then
	[ -d "$SITES" ] || fail "$SITES is missing - is the edge installed?"
	if cmp -s Caddyfile.site "$SITES/$FRAGMENT"; then
		ok "site fragment unchanged"
	else
		cp -p "$SITES/$FRAGMENT" "$SITES/$FRAGMENT.prev" 2>/dev/null || true
		install -m 644 Caddyfile.site "$SITES/$FRAGMENT" || fail "cannot install the site fragment"
		if ! edge_caddy validate >/dev/null 2>&1; then
			if [ -f "$SITES/$FRAGMENT.prev" ]; then
				cp -p "$SITES/$FRAGMENT.prev" "$SITES/$FRAGMENT"
			else
				rm -f "$SITES/$FRAGMENT"
			fi
			fail "the new site fragment is invalid; the running configuration was never touched"
		fi
		edge_caddy reload >/dev/null 2>&1 \
			|| fail "caddy refused the reload; the previous configuration is still serving"
		ok "edge reloaded with the new fragment (previous kept as .prev)"
	fi
else
	ok "Caddyfile.site not present - ingress left alone"
fi

# 2. Swap the binary. The Dockerfile copies it from the build context, so an
#    in-place overwrite would be fine, but keeping the previous one makes a
#    manual rollback possible without CI.
chmod +x "$NEW" || fail "chmod $NEW"
[ -f "$BINARY" ] && { cp -p "$BINARY" "$PREV" || fail "keep previous binary"; }
mv -f "$NEW" "$BINARY" || fail "replace $BINARY"
ok "binary in place ($(sha256sum "$BINARY" | cut -c1-12))"

# 3. Image + container. Only `web` is recreated; the database is left running so
#    the staging data survives a code deploy.
export IMAGE_TAG="hn-reader-cloud:staging-$SHA"
docker build -f Dockerfile -t "$IMAGE_TAG" . >/dev/null 2>&1 || fail "docker build failed"
ok "image built: $IMAGE_TAG"
docker compose up -d --force-recreate web >/dev/null 2>&1 || fail "docker compose up failed"

# 4. Health gate. Failing here is not a warning: staging is in an unknown state
#    until it answers with the version that was just deployed.
if ! wait_healthy "$SHA"; then
	echo "FAIL the health endpoint never reported version=$SHA" >&2
	if [ -s "$STATE" ]; then
		prev_tag="$(cat "$STATE")"
		if docker image inspect "$prev_tag" >/dev/null 2>&1; then
			echo "FAIL rolling back to $prev_tag" >&2
			IMAGE_TAG="$prev_tag" docker compose up -d --force-recreate web >/dev/null 2>&1
			sleep 8
			if wait_healthy ""; then
				echo "FAIL rolled back to $prev_tag, which is serving again" >&2
			else
				echo "FAIL rolled back but staging is STILL unhealthy - this needs a human" >&2
			fi
		else
			echo "FAIL no local image $prev_tag to roll back to" >&2
		fi
	fi
	docker logs --tail 40 hn-reader-test-server 2>&1 | sed 's/^/    /' >&2
	exit 1
fi

printf '%s\n' "$IMAGE_TAG" > "$STATE"
ok "recorded $STATE = $IMAGE_TAG"

# 5. Keep the last few staging images for rollbacks; the rest only eat disk.
docker images --format '{{.Repository}}:{{.Tag}}' \
	| grep '^hn-reader-cloud:staging-' \
	| tail -n +$((KEEP_IMAGES + 1)) \
	| xargs -r docker rmi >/dev/null 2>&1 || true

echo
echo "staging is up: https://test.hn-reader.whoareyou.ren (loopback: $HEALTH)"
