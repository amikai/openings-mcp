package avature

import (
	"strings"
	"testing"
)

func TestCompaniesLoad(t *testing.T) {
	if len(Companies) == 0 {
		t.Fatal("no companies loaded")
	}
	for _, c := range Companies {
		if got, ok := CompaniesBySlug[strings.ToLower(c.Slug())]; !ok || got.Name != c.Name {
			t.Errorf("CompaniesBySlug missing %q", c.Slug())
		}
	}
}

func TestLoadCompaniesRejectsBadEntries(t *testing.T) {
	cases := map[string]string{
		"empty name":     `[{"company": "", "url": "https://x.avature.net/careers"}]`,
		"http scheme":    `[{"company": "X", "url": "http://x.avature.net/careers"}]`,
		"no portal":      `[{"company": "X", "url": "https://x.avature.net"}]`,
		"locale segment": `[{"company": "X", "url": "https://x.avature.net/en_US"}]`,
		"deep path":      `[{"company": "X", "url": "https://x.avature.net/en_US/careers"}]`,
		"trailing slash": `[{"company": "X", "url": "https://x.avature.net/careers/"}]`,
		"duplicate slug": `[{"company": "A", "url": "https://x.avature.net/careers"}, {"company": "B", "url": "https://x.avature.net/careers"}]`,
		"duplicate name": `[{"company": "A", "url": "https://x.avature.net/careers"}, {"company": "A", "url": "https://y.avature.net/careers"}]`,
		"unsorted":       `[{"company": "B", "url": "https://b.avature.net/careers"}, {"company": "A", "url": "https://a.avature.net/careers"}]`,
	}
	for name, data := range cases {
		if _, err := loadCompanies([]byte(data)); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}
