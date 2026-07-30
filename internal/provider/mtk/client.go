package mtk

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// DefaultBaseURL is the public website base URL for MediaTek Careers.
const DefaultBaseURL = "https://careers.mediatek.com"

const (
	defaultLimit = 6
	maxLimit     = 100
	searchPath   = "/api/trpc/job.getJobs"
)

var jobIDRE = regexp.MustCompile(`^MTK[0-9]+$`)

// Client accesses MediaTek's public careers site.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// SearchRequest contains the site's server-side search fields. Filter values
// are the opaque codes accepted by MediaTek; the debug CLI maps labels to
// codes using the captured option tables.
type SearchRequest struct {
	Keyword         string
	Categories      []string
	WorkExperiences []string
	Locations       []string
	Programs        []string
	Page            int
	Limit           int
}

type SearchResult struct {
	Status     string     `json:"status"`
	Message    string     `json:"message,omitempty"`
	Jobs       []Job      `json:"jobs"`
	Pagination Pagination `json:"pagination"`
}

type Pagination struct {
	CurrentPage int `json:"current_page"`
	TotalPages  int `json:"total_pages"`
	TotalItems  int `json:"total_items"`
}

type Job struct {
	ID                 string      `json:"id"`
	Title              string      `json:"title"`
	Summary            string      `json:"summary,omitempty"`
	Description        string      `json:"description,omitempty"`
	JobPostStatus      string      `json:"job_post_status,omitempty"`
	PublishedDate      string      `json:"published_date,omitempty"`
	Category           string      `json:"category,omitempty"`
	CategoryCode       string      `json:"category_code,omitempty"`
	WorkExperience     string      `json:"work_experience,omitempty"`
	WorkExperienceCode string      `json:"work_experience_code,omitempty"`
	Location           string      `json:"location,omitempty"`
	LocationCode       string      `json:"location_code,omitempty"`
	Program            string      `json:"program,omitempty"`
	ProgramCode        string      `json:"program_code,omitempty"`
	Education          []Education `json:"education,omitempty"`
}

type Education struct {
	Degree string `json:"degree"`
	Major  string `json:"major"`
}

type JobDetail struct {
	ID             string `json:"id"`
	URL            string `json:"url"`
	Title          string `json:"title"`
	Category       string `json:"category,omitempty"`
	Location       string `json:"location,omitempty"`
	Experience     string `json:"experience,omitempty"`
	Education      string `json:"education,omitempty"`
	Description    string `json:"description,omitempty"`
	Qualifications string `json:"qualifications,omitempty"`
}

// NewClient uses [http.DefaultClient] when httpClient is nil.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	return &Client{httpClient: cmp.Or(httpClient, http.DefaultClient), baseURL: strings.TrimRight(baseURL, "/")}
}

// Search returns one server-side paginated page of MediaTek postings.
func (c *Client) Search(ctx context.Context, req SearchRequest) (*SearchResult, error) {
	if req.Page < 0 {
		return nil, fmt.Errorf("page must be >= 1, got %d", req.Page)
	}
	page := req.Page
	if page == 0 {
		page = 1
	}
	limit := req.Limit
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < 1 || limit > maxLimit {
		return nil, fmt.Errorf("limit must be between 1 and %d, got %d", maxLimit, limit)
	}

	input := searchInput{
		Locale: "en_US",
		Page:   page,
		Query:  queryInfo(req.Keyword),
		Filters: searchFilters{
			Categories:      copyStrings(req.Categories),
			WorkExperiences: copyStrings(req.WorkExperiences),
			Locations:       copyStrings(req.Locations),
			Programs:        copyStrings(req.Programs),
		},
		SortBy: "publishedDate",
		Order:  "DESC",
		Limit:  limit,
	}

	u, err := c.searchURL(input)
	if err != nil {
		return nil, fmt.Errorf("build search URL: %w", err)
	}
	var envelope searchEnvelope
	if err := c.getJSON(ctx, u, &envelope); err != nil {
		return nil, fmt.Errorf("search jobs: %w", err)
	}
	if envelope.Error != nil {
		return nil, fmt.Errorf("search jobs: %s", envelope.Error.Message)
	}
	return &SearchResult{
		Status:     envelope.Result.Data.JSON.Status,
		Message:    envelope.Result.Data.JSON.Message,
		Jobs:       envelope.Result.Data.JSON.Jobs,
		Pagination: envelope.Result.Data.JSON.Pagination,
	}, nil
}

// JobDetail returns the HTML detail page for a stable ID returned by Search.
func (c *Client) JobDetail(ctx context.Context, jobID string) (*JobDetail, error) {
	if !jobIDRE.MatchString(jobID) {
		return nil, fmt.Errorf("invalid job id %q: expected MTK followed by digits", jobID)
	}
	u, err := url.JoinPath(c.baseURL, "en", "jobs", jobID)
	if err != nil {
		return nil, fmt.Errorf("build job detail URL: %w", err)
	}
	doc, err := c.getHTML(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("job detail %q: %w", jobID, err)
	}
	detail, ok := parseDetailHTML(doc, jobID)
	if !ok {
		return nil, fmt.Errorf("job detail %q: unrecognized detail page", jobID)
	}
	detail.URL = JobURL(c.baseURL, jobID)
	return &detail, nil
}

