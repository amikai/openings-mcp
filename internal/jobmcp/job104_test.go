package jobmcp

import (
	"net/http"
	"testing"

	"github.com/amikai/job-mcp/internal/provider/job104"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
)

func TestRegisterJob104(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "v0"}, nil)

	RegisterJob104(server, job104.NewClient(http.DefaultClient))

	assertTools(t, server, "104_search_jobs", "104_get_job_detail")
}

func TestJob104ToRequest(t *testing.T) {
	in := job104SearchInput{
		Keyword: "golang",
		Area:    "taipei",
		JobType: "part",
		Sort:    "newest",
		Remote:  "full",
		Page:    2,
	}
	got, err := job104ToRequest(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ro := 1
	order := 15
	remoteWork := 2
	page := 2
	want := &job104.JobsRequest{
		Keyword:    "golang",
		Area:       job104.AreaTaipei,
		RO:         &ro,
		Order:      &order,
		RemoteWork: &remoteWork,
		Page:       &page,
	}
	assert.Equal(t, want, got)
}

func TestJob104ToRequestInvalidArea(t *testing.T) {
	_, err := job104ToRequest(job104SearchInput{Keyword: "x", Area: "atlantis"})
	assert.Error(t, err)
}
