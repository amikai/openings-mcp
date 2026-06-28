#!/bin/bash
# COMPANY_CODE: a5h92m0 (TSMC)
COMPANY_CODE="a5h92m0"
curl -s \
  -H "User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36" \
  -H "Accept: application/json, text/plain, */*" \
  -H "Accept-Language: zh-TW,zh;q=0.9,en-US;q=0.8,en;q=0.7" \
  -H "Referer: https://www.104.com.tw/company/${COMPANY_CODE}" \
  "https://www.104.com.tw/api/companies/${COMPANY_CODE}/jobs?page=1&pageSize=10" \
  | jq . > company_jobs_rsp.json
