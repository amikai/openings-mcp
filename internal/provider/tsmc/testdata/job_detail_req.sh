#!/bin/bash
# Job ID: 21826 (R&D Advanced Packaging Integration Engineer, Taiwan)
JOB_ID="21826"
curl -s \
  -A "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36" \
  -H "Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8" \
  -H "Accept-Language: zh-TW,zh;q=0.9,en;q=0.8" \
  -H "Referer: https://careers.tsmc.com/zh_TW/careers/SearchJobs" \
  "https://careers.tsmc.com/zh_TW/careers/JobDetail?jobId=${JOB_ID}&source=External+Career+Site" \
  > job_detail_rsp.html
