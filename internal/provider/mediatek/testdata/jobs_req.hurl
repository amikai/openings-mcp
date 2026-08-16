# Site-filtered search: Software jobs in HsinChu.
GET https://careers.mediatek.com/api/trpc/job.getJobs?input=%7B%22json%22%3A%7B%22locales%22%3A%22en_US%22%2C%22page%22%3A1%2C%22jobQueryInfo%22%3A%7B%22keywords%22%3A%5B%5D%2C%22relation%22%3A%22AND%22%7D%2C%22filters%22%3A%7B%22categorys%22%3A%5B%229020%22%5D%2C%22workExperiences%22%3A%5B%5D%2C%22locations%22%3A%5B%220000009256%22%5D%2C%22programs%22%3A%5B%5D%7D%2C%22sortBy%22%3A%22publishedDate%22%2C%22order%22%3A%22DESC%22%2C%22limit%22%3A6%7D%7D

HTTP 200

[Asserts]
jsonpath "$.result.data.json.status" == "complete"
jsonpath "$.result.data.json.pagination.total_items" == 31
jsonpath "$.result.data.json.jobs" count == 6
jsonpath "$.result.data.json.jobs[0].id" == "MTK120260629001"
