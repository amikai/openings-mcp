#!/bin/bash
curl -s \
  -H "User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36" \
  -H "Accept: application/json, text/plain, */*" \
  -H "Accept-Language: zh-TW,zh;q=0.9,en-US;q=0.8,en;q=0.7" \
  -H "Referer: https://www.104.com.tw/company/search/" \
  "https://www.104.com.tw/company/ajax/list?keyword=%E5%8F%B0%E7%A9%8D%E9%9B%BB&page=1&pageSize=10" \
  | jq . > companies_rsp.json
