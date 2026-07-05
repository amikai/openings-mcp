package workday

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompanyByTenant(t *testing.T) {
	tests := []struct {
		name   string
		tenant string
		want   Company
		wantOk bool
	}{
		{
			name:   "exact lowercase match",
			tenant: "3m",
			want:   Company{Name: "3M", Tenant: "3m", Instance: "wd1", Site: "Search"},
			wantOk: true,
		},
		{
			name:   "case-insensitive match",
			tenant: "3M",
			want:   Company{Name: "3M", Tenant: "3m", Instance: "wd1", Site: "Search"},
			wantOk: true,
		},
		{
			name:   "another known tenant",
			tenant: "att",
			want:   Company{Name: "AT&T", Tenant: "att", Instance: "wd1", Site: "ATTGeneral"},
			wantOk: true,
		},
		{
			name:   "unknown tenant",
			tenant: "doesnotexist-tenant-xyz",
			want:   Company{},
			wantOk: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := CompanyByTenant(tt.tenant)
			assert.Equal(t, tt.wantOk, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCompanyBaseURL(t *testing.T) {
	c := Company{Tenant: "3m", Instance: "wd1", Site: "Search"}
	assert.Equal(t, "https://3m.wd1.myworkdayjobs.com/wday/cxs/3m/Search", c.BaseURL())
}

func TestCompaniesSortedAndComplete(t *testing.T) {
	cs := Companies()
	assert.Len(t, cs, 200)
	assert.True(t, sort.SliceIsSorted(cs, func(i, j int) bool { return cs[i].Name < cs[j].Name }))
}