// JobURL returns the public MediaTek posting URL for a job ID.
func JobURL(baseURL, jobID string) string {
	u, err := url.JoinPath(strings.TrimRight(baseURL, "/"), "en", "jobs", jobID)
	if err != nil {
		return ""
	}
	return u
}

type searchInput struct {
	Locale  string        `json:"locales"`
	Page    int           `json:"page"`
	Query   queryInput    `json:"jobQueryInfo"`
	Filters searchFilters `json:"filters"`
	SortBy  string        `json:"sortBy"`
	Order   string        `json:"order"`
	Limit   int           `json:"limit"`
}

type queryInput struct {
	Keywords []string `json:"keywords"`
	Relation string   `json:"relation"`
}

type searchFilters struct {
	Categories      []string `json:"categorys"`
	WorkExperiences []string `json:"workExperiences"`
	Locations       []string `json:"locations"`
	Programs        []string `json:"programs"`
}

type searchEnvelope struct {
	Result struct {
		Data struct {
			JSON SearchResult `json:"json"`
		} `json:"data"`
	} `json:"result"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (j *Job) UnmarshalJSON(data []byte) error {
	type jobWire struct {
		ID            string `json:"id"`
		Title         string `json:"title"`
		Summary       string `json:"summary"`
		Description   string `json:"description"`
		JobPostStatus string `json:"jobPostStatus"`
		PublishedDate string `json:"publishedDate"`
		Properties    struct {
			Category struct {
				Label string `json:"label"`
				Code  string `json:"code"`
			} `json:"category"`
			WorkExperience struct {
				Label string `json:"label"`
				Code  string `json:"code"`
			} `json:"workExperience"`
			Location struct {
				Label string `json:"label"`
				Code  string `json:"code"`
			} `json:"location"`
			Program struct {
				Label string `json:"label"`
				Code  string `json:"code"`
			} `json:"program"`
			Education []struct {
				Degree string `json:"educationDegree"`
				Major  string `json:"educationMajor"`
			} `json:"jobEducationInfos"`
		} `json:"properties"`
	}
	var wire jobWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*j = Job{
		ID:                 wire.ID,
		Title:              wire.Title,
		Summary:            wire.Summary,
		Description:        wire.Description,
		JobPostStatus:      wire.JobPostStatus,
		PublishedDate:      wire.PublishedDate,
		Category:           wire.Properties.Category.Label,
		CategoryCode:       wire.Properties.Category.Code,
		WorkExperience:     wire.Properties.WorkExperience.Code,
		WorkExperienceCode: wire.Properties.WorkExperience.Label,
		Location:           wire.Properties.Location.Code,
		LocationCode:       wire.Properties.Location.Label,
		Program:            wire.Properties.Program.Label,
		ProgramCode:        wire.Properties.Program.Code,
		Education:          make([]Education, 0, len(wire.Properties.Education)),
	}
	for _, education := range wire.Properties.Education {
		j.Education = append(j.Education, Education{Degree: education.Degree, Major: education.Major})
	}
	return nil
}

func (c *Client) searchURL(input searchInput) (string, error) {
	b, err := json.Marshal(struct {
		JSON searchInput `json:"json"`
	}{JSON: input})
	if err != nil {
		return "", err
	}
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return "", err
	}
	u.Path = strings.TrimRight(u.Path, "/") + searchPath
	q := u.Query()
	q.Set("input", string(b))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (c *Client) getJSON(ctx context.Context, rawURL string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	setHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	return nil
}

func (c *Client) getHTML(ctx context.Context, rawURL string) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	setHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}
	return doc, nil
}

func setHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/html, application/xhtml+xml;q=0.9, */*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	// The Next.js locale middleware redirects a cookie-less request back to
	// the same path. The browser normally sets this cookie on the first page
	// load; the provider is intentionally stateless, so send the stable public
	// English locale explicitly.
	req.Header.Set("Cookie", "NEXT_LOCALE=en")
}

func queryInfo(keyword string) queryInput {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return queryInput{Keywords: []string{}, Relation: "AND"}
	}
	lower := strings.ToLower(keyword)
	andAt := strings.Index(lower, " and ")
	orAt := strings.Index(lower, " or ")
	relation := "AND"
	if andAt < 0 || orAt >= 0 && orAt < andAt {
		if orAt >= 0 {
			relation = "OR"
		}
	}
	parts := splitQueryWords(keyword)
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return queryInput{Keywords: parts, Relation: relation}
}

func splitQueryWords(value string) []string {
	parts := strings.Fields(value)
	var out []string
	var current []string
	for _, part := range parts {
		if strings.EqualFold(part, "and") || strings.EqualFold(part, "or") {
			if len(current) > 0 {
				out = append(out, strings.Join(current, " "))
				current = nil
			}
			continue
		}
		current = append(current, part)
	}
	if len(current) > 0 {
		out = append(out, strings.Join(current, " "))
	}
	return out
}

func copyStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string{}, values...)
}
