package ultipro

import "encoding/json"

// candidateOpportunityDetailMarker precedes the embedded posting object on
// OpportunityDetail's HTML page: `new US.Opportunity.CandidateOpportunityDetail({...})`.
const candidateOpportunityDetailMarker = "CandidateOpportunityDetail("

// extractOpportunityDetail finds and decodes the JSON object literal
// embedded after candidateOpportunityDetailMarker. ok is false when the
// marker is absent (the not-found app-shell response — see
// ErrJobNotFound) or the extracted text fails to parse as JSON.
func extractOpportunityDetail(html []byte) (*OpportunityDetail, bool) {
	start := indexAfter(html, candidateOpportunityDetailMarker)
	if start < 0 {
		return nil, false
	}
	end := balancedObjectEnd(html, start)
	if end < 0 {
		return nil, false
	}
	var detail OpportunityDetail
	if err := json.Unmarshal(html[start:end], &detail); err != nil {
		return nil, false
	}
	if detail.ID == "" || detail.Title == "" {
		return nil, false
	}
	return &detail, true
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
