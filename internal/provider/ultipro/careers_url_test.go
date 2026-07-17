package ultipro

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParse(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u
}

func TestParseCareersURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want CareersSite
		ok   bool
	}{
		{
			name: "plain",
			raw:  "https://recruiting.ultipro.com/TEC1006TESER/JobBoard/18180d88-ced0-4361-bd09-d5eef66dab24/",
			want: CareersSite{Host: "recruiting.ultipro.com", CompanyCode: "TEC1006TESER", BoardID: "18180d88-ced0-4361-bd09-d5eef66dab24"},
			ok:   true,
		},
		{
			name: "second host and deep opportunity link ignored",
			raw:  "https://recruiting2.ultipro.com/SAL1002/JobBoard/bcc2e2d1-d94c-2041-4126-28086417eb0a/OpportunityDetail?opportunityId=abc",
			want: CareersSite{Host: "recruiting2.ultipro.com", CompanyCode: "SAL1002", BoardID: "bcc2e2d1-d94c-2041-4126-28086417eb0a"},
			ok:   true,
		},
		{
			name: "uppercase board id lowercased",
			raw:  "https://recruiting.ultipro.com/TEC1006TESER/JobBoard/18180D88-CED0-4361-BD09-D5EEF66DAB24/",
			want: CareersSite{Host: "recruiting.ultipro.com", CompanyCode: "TEC1006TESER", BoardID: "18180d88-ced0-4361-bd09-d5eef66dab24"},
			ok:   true,
		},
		{name: "wrong domain", raw: "https://recruiting.example.com/TEC1006TESER/JobBoard/18180d88-ced0-4361-bd09-d5eef66dab24/", ok: false},
		{name: "no board segment", raw: "https://recruiting.ultipro.com/TEC1006TESER/", ok: false},
		{name: "not ultipro at all", raw: "https://jobs.smartrecruiters.com/Equinox", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseCareersURL(mustParse(t, tt.raw))
			require.Equal(t, tt.ok, ok)
			if ok {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestCareersSiteURLs(t *testing.T) {
	s := CareersSite{Host: "recruiting.ultipro.com", CompanyCode: "TEC1006TESER", BoardID: "18180d88-ced0-4361-bd09-d5eef66dab24"}
	assert.Equal(t, "https://recruiting.ultipro.com/TEC1006TESER/JobBoard/18180d88-ced0-4361-bd09-d5eef66dab24", s.BaseURL())
	assert.Equal(t, "https://recruiting.ultipro.com/TEC1006TESER/JobBoard/18180d88-ced0-4361-bd09-d5eef66dab24/", s.CanonicalURL())
}

func TestCanonicalURLRoundTrips(t *testing.T) {
	orig, ok := ParseCareersURL(mustParse(t, "https://recruiting.ultipro.com/TEC1006TESER/JobBoard/18180d88-ced0-4361-bd09-d5eef66dab24/OpportunityDetail?opportunityId=abc"))
	require.True(t, ok)
	again, ok := ParseCareersURL(mustParse(t, orig.CanonicalURL()))
	require.True(t, ok)
	assert.Equal(t, orig, again)
}
