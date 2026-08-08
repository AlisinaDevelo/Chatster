#!/usr/bin/env bash
set -euo pipefail

base_url="${1:-${CHATSTER_DEMO_URL:-}}"
if [[ -z "$base_url" ]]; then
	printf 'usage: %s <http(s)://host>\n' "$0" >&2
	exit 2
fi

base_url="${base_url%/}"
case "$base_url" in
	http://*|https://*) ;;
	*) printf 'deployment URL must start with http:// or https://\n' >&2; exit 2 ;;
esac

timeout_seconds="${CHATSTER_SMOKE_TIMEOUT_SECONDS:-15}"
if ! [[ "$timeout_seconds" =~ ^[1-9][0-9]*$ ]]; then
	printf 'CHATSTER_SMOKE_TIMEOUT_SECONDS must be a positive integer\n' >&2
	exit 2
fi

request() {
	curl --fail --silent --show-error --location --max-time "$timeout_seconds" "$base_url$1"
}

assert_contains() {
	local body="$1"
	local expected="$2"
	local label="$3"
	if ! grep -Fq "$expected" <<<"$body"; then
		printf '%s response did not contain the expected marker\n' "$label" >&2
		return 1
	fi
}

health="$(request /health)"
assert_contains "$health" '"status":"ok"' "/health"
assert_contains "$health" '"database":"ok"' "/health"

for path in /rooms/general /rooms/engineering; do
	request "$path" >/dev/null
done

metrics="$(request /metrics)"
assert_contains "$metrics" 'chatster_' "/metrics"

history="$(request '/api/messages?room=general&limit=1')"
assert_contains "$history" '"room":"general"' "/api/messages"
assert_contains "$history" '"messages"' "/api/messages"

printf 'deployment smoke passed: %s\n' "$base_url"
