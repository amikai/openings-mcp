package workday

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
			raw:  "https://stripe.wd5.myworkdayjobs.com/Stripe_Careers",
			want: CareersSite{Host: "stripe.wd5.myworkdayjobs.com", Tenant: "stripe", Site: "Stripe_Careers"},
			ok:   true,
		},
		{
			name: "locale prefix stripped",
			raw:  "https://stripe.wd5.myworkdayjobs.com/en-US/Stripe_Careers",
			want: CareersSite{Host: "stripe.wd5.myworkdayjobs.com", Tenant: "stripe", Site: "Stripe_Careers"},
			ok:   true,
		},
		{
			name: "lowercase locale and deep job link ignored",
			raw:  "https://acme.wd103.myworkdayjobs.com/zh-tw/jobs4acme/job/Taipei/Engineer_JR1",
			want: CareersSite{Host: "acme.wd103.myworkdayjobs.com", Tenant: "acme", Site: "jobs4acme"},
			ok:   true,
		},
		{
			// myworkdaysite.com is deliberately unsupported (#113): its
			// real shape puts the tenant in the path, and those tenants
			// are all reachable via myworkdayjobs.com instead.
			name: "myworkdaysite tenant-in-path rejected",
			raw:  "https://wd5.myworkdaysite.com/en-US/recruiting/devonenergy/Careers",
			ok:   false,
		},
		{name: "myworkdaysite four-label form rejected", raw: "https://acme.wd1.myworkdaysite.com/recruiting", ok: false},
		{
			name: "query and fragment ignored",
			raw:  "https://acme.wd1.myworkdayjobs.com/External?q=go#top",
			want: CareersSite{Host: "acme.wd1.myworkdayjobs.com", Tenant: "acme", Site: "External"},
			ok:   true,
		},
		{name: "no site segment", raw: "https://acme.wd1.myworkdayjobs.com/", ok: false},
		{name: "locale only, no site", raw: "https://acme.wd1.myworkdayjobs.com/en-US", ok: false},
		// Empty path segments are rejected (unlike the old Trim-based split,
		// which skipped a leading empty segment from "//Site").
		{name: "double slash rejected", raw: "https://acme.wd1.myworkdayjobs.com//External", ok: false},
		{name: "wrong domain", raw: "https://acme.wd1.example.com/External", ok: false},
		{name: "three host labels", raw: "https://www.myworkdayjobs.com/External", ok: false},
		{name: "five host labels", raw: "https://a.b.wd1.myworkdayjobs.com/External", ok: false},
		{name: "instance not wd-prefixed", raw: "https://acme.prod.myworkdayjobs.com/External", ok: false},
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
	s := CareersSite{Host: "stripe.wd5.myworkdayjobs.com", Tenant: "stripe", Site: "Stripe_Careers"}
	assert.Equal(t, "https://stripe.wd5.myworkdayjobs.com/wday/cxs/stripe/Stripe_Careers", s.BaseURL())
	assert.Equal(t, "https://stripe.wd5.myworkdayjobs.com/Stripe_Careers", s.CanonicalURL())
}

func TestCanonicalURLRoundTrips(t *testing.T) {
	// The ats layer circulates CanonicalURL as a slug and re-parses it, so
	// parse(canonical) must reproduce the same CareersSite.
	orig, ok := ParseCareersURL(mustParse(t, "https://stripe.wd5.myworkdayjobs.com/en-US/Stripe_Careers/job/SF/Eng_1"))
	require.True(t, ok)
	again, ok := ParseCareersURL(mustParse(t, orig.CanonicalURL()))
	require.True(t, ok)
	assert.Equal(t, orig, again)
}
