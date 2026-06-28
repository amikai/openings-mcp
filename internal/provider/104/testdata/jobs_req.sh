#!/bin/bash
curl -s \
  -H "User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36" \
  -H "Accept: application/json, text/plain, */*" \
  -H "Accept-Language: zh-TW,zh;q=0.9,en-US;q=0.8,en;q=0.7" \
  -H "Referer: https://www.104.com.tw/jobs/search/" \
  "https://www.104.com.tw/jobs/search/api/jobs?keyword=Golang&area=6001001000&asc=0&jobsource=2018indexpoc" \
  | jq . > jobs_rsp.json
