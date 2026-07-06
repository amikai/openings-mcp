#!/bin/bash
# Captures Ashby public job-board fixtures from the real `browserbase` board
# (small - 5 jobs at capture time - but exercises secondaryLocations,
# streetAddress, compensation tiers, and null tier titles).
BASE="https://api.ashbyhq.com/posting-api/job-board/browserbase"
curl -s "$BASE" | jq . > board_rsp.json
curl -s "$BASE?includeCompensation=true" | jq . > board_comp_rsp.json
