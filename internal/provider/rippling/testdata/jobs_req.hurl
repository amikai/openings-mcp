# Quirk: the list emits one entry per (job, location) pair, so a multi-location
# job appears once per workLocation with the same uuid (pythian: 33 entries,
# 12 distinct jobs). Consumers must dedupe by uuid.
GET https://api.rippling.com/platform/api/ats/v1/board/pythian/jobs

HTTP 200
