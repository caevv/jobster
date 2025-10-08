#!/usr/bin/env bash
#
# send-slack.sh - Jobster agent for sending Slack notifications
#
# Agent Contract:
# - Receives CONFIG_JSON env var with hook configuration (channel, message)
# - Receives job metadata via env vars (JOB_ID, RUN_ID, HOOK, etc.)
# - Requires SLACK_WEBHOOK_URL to be set in environment
# - Outputs JSON status to stdout
# - Exit 0 on success, non-zero on error
#
set -euo pipefail

# Read configuration from CONFIG_JSON env var
cfg="${CONFIG_JSON:-{}}"

# Extract channel and message from config (with defaults)
channel="$(echo "$cfg" | jq -r '.channel // "#ops"')"
message="$(echo "$cfg" | jq -r '.message // "Job finished"')"

# Check if SLACK_WEBHOOK_URL is set
if [[ -z "${SLACK_WEBHOOK_URL:-}" ]]; then
  echo '{"status":"error","error":"SLACK_WEBHOOK_URL not set"}' >&2
  exit 1
fi

# Build notification text with job metadata
notification_text="${message} (job=${JOB_ID:-unknown} hook=${HOOK:-unknown} run=${RUN_ID:-unknown})"

# Check for optional exit code in error hooks
if [[ -n "${EXIT_CODE:-}" ]]; then
  notification_text="${notification_text} exit_code=${EXIT_CODE}"
fi

# Create Slack payload
payload=$(jq -n \
  --arg text "$notification_text" \
  --arg channel "$channel" \
  '{channel: $channel, text: $text}')

# Send to Slack webhook
response=$(curl -sS -w "\n%{http_code}" \
  -H 'Content-Type: application/json' \
  -d "$payload" \
  "${SLACK_WEBHOOK_URL}")

# Extract HTTP status code (last line)
http_code=$(echo "$response" | tail -n1)

# Check if successful
if [[ "$http_code" == "200" ]]; then
  echo '{"status":"ok","metrics":{"notified":1},"channel":"'"$channel"'"}'
  exit 0
else
  echo '{"status":"error","error":"HTTP '"$http_code"'","metrics":{"notified":0}}' >&2
  exit 1
fi
