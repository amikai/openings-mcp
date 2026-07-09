POST https://api.cake.me/api/client/v1/jobs/search
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36
Accept: application/json, text/plain, */*
Content-Type: application/json
Referer: https://www.cake.me/jobs
{
  "query": "Golang",
  "page": 1,
  "per_page": 20,
  "sort_by": "popularity",
  "filters": {
    "locations": ["台灣"],
    "professions": ["it_back-end-engineer"],
    "job_types": ["full_time"],
    "seniority_levels": ["mid_senior_level"],
    "year_of_seniority": ["5_10"],
    "number_of_management": ["none"],
    "remote": ["partial_remote_work"],
    "inclusivity_traits": ["lgbtq"],
    "lang_names": ["Chinese"],
    "salary": {
      "type": "per_month",
      "currency": "TWD",
      "min": 60000,
      "max": 150000
    },
    "page": {
      "number_of_employees": ["51_200"],
      "sectors": ["tech_software"],
      "tech_labels": ["go"]
    }
  }
}

HTTP 200
