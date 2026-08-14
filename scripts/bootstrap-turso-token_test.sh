#!/usr/bin/env bash

set -Eeuo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
subject="${script_dir}/bootstrap-turso-token"
fake_bin="${script_dir}/testdata/bootstrap-turso-token/bin"
capture_dir="$(mktemp -d)"
trap 'rm -rf "$capture_dir"' EXIT

fail() {
  printf 'bootstrap-turso-token_test: %s\n' "$1" >&2
  exit 1
}

run_subject() {
  PATH="${fake_bin}:${PATH}" \
    FAKE_CAPTURE_DIR="$capture_dir" \
    "$subject" \
    --database bokiccio-demo \
    --database-url libsql://bokiccio-demo.example.turso.io \
    --project bokiccio-example \
    --secret bokiccio-demo-turso-token
}

output="$(run_subject)"
expected='{"database_url":"libsql://bokiccio-demo.example.turso.io","secret_id":"bokiccio-demo-turso-token","secret_version":"7"}'
[[ "$output" == "$expected" ]] || fail "success output must contain only non-secret metadata"

IFS= read -r captured_token <"${capture_dir}/secret-payload" || true
[[ "$captured_token" == "test-token-value" ]] || fail "token must be streamed to Secret Manager without a trailing newline"
[[ "$output" != *"test-token-value"* ]] || fail "token must not appear on stdout"

commands="$(<"${capture_dir}/commands")"
[[ "$commands" != *"test-token-value"* ]] || fail "token must not appear in command arguments"

: >"${capture_dir}/failure-stderr"
set +e
failure_output="$(PATH="${fake_bin}:${PATH}" \
  FAKE_CAPTURE_DIR="$capture_dir" \
  FAKE_GCLOUD_ADD_FAIL=1 \
  "$subject" \
  --database bokiccio-demo \
  --database-url libsql://bokiccio-demo.example.turso.io \
  --project bokiccio-example \
  --secret bokiccio-demo-turso-token 2>"${capture_dir}/failure-stderr")"
failure_status=$?
set -e

((failure_status != 0)) || fail "Secret Manager failure must fail the command"
[[ -z "$failure_output" ]] || fail "failure must not write to stdout"
failure_stderr="$(<"${capture_dir}/failure-stderr")"
[[ "$failure_stderr" == *"do not retry until Secret Manager versions are checked"* ]] || fail "failure must explain the safe recovery boundary"
[[ "$failure_stderr" != *"test-token-value"* ]] || fail "failure must not expose the token"

set +e
invalid_output="$({
  PATH="${fake_bin}:${PATH}" FAKE_CAPTURE_DIR="$capture_dir" "$subject" \
    --database 'Invalid Database' \
    --database-url libsql://bokiccio-demo.example.turso.io \
    --project bokiccio-example \
    --secret bokiccio-demo-turso-token
} 2>&1)"
invalid_status=$?
set -e

((invalid_status != 0)) || fail "invalid database name must be rejected"
[[ "$invalid_output" == *"valid Turso database name"* ]] || fail "invalid input must return a non-secret validation error"

printf 'bootstrap-turso-token tests passed\n'
