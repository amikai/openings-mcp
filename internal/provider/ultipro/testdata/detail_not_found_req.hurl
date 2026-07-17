# Unlike iCIMS' 410, an unknown opportunityId is HTTP 200 here — the
# response is the bare client-side app shell with no
# CandidateOpportunityDetail payload. Callers must treat a 200 lacking the
# marker as not-found.
GET https://recruiting.ultipro.com/TEC1006TESER/JobBoard/18180d88-ced0-4361-bd09-d5eef66dab24/OpportunityDetail?opportunityId=00000000-0000-0000-0000-000000000000
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8

HTTP 200
[Asserts]
header "Content-Type" contains "text/html"
body not contains "CandidateOpportunityDetail"
