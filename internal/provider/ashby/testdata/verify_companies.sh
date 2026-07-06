#!/bin/bash
# Re-verifies every board in ../companies.yaml against the live Ashby
# posting API: each must answer HTTP 200 with a non-empty jobs array.
# Boards that fail (org left Ashby / renamed its board) or report zero jobs
# (possibly abandoned) are flagged for manual review.
set -u
cd "$(dirname "$0")/.."
fail=0
while read -r board; do
  rsp=$(curl -s --max-time 60 "https://api.ashbyhq.com/posting-api/job-board/$board")
  n=$(printf '%s' "$rsp" | jq '.jobs | length' 2>/dev/null)
  if [ -z "$n" ] || [ "$n" = "null" ]; then
    echo "BAD  $board: not a job-board response"
    fail=1
  elif [ "$n" -eq 0 ]; then
    echo "WARN $board: 0 jobs (possibly abandoned board)"
  else
    echo "OK   $board: $n jobs"
  fi
done < <(awk '/^ *board:/ {gsub(/"/, "", $2); print $2}' companies.yaml)
exit $fail
