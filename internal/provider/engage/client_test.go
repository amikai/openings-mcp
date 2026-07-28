package engage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoard(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.Board(t.Context(), MockSlug)
	require.NoError(t, err)
	assert.Equal(t, MockSlug, got.Slug)
	require.Len(t, got.Categories, 2)

	first := got.Categories[0]
	assert.Equal(t, "中途採用", first.Name)
	assert.True(t, first.AtCap)
	assert.Len(t, first.Jobs, CategoryCap)

	second := got.Categories[1]
	assert.Equal(t, "アルバイト・パート採用", second.Name)
	assert.False(t, second.AtCap)
	assert.Len(t, second.Jobs, 7)

	job := first.Jobs[0]
	assert.Equal(t, "17046487", job.WorkID)
	assert.NotEmpty(t, job.Title)
	assert.NotEmpty(t, job.Salary)
	assert.NotEmpty(t, job.Area)
	assert.Equal(t, "中途採用", job.EmploymentType)
	assert.False(t, job.LastUpdated.IsZero())

	assert.True(t, got.AnyAtCap())
	assert.Len(t, got.Jobs(), 107)
}

func TestBoardAtCapNotAboveCap(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.Board(t.Context(), MockCapSlug)
	require.NoError(t, err)
	require.Len(t, got.Categories, 1)
	assert.True(t, got.Categories[0].AtCap)
	assert.Len(t, got.Categories[0].Jobs, CategoryCap)
	assert.True(t, got.AnyAtCap())
}

func TestBoardMinimal(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.Board(t.Context(), MockMinimalSlug)
	require.NoError(t, err)
	require.Len(t, got.Categories, 1)
	assert.False(t, got.Categories[0].AtCap)
	assert.Len(t, got.Jobs(), 1)
	assert.False(t, got.AnyAtCap())
}

func TestBoardNotFound(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	_, err := c.Board(t.Context(), MockUnknownSlug)
	require.ErrorIs(t, err, ErrBoardNotFound)
}

func TestJob(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	got, err := c.Job(t.Context(), MockSlug, MockWorkID)
	require.NoError(t, err)
	assert.Equal(t, MockWorkID, got.WorkID)
	assert.NotEmpty(t, got.Title)
	assert.Contains(t, got.DescriptionHTML, "NOVA")
	assert.Equal(t, "FULL_TIME", got.EmploymentType)
	assert.False(t, got.DatePosted.IsZero())
	assert.NotEmpty(t, got.Organization.Name)
	require.Len(t, got.Salaries, 1)
	assert.Equal(t, "JPY", got.Salaries[0].Currency)
	assert.Equal(t, "YEAR", got.Salaries[0].UnitText)
	assert.Equal(t, "3600000", got.Salaries[0].MinValue)
	assert.Empty(t, got.Salaries[0].MaxValue)
	assert.NotEmpty(t, got.Location.Region)
	assert.True(t, got.DirectApply)
	assert.NotEmpty(t, got.Sections)

	var sawQualifications bool
	for _, s := range got.Sections {
		if s.Heading == "応募資格・条件" {
			sawQualifications = true
			assert.NotEmpty(t, s.Text)
		}
	}
	assert.True(t, sawQualifications, "expected a 応募資格・条件 section")
}

func TestJobNotFound(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	_, err := c.Job(t.Context(), MockSlug, MockUnknownWorkID)
	require.ErrorIs(t, err, ErrJobNotFound)
}

func TestJobEmptyWorkID(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	_, err := c.Job(t.Context(), MockSlug, "")
	require.Error(t, err)
}

func TestCompanies(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	page1, err := c.Companies(t.Context(), 1, 0)
	require.NoError(t, err)
	assert.Equal(t, 1031581, page1.TotalCount)
	assert.True(t, page1.HasMore)
	require.NotEmpty(t, page1.Records)
	rec := page1.Records[0]
	assert.Equal(t, MockSlug, rec.Slug)
	assert.NotEmpty(t, rec.Name)
	assert.NotEmpty(t, rec.OfficialName)
	assert.NotZero(t, rec.CompanyID)
	assert.NotEmpty(t, rec.WorkID)

	// The p_t/f_t stitch: page 2 requires the previous page's TotalCount, not
	// a caller-chosen offset (see API.md).
	page2, err := c.Companies(t.Context(), 2, page1.TotalCount)
	require.NoError(t, err)
	assert.NotEmpty(t, page2.Records)
}

func TestCompaniesPage2WrongPrevTotal(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client())

	_, err := c.Companies(t.Context(), 2, 0)
	require.Error(t, err)
}
