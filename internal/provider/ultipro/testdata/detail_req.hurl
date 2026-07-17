# Job detail is server-rendered HTML with the full posting embedded as a
# JSON object literal after `new US.Opportunity.CandidateOpportunityDetail(`
# — there is no separate JSON API. postingId is not required; opportunityId
# alone resolves the posting.
GET https://recruiting.ultipro.com/TEC1006TESER/JobBoard/18180d88-ced0-4361-bd09-d5eef66dab24/OpportunityDetail?opportunityId=0b81b2f5-ffe3-4604-93b8-19d912c2424f
User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8

HTTP 200
[Asserts]
header "Content-Type" contains "text/html"
body contains "CandidateOpportunityDetail"
