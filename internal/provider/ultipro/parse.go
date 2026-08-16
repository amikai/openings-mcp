package ultipro

import (
	"encoding/json"
	"fmt"
)

// candidateOpportunityDetailMarker precedes the embedded posting object on
// OpportunityDetail's HTML page: `new US.Opportunity.CandidateOpportunityDetail({...})`.
const candidateOpportunityDetailMarker = "CandidateOpportunityDetail("

// extractOpportunityDetail finds and decodes the JSON object literal
// embedded after candidateOpportunityDetailMarker. It returns
// ErrJobNotFound only when the marker itself is absent — the genuine
// not-found app-shell response (see openapi.yaml). Any other failure
// (unbalanced braces, invalid JSON, missing required fields) means the
// marker WAS present but the payload is malformed, so it's reported as a
// distinct error: an upstream template change or a parser bug should
// surface as a fetch failure, not get mistaken for a bad job id.
func extractOpportunityDetail(html []byte) (*OpportunityDetail, error) {
	start := indexAfter(html, candidateOpportunityDetailMarker)
	if start < 0 {
		return nil, ErrJobNotFound
	}
	end := balancedObjectEnd(html, start)
	if end < 0 {
		return nil, fmt.Errorf("unbalanced %s object", candidateOpportunityDetailMarker)
	}
	var detail OpportunityDetail
	if err := json.Unmarshal(html[start:end], &detail); err != nil {
		return nil, fmt.Errorf("decode %s object: %w", candidateOpportunityDetailMarker, err)
	}
	if detail.ID == "" || detail.Title == "" {
		return nil, fmt.Errorf("%s object missing required id/title", candidateOpportunityDetailMarker)
	}
	return &detail, nil
}

func indexAfter(html []byte, marker string) int {
	for i := 0; i+len(marker) <= len(html); i++ {
		if string(html[i:i+len(marker)]) == marker {
			return i + len(marker)
		}
	}
	return -1
}

// balancedObjectEnd returns the index one past the closing '}' that
// balances the '{' at or after start, tracking JSON string literals (with
// backslash escapes) so braces inside string values don't unbalance the
// count. Returns -1 if the object never closes.
func balancedObjectEnd(html []byte, start int) int {
	depth := 0
	inString := false
	escaped := false
	started := false
	for i := start; i < len(html); i++ {
		c := html[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
			started = true
		case '}':
			depth--
			if started && depth == 0 {
				return i + 1
			}
		}
	}
	return -1
}
