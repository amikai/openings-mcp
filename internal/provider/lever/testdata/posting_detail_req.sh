#!/bin/bash
# Fetches the first posting in postings_rsp.json; run postings_req.sh first.
id=$(jq -r '.[0].id' postings_rsp.json)
curl -s "https://api.lever.co/v0/postings/leverdemo/${id}?mode=json" \
  | jq . > posting_detail_rsp.json
