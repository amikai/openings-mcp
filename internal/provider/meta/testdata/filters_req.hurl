POST https://www.metacareers.com/graphql
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36
X-FB-LSD: openings-mcp
Origin: https://www.metacareers.com
Referer: https://www.metacareers.com/jobsearch/
Sec-Fetch-Dest: empty
Sec-Fetch-Mode: cors
Sec-Fetch-Site: same-origin
[Form]
lsd: openings-mcp
variables: {}
doc_id: 25103492705924273

HTTP 200
# Meta's /graphql answers JSON with `Content-Type: text/html; charset="utf-8"`;
# hurl rejects the quoted charset and cannot decode the body, so asserts use raw bytes.
[Asserts]
bytes startsWith hex,7b226461746122;
bytes contains hex,6a6f625f7365617263685f66696c74657273;
bytes contains hex,7465616d5f646973706c61795f6e616d65;
bytes contains hex,746563686e6f6c6f67696573;
bytes contains hex,22726f6c657322;
