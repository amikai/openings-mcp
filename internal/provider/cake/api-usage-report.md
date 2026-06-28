# Cake.me Minimal Job Description API Report

Date: 2026-06-27

## Goal

Find the smallest practical Cake.me API workflow needed to search jobs and fetch
the job description content.

The useful workflow is two calls:

1. Search jobs to get a job `path`.
2. Fetch job detail by that `path` to get the full HTML `description` and
   `requirements`.

## Minimal Workflow

### 1. Search Jobs

Endpoint:

```text
POST https://api.cake.me/api/client/v1/jobs/search
```

Minimal request that succeeds:

```json
{
  "query": "Golang",
  "sort_by": "popularity",
  "filters": {}
}
```

Observed behavior:

- `query` is required.
- `sort_by` is required.
- `filters` is required, but `{}` is enough.
- `page` is optional and defaults to `1`.
- The API currently returns `20` jobs per page.
- Each normal result includes `path`, `title`, and a search-result
  `description`.

Example response summary:

```json
{
  "status": 200,
  "total_entries": 360,
  "total_pages": 18,
  "per_page": 20,
  "current_page": 1,
  "data_len": 20,
  "first_path": "senior-golang-web-backend-engineer-taoyuan"
}
```

### 2. Get Job Detail

Endpoint:

```text
GET https://api.cake.me/api/client/v1/jobs/{path}
```

Example:

```text
GET https://api.cake.me/api/client/v1/jobs/senior-golang-web-backend-engineer-taoyuan
```

Observed detail response includes:

```text
id
path
page_path
title
description
requirements
```

The detail `description` is not the same as the search-result `description`.
Observed differences:

- Search `description` is plain text and shorter.
- Detail `description` is HTML and fuller.
- Detail also includes `requirements`, which is separate HTML content and may
  be empty.

For the first tested Golang result:

```json
{
  "search_description_len": 170,
  "detail_description_len": 251,
  "requirements_len": 1051,
  "same_description": false
}
```

## Parameter Combinations Tested

### Search Minimality

| Case | Body | Status | Result |
| --- | --- | ---: | --- |
| Full normal | `query`, `page`, `sort_by`, `filters:{}` | 200 | Works |
| Omit `page` | `query`, `sort_by`, `filters:{}` | 200 | Works, page 1 |
| Latest sort, omit `page` | `query`, `sort_by:"latest"`, `filters:{}` | 200 | Works |
| Empty query, omit `page` | `query:""`, `sort_by`, `filters:{}` | 200 | Works |
| Only `query` | `query` | 422 | Missing `sort_by`, `filters` |
| Query and filters | `query`, `filters:{}` | 422 | Missing `sort_by` |
| Query and sort | `query`, `sort_by` | 422 | Missing `filters` |
| Omit filters | `query`, `page`, `sort_by` | 422 | Missing `filters` |
| Omit sort | `query`, `page`, `filters:{}` | 422 | Missing `sort_by` |
| Empty body | `{}` | 422 | Missing `query`, `sort_by`, `filters` |

### Normal Optional Filters

These are not required for job descriptions, but were tested to confirm the
minimal OpenAPI can allow filter objects without modeling every filter enum:

| Case | Body | Status |
| --- | --- | ---: |
| Location filter | `filters: { "locations": ["Taiwan"] }` | 200 |
| Salary filter | `filters: { "salary": { "currency": "TWD", "type": "per_month", "min": 60000, "max": 150000 } }` | 200 |

### Detail Endpoint

Eight normal `path` values returned from search were tested against:

```text
GET /api/client/v1/jobs/{path}
```

All returned `200` with a stable detail shape including `description` and
`requirements`.

## APIs That Are Not Needed For This Goal

`POST /api/client/v1/jobs/meta/batch` is not needed to fetch job descriptions.
It returns dynamic metadata such as popularity, application state, read rates,
and tech tags. None of those fields are required to retrieve the job detail
description.

The search response's `available_facets` is also not needed for the minimal
description workflow. It is useful for building a full filter UI, but not for:

1. Searching by keyword.
2. Getting a result `path`.
3. Fetching full job detail by path.

## OpenAPI Impact

The minimal OpenAPI should model only:

- `POST /api/client/v1/jobs/search`
- `GET /api/client/v1/jobs/{path}`
- Minimal search request fields: `query`, `sort_by`, `filters`, optional `page`
- Minimal search response fields: pagination plus `data[].path/title/description`
- Minimal detail response fields: `id`, `path`, `page_path`, `title`,
  `description`, `requirements`

This keeps the schema focused on the actual job-description workflow instead of
modeling the full Cake.me frontend.

