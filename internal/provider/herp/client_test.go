package herp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amikai/openings-mcp/internal/provider/herp"
)

func TestGetCompany(t *testing.T) {
	srv := herp.NewMockServer()
	defer srv.Close()

	client, err := herp.NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetCompany(t.Context(), herp.GetCompanyParams{Slug: herp.MockSlug})
	require.NoError(t, err)

	rsp, ok := res.(*herp.CompanyResponse)
	require.True(t, ok, "want *CompanyResponse, got %T", res)

	company := rsp.Company
	assert.Equal(t, "株式会社Helpfeel", company.CompanyName)
	assert.Equal(t, herp.MockSlug, company.CompanySlug)
	assert.Equal(t, herp.NewOptNilFloat64(33.3), company.CompanyCumulativeFunding)
	assert.Equal(t, herp.NewOptNilInt(245), company.CompanyNumberOfEmployees)
	require.NotEmpty(t, company.Directors)
	require.NotEmpty(t, company.FundingHistories)
	require.NotEmpty(t, company.NumberOfEmployeesHistories)

	require.NotEmpty(t, company.Jobs)
	job := company.Jobs[0]
	assert.Equal(t, "ZluM2laebsdh", job.ID)
	assert.Equal(t, "100｜インサイドセールス", job.Name)
	assert.Equal(t, "正社員（期間の定めなし）", job.FormOfEmployment)
	assert.Equal(t, "2026-02-03T07:50:41.000Z", job.JobPublishedAt)
	assert.Equal(t, "2026-02-24T10:17:01.000Z", job.UpdatedAt)
	assert.Equal(
		t,
		herp.NewOptNilJobJobRemoteworkType(herp.JobJobRemoteworkTypeFULLREMOTEWORK),
		job.JobRemoteworkType,
	)
	assert.Equal(t, []herp.JobJobEmploymentTypeIdsItem{herp.JobJobEmploymentTypeIdsItemFULLTIME}, job.JobEmploymentTypeIds)

	// A posting carries the parsed salary bounds alongside the free text the
	// company wrote, and either bound can be missing on its own.
	require.True(t, job.Salary.Set)
	assert.Equal(t, herp.NewOptNilInt(4000000), job.Salary.Value.Minimum)
	assert.Equal(t, herp.OptNilInt{Set: true, Null: true}, job.Salary.Value.Maximum)
	assert.Equal(t, herp.NewOptNilSalaryPeriod(herp.SalaryPeriodANNUAL), job.Salary.Value.Period)

	require.Len(t, job.JobRoles, 1)
	assert.Equal(t, "inside_sales", job.JobRoles[0].ID)
	assert.Equal(t, "インサイドセールス", job.JobRoles[0].Name)
	assert.Equal(t, "セールス・事業開発", job.JobRoles[0].ParentJobRoleName)

	// A posting can span prefectures, and only some entries name a city.
	require.Len(t, job.JobLocations, 2)
	assert.Equal(t, "13", job.JobLocations[0].PrefCode)
	assert.Equal(t, "東京都", job.JobLocations[0].PrefName)
	assert.Equal(t, herp.NewOptNilString("中央区"), job.JobLocations[0].CityName)
	assert.Equal(t, "京都府", job.JobLocations[1].PrefName)
	assert.Equal(t, herp.OptNilString{Set: true, Null: true}, job.JobLocations[1].CityName)
}

func TestGetCompanyNotFound(t *testing.T) {
	srv := herp.NewMockServer()
	defer srv.Close()

	client, err := herp.NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetCompany(t.Context(), herp.GetCompanyParams{Slug: herp.MockUnknownSlug})
	require.NoError(t, err)

	rsp, ok := res.(*herp.ErrorResponse)
	require.True(t, ok, "want *ErrorResponse, got %T", res)
	// The upstream's only 404 body: a message with no usable detail.
	assert.Equal(t, herp.NewOptString("Function"), rsp.Error.Value.Message)
}

func TestGetCompanySparseFields(t *testing.T) {
	srv := herp.NewMockServer()
	defer srv.Close()

	client, err := herp.NewClient(srv.URL)
	require.NoError(t, err)

	res, err := client.GetCompany(t.Context(), herp.GetCompanyParams{Slug: herp.MockSparseSlug})
	require.NoError(t, err)

	rsp, ok := res.(*herp.CompanyResponse)
	require.True(t, ok, "want *CompanyResponse, got %T", res)

	var job herp.Job
	for _, j := range rsp.Company.Jobs {
		if j.ID == "zCbE82jVP-8s" {
			job = j
			break
		}
	}
	require.Equal(t, "zCbE82jVP-8s", job.ID, "sparse posting missing from fixture")

	// A company can publish a posting with no location of any kind and an
	// unparsed salary, so nothing below the free text is guaranteed.
	assert.Empty(t, job.Location)
	assert.Empty(t, job.JobLocations)
	assert.Equal(t, herp.OptNilJobJobRemoteworkType{Set: true, Null: true}, job.JobRemoteworkType)
	require.True(t, job.Salary.Set)
	assert.NotEmpty(t, job.Salary.Value.Text)
	assert.Equal(t, herp.OptNilInt{Set: true, Null: true}, job.Salary.Value.Minimum)
	assert.Equal(t, herp.OptNilSalaryPeriod{Set: true, Null: true}, job.Salary.Value.Period)
}

func TestListJobs(t *testing.T) {
	srv := herp.NewMockServer()
	defer srv.Close()

	client, err := herp.NewClient(srv.URL)
	require.NoError(t, err)

	rsp, err := client.ListJobs(t.Context(), herp.ListJobsParams{
		Page:  herp.NewOptInt(1),
		Limit: herp.NewOptInt(2),
	})
	require.NoError(t, err)

	// total counts companies, not postings, and each row carries only a
	// preview of that company's board.
	assert.Equal(t, 1, rsp.Page)
	assert.Greater(t, rsp.Total, 2000)
	require.Len(t, rsp.Companies, 2)
	for _, c := range rsp.Companies {
		assert.NotEmpty(t, c.CompanySlug)
		assert.NotEmpty(t, c.CompanyName)
		assert.LessOrEqual(t, len(c.Jobs), 2)
		// The profile histories are exclusive to getCompany.
		assert.Empty(t, c.Directors)
		assert.Empty(t, c.FundingHistories)
	}
}

func TestCompaniesRoster(t *testing.T) {
	require.NotEmpty(t, herp.Companies)
	for _, c := range herp.Companies {
		assert.NotEmpty(t, c.Name)
		assert.NotEmpty(t, c.Slug)
		assert.Equal(t, c, herp.CompaniesBySlug[c.Slug])
	}
	assert.Equal(
		t,
		"https://herp.careers/careers/companies/notainc",
		herp.Company{Slug: "notainc"}.CareersURL(),
	)
}
