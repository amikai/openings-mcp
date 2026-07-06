package lever

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMockServer wraps the package mock server with per-request assertions
// on the exact query encoding the generated client produces.
func newMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v0/postings/{site}", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "leverdemo", r.PathValue("site"))
		q := r.URL.Query()
		assert.Equal(t, "json", q.Get("mode"))
		assert.Equal(t, "0", q.Get("skip"))
		assert.Equal(t, "3", q.Get("limit"))
		assert.Equal(t, []string{"Arlington, TX", "New York, NY"}, q["location"])
		assert.Equal(t, []string{"Customer Success"}, q["department"])
		serveMockJSON(mockPostingsRsp)(w, r)
	})
	mux.HandleFunc("/v0/postings/{site}/{postingId}", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "leverdemo", r.PathValue("site"))
		assert.Equal(t, "33538a2f-d27d-4a96-8f05-fa4b0e4d940e", r.PathValue("postingId"))
		serveMockJSON(mockPostingDetailRsp)(w, r)
	})
	return httptest.NewServer(mux)
}

var wantPosting = Posting{
	ID:   "33538a2f-d27d-4a96-8f05-fa4b0e4d940e",
	Text: "AbelsonTaylor Writer",
	Categories: NewOptPostingCategories(PostingCategories{
		Location:     NewOptString("Arlington, TX"),
		Commitment:   NewOptString("Regular Full Time (Salary)"),
		Team:         NewOptString("Professional Services"),
		Department:   NewOptString("Customer Success"),
		AllLocations: []string{"Arlington, TX"},
	}),
	Country:       NewOptNilString("US"),
	CreatedAt:     NewOptInt64(1553186035299),
	WorkplaceType: NewOptString("hybrid"),
	Opening:       NewOptString("<div>Welcome to the <b>Demo Job Listing</b> for Lever! This is a fictional job created solely for demonstration purposes and is <b>not an actual open position</b>. We’ve crafted this listing to showcase the functionality of our ATS platform, including job descriptions, application processes, and more.</div><div><br></div><div>While you can explore the application process and features here, please note that <b>applications submitted to this job will not be reviewed or responded to</b> as it’s for demonstration only. </div>"),
	OpeningPlain:  NewOptString("Welcome to the Demo Job Listing for Lever! This is a fictional job created solely for demonstration purposes and is not an actual open position. We’ve crafted this listing to showcase the functionality of our ATS platform, including job descriptions, application processes, and more.\n\nWhile you can explore the application process and features here, please note that applications submitted to this job will not be reviewed or responded to as it’s for demonstration only. \n"),
	Description:   NewOptString("<div>Welcome to the <b>Demo Job Listing</b> for Lever! This is a fictional job created solely for demonstration purposes and is <b>not an actual open position</b>. We’ve crafted this listing to showcase the functionality of our ATS platform, including job descriptions, application processes, and more.</div><div><br></div><div>While you can explore the application process and features here, please note that <b>applications submitted to this job will not be reviewed or responded to</b> as it’s for demonstration only. </div><div><br></div><div>this job is AMAAAAAAAAAAAAZING!</div>"),
	DescriptionPlain: NewOptString("Welcome to the Demo Job Listing for Lever! This is a fictional job created solely for demonstration purposes and is not an actual open position. We’ve crafted this listing to showcase the functionality of our ATS platform, including job descriptions, application processes, and more.\n\nWhile you can explore the application process and features here, please note that applications submitted to this job will not be reviewed or responded to as it’s for demonstration only. \n\n\nthis job is AMAAAAAAAAAAAAZING!\n"),
	DescriptionBody:      NewOptString("<div>this job is AMAAAAAAAAAAAAZING!</div>"),
	DescriptionBodyPlain: NewOptString("this job is AMAAAAAAAAAAAAZING!\n"),
	Lists: []PostingListEntry{
		{
			Text:    "Qualifications",
			Content: "<li>be smart</li><li>be very smart</li>",
		},
		{
			Text:    "Duties",
			Content: "<li>work hard</li><li>work VERY hard</li><li><b>bold text</b></li><li><i>italic text</i></li><li><s>strikethrough text</s></li><li><u>underline text</u></li><li><a rel=\"noopener noreferrer\" class=\"postings-link\" href=\"https://google.com\">link text</a></li>",
		},
	},
	Additional:      NewOptString("<div>you will never find a job better than this one!!!</div><div><br></div><div>Lever builds modern recruiting software for teams to source, interview, and hire top talent. Our team strives to set a new bar for enterprise software with modern, well-designed, real-time apps. We participated in Y Combinator in summer 2012, and since then have raised $73 million. As the applicant tracking system of choice for Netflix, Eventbrite, ClearSlide, change.org, and thousands more leading companies, Lever means you hire the best by hiring together.</div><div><br></div><div>Lever is an equal opportunity employer. We are committed to providing reasonable accommodations and will work with you to meet your needs. If you are a person with a disability and require assistance during the application process, please don’t hesitate to reach out! We celebrate our inclusive work environment and welcome members of all backgrounds and perspectives.&nbsp;<a class=\"postings-link\" href=\"https://inside.lever.co/\">Learn more about our team culture and commitment to diversity and inclusion.</a>&nbsp;</div>"),
	AdditionalPlain: NewOptString("you will never find a job better than this one!!!\n\n\nLever builds modern recruiting software for teams to source, interview, and hire top talent. Our team strives to set a new bar for enterprise software with modern, well-designed, real-time apps. We participated in Y Combinator in summer 2012, and since then have raised $73 million. As the applicant tracking system of choice for Netflix, Eventbrite, ClearSlide, change.org, and thousands more leading companies, Lever means you hire the best by hiring together.\n\nLever is an equal opportunity employer. We are committed to providing reasonable accommodations and will work with you to meet your needs. If you are a person with a disability and require assistance during the application process, please don’t hesitate to reach out! We celebrate our inclusive work environment and welcome members of all backgrounds and perspectives. Learn more about our team culture and commitment to diversity and inclusion. \n"),
	HostedUrl:       NewOptString("https://jobs.lever.co/leverdemo/33538a2f-d27d-4a96-8f05-fa4b0e4d940e"),
	ApplyUrl:        NewOptString("https://jobs.lever.co/leverdemo/33538a2f-d27d-4a96-8f05-fa4b0e4d940e/apply"),
}

