#!/usr/bin/env bash
#
# cleanup.sh - Jobster agent for cleanup tasks
#
# Agent Contract:
# - Receives CONFIG_JSON env var with hook configuration (targets, max_age_days)
# - Receives job metadata via env vars (JOB_ID, RUN_ID, HOOK, etc.)
# - Can perform cleanup tasks like deleting old files, clearing caches
# - Outputs JSON status to stdout
# - Exit 0 on success, non-zero on error
#
set -euo pipefail

# Read configuration from CONFIG_JSON env var
cfg="${CONFIG_JSON:-{}}"

# Extract configuration options
targets=$(echo "$cfg" | jq -r '.targets // [] | .[]' 2>/dev/null || echo "")
max_age_days=$(echo "$cfg" | jq -r '.max_age_days // 30')
dry_run=$(echo "$cfg" | jq -r '.dry_run // "true"')

# Initialize counters
files_cleaned=0
dirs_cleaned=0
bytes_freed=0

# Log what we're doing
log_msg="Cleanup agent running for job ${JOB_ID:-unknown}"
echo "$log_msg" >&2

# Function to clean old files in a directory
cleanup_directory() {
  local target="$1"
  local age="$2"
  local is_dry_run="$3"

  if [[ ! -d "$target" ]]; then
    echo "Target directory does not exist: $target" >&2
    return 1
  fi

  # Find files older than max_age_days
  # This is a safe example that just logs what would be deleted
  while IFS= read -r -d '' file; do
    size=$(stat -f%z "$file" 2>/dev/null || stat -c%s "$file" 2>/dev/null || echo 0)

    if [[ "$is_dry_run" == "true" ]]; then
      echo "Would delete: $file (size: $size bytes)" >&2
      ((files_cleaned++)) || true
      ((bytes_freed+=size)) || true
    else
      echo "Deleting: $file (size: $size bytes)" >&2
      rm -f "$file" && {
        ((files_cleaned++)) || true
        ((bytes_freed+=size)) || true
      }
    fi
  done < <(find "$target" -type f -mtime "+$age" -print0 2>/dev/null)
}

# Process each target directory
if [[ -n "$targets" ]]; then
  while IFS= read -r target; do
    if [[ -n "$target" ]]; then
      cleanup_directory "$target" "$max_age_days" "$dry_run" || true
    fi
  done <<< "$targets"
else
  # Default behavior: log that we would clean temp files
  echo "No targets specified. Would clean default temp locations." >&2
  echo "Example: /tmp/jobster-* older than $max_age_days days" >&2
fi

# Output JSON status
output=$(jq -n \
  --arg status "ok" \
  --argjson files "$files_cleaned" \
  --argjson dirs "$dirs_cleaned" \
  --argjson bytes "$bytes_freed" \
  --arg dry_run "$dry_run" \
  --arg note "Cleanup completed (dry_run=$dry_run)" \
  '{
    status: $status,
    metrics: {
      files_cleaned: $files,
      dirs_cleaned: $dirs,
      bytes_freed: $bytes
    },
    dry_run: ($dry_run == "true"),
    notes: $note
  }')

echo "$output"
exit 0
