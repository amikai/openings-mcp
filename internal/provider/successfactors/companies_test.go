package successfactors

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompaniesAreSortedAndIndexed(t *testing.T) {
	require.NotEmpty(t, Companies)
	for i, c := range Companies {
		assert.Equal(t, c, CompaniesByHost[c.Host])
		if i > 0 {
			assert.Less(t, Companies[i-1].Name, c.Name)
		}
	}
}

func TestLoadCompaniesRejectsDuplicateHost(t *testing.T) {
	_, err := loadCompanies([]byte(`- company: Acme
  host: jobs.acme.com
- company: Other Acme
  host: jobs.acme.com
`))
	assert.ErrorContains(t, err, "duplicate host")
}

func TestLoadCompaniesRejectsInvalidHost(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "uppercase", data: "- company: Acme\n  host: Jobs.acme.com\n"},
		{name: "port", data: "- company: Acme\n  host: jobs.acme.com:443\n"},
		{name: "missing", data: "- company: Acme\n  host: \"\"\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadCompanies([]byte(tt.data))
			assert.Error(t, err)
		})
	}
}

func TestCareersURL(t *testing.T) {
	c := Company{Name: "SAP", Host: "jobs.sap.com"}
	assert.Equal(t, "https://jobs.sap.com/search/", c.CareersURL())
}