func TestListPostings(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	c, err := NewClient(srv.URL, WithClient(srv.Client()))
	require.NoError(t, err)

	got, err := c.ListPostings(t.Context(), ListPostingsParams{
		Site:       "leverdemo",
		Mode:       ListPostingsModeJSON,
		Skip:       NewOptInt(0),
		Limit:      NewOptInt(3),
		Location:   []string{"Arlington, TX", "New York, NY"},
		Department: []string{"Customer Success"},
	})
	require.NoError(t, err)
	require.Len(t, got, 3)

	assert.Equal(t, wantPosting, got[0])
}

func TestGetPosting(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	c, err := NewClient(srv.URL, WithClient(srv.Client()))
	require.NoError(t, err)

	got, err := c.GetPosting(t.Context(), GetPostingParams{
		Site:      "leverdemo",
		PostingId: "33538a2f-d27d-4a96-8f05-fa4b0e4d940e",
	})
	require.NoError(t, err)
	assert.Equal(t, &wantPosting, got)
}

func TestListPostingsNotFound(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	c, err := NewClient(srv.URL, WithClient(srv.Client()))
	require.NoError(t, err)

	_, err = c.ListPostings(t.Context(), ListPostingsParams{
		Site: MockNotFoundSite,
		Mode: ListPostingsModeJSON,
	})
	require.Error(t, err)

	ue, ok := errors.AsType[*ErrorResponseStatusCode](err)
	require.True(t, ok, "expected *ErrorResponseStatusCode in %v", err)
	want := &ErrorResponseStatusCode{
		StatusCode: 404,
		Response:   ErrorResponse{Ok: false, Error: "Document not found"},
	}
	assert.Equal(t, want, ue)
}

func TestGetPostingNotFound(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	c, err := NewClient(srv.URL, WithClient(srv.Client()))
	require.NoError(t, err)

	_, err = c.GetPosting(t.Context(), GetPostingParams{
		Site:      "leverdemo",
		PostingId: MockNotFoundPostingID,
	})
	require.Error(t, err)

	ue, ok := errors.AsType[*ErrorResponseStatusCode](err)
	require.True(t, ok, "expected *ErrorResponseStatusCode in %v", err)
	want := &ErrorResponseStatusCode{
		StatusCode: 404,
		Response:   ErrorResponse{Ok: false, Error: "Document not found"},
	}
	assert.Equal(t, want, ue)
}
