#!/bin/bash
# Search: engineer, Taiwan (1277=13209), R&D (558=38617), Engineer/Admin (147=5709), Regular (542=5701)
curl -s \
  -A "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36" \
  -H "Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8" \
  -H "Accept-Language: zh-TW,zh;q=0.9,en;q=0.8" \
  "https://careers.tsmc.com/zh_TW/careers/SearchJobs/engineer?listFilterMode=1&jobRecordsPerPage=10&1277=13209&558=38617&147=5709&542=5701" \
  > search_jobs_rsp.html
