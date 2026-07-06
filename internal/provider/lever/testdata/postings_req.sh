#!/bin/bash
curl -s "https://api.lever.co/v0/postings/leverdemo?mode=json&limit=3" \
  | jq . > postings_rsp.json
