package weworkremotely

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// Category pairs a category's display name (as it appears in every item's
// <category> element) with the URL slug of its dedicated RSS feed.
type Category struct {
	Name string
	Slug string
}

// Categories are the fixed set of feeds We Work Remotely publishes under
// /categories/<slug>.rss. There is no endpoint that enumerates them; this
// list was read off the site's own category navigation (2026-07-19).
var Categories = []Category{
	{Name: "Back-End Programming", Slug: "remote-back-end-programming-jobs"},
	{Name: "Customer Support", Slug: "remote-customer-support-jobs"},
	{Name: "Design", Slug: "remote-design-jobs"},
	{Name: "DevOps and Sysadmin", Slug: "remote-devops-sysadmin-jobs"},
	{Name: "Front-End Programming", Slug: "remote-front-end-programming-jobs"},
	{Name: "Full-Stack Programming", Slug: "remote-full-stack-programming-jobs"},
	{Name: "Management and Finance", Slug: "remote-management-and-finance-jobs"},
	{Name: "Product", Slug: "remote-product-jobs"},
	{Name: "Sales and Marketing", Slug: "remote-sales-and-marketing-jobs"},
	// Note the missing "remote-" prefix — the only category slug that
	// doesn't follow the remote-<name>-jobs pattern.
	{Name: "All Other Remote", Slug: "all-other-remote-jobs"},
}

// Job is one posting, flattened from an RSS <item>.
type Job struct {
	ID          string // URL slug, e.g. "lawnstarter-data-governance-platform-manager"; pass to [Client.Detail]
	Company     string
	Title       string
	Category    string // display name, matches a [Categories] entry's Name
	Region      string // e.g. "Anywhere in the World", or a US state/country name; often the only location hint
	Country     string // sometimes an emoji-flag-prefixed country name; often empty
	State       string // sometimes a US state or foreign province/region; often empty
	Skills      string // free-text, comma-joined tags; often empty
	Type        string // observed values: "Full-Time", "Contract"
	Description string // full posting body, HTML
	PostedAt    time.Time
	ExpiresAt   time.Time
	URL         string
	LogoURL     string
}

// Client fetches We Work Remotely's public RSS feeds.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient builds a Client. When httpClient is nil, http.DefaultClient is
// used. The feeds need no cookies, auth, or session warm-up — just a
// non-empty User-Agent (an empty one is enough to get a Cloudflare
// challenge on some request shapes).
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{httpClient: httpClient, baseURL: strings.TrimSuffix(baseURL, "/")}
}

// Jobs fetches one category's full, unfiltered feed.
func (c *Client) Jobs(ctx context.Context, category Category) ([]Job, error) {
	jobs, err := c.fetch(ctx, c.baseURL+"/categories/"+category.Slug+".rss")
	if err != nil {
		return nil, fmt.Errorf("fetch %s feed: %w", category.Name, err)
	}
	return jobs, nil
}

// AllJobs fetches every category feed and merges them, deduplicating by
// [Job.ID]. Categories are independent feeds fetched one at a time (10 HTTP
// requests, not 1); one category's fetch failing does not abort the rest —
// AllJobs returns the jobs it did get plus every failure joined into the
// returned error, so a transient hiccup on one feed (e.g. the Cloudflare
// challenge noted in the package doc) degrades results instead of failing
// outright. A nil error means every category succeeded; check len(jobs)
// rather than err == nil to tell partial success from total failure.
func (c *Client) AllJobs(ctx context.Context) ([]Job, error) {
	var all []Job
	var errs []error
	for _, cat := range Categories {
		jobs, err := c.Jobs(ctx, cat)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		all = append(all, jobs...)
	}
	return dedupeByID(all), errors.Join(errs...)
}

// Search fetches the relevant feed(s) for opts and filters them with
// [FilterJobs]. A recognized opts.Category fetches only that one feed
// instead of the full 10-feed dump; either path deduplicates by [Job.ID]
// first (see [Client.Jobs] — a raw feed can list the same job twice), so
// Search never returns a duplicate regardless of which path ran. When
// opts.Category isn't recognized and some (not all) category feeds fail,
// Search still returns the filtered results from whichever feeds
// succeeded, alongside the joined error — check len(result) to
// distinguish a partial result from having nothing to filter.
func (c *Client) Search(ctx context.Context, opts FilterOptions) ([]Job, error) {
	if cat, ok := lookupCategory(opts.Category); ok {
		jobs, err := c.Jobs(ctx, cat)
		if err != nil {
			return nil, err
		}
		narrowed := opts
		narrowed.Category = "" // the fetch already narrowed it; avoid re-filtering
		return FilterJobs(dedupeByID(jobs), narrowed), nil
	}
	jobs, err := c.AllJobs(ctx)
	if len(jobs) == 0 && err != nil {
		return nil, err
	}
	return FilterJobs(jobs, opts), err
}

// Detail resolves one job by [Job.ID] from a fresh [Client.AllJobs] fetch.
// There is no per-job endpoint in use here — see the package doc for why.
// A job found in a feed that did succeed is returned even if other feeds
// failed; an ID missing from every feed that did succeed is an error,
// which mentions any concurrent feed failures since they may be why it
// wasn't found.
func (c *Client) Detail(ctx context.Context, id string) (*Job, error) {
	jobs, err := c.AllJobs(ctx)
	for i := range jobs {
		if jobs[i].ID == id {
			return &jobs[i], nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("job %q not found, and some category feeds failed to fetch (it may be in one of them): %w", id, err)
	}
	return nil, fmt.Errorf("job %q not found in the current feeds; it may have expired or rotated out", id)
}

func lookupCategory(name string) (Category, bool) {
	for _, c := range Categories {
		if strings.EqualFold(c.Name, name) {
			return c, true
		}
	}
	return Category{}, false
}

// dedupeByID drops later jobs sharing an earlier one's ID, preserving
// order. A single feed can list the same job twice under an identical
// link/guid (see the package doc), so every caller-facing path applies
// this rather than trusting a feed to be duplicate-free.
func dedupeByID(jobs []Job) []Job {
	seen := make(map[string]bool, len(jobs))
	out := make([]Job, 0, len(jobs))
	for _, j := range jobs {
		if seen[j.ID] {
			continue
		}
		seen[j.ID] = true
		out = append(out, j)
	}
	return out
}

func (c *Client) fetch(ctx context.Context, rawURL string) ([]Job, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/rss+xml, application/xml;q=0.9, */*;q=0.8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var feed rssFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("parse rss: %w", err)
	}

	jobs := make([]Job, 0, len(feed.Channel.Items))
	for _, it := range feed.Channel.Items {
		jobs = append(jobs, it.toJob())
	}
	return jobs, nil
}

// rssFeed mirrors the subset of the RSS 2.0 + media namespace structure
// WWR's feeds actually populate.
type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string   `xml:"title"`
	Region      string   `xml:"region"`
	Country     string   `xml:"country"`
	State       string   `xml:"state"`
	Skills      string   `xml:"skills"`
	Category    string   `xml:"category"`
	Type        string   `xml:"type"`
	Description string   `xml:"description"`
	PubDate     string   `xml:"pubDate"`
	ExpiresAt   string   `xml:"expires_at"`
	GUID        string   `xml:"guid"`
	Link        string   `xml:"link"`
	Media       rssMedia `xml:"http://search.yahoo.com/mrss content"`
}

type rssMedia struct {
	URL string `xml:"url,attr"`
}

func (it rssItem) toJob() Job {
	company, title := splitTitle(it.Title)
	return Job{
		ID:          jobID(it.Link, it.GUID),
		Company:     company,
		Title:       title,
		Category:    it.Category,
		Region:      it.Region,
		Country:     it.Country,
		State:       it.State,
		Skills:      it.Skills,
		Type:        it.Type,
		Description: it.Description,
		PostedAt:    parseRSSTime(it.PubDate),
		ExpiresAt:   parseRSSTime(it.ExpiresAt),
		URL:         it.Link,
		LogoURL:     it.Media.URL,
	}
}

// splitTitle divides a raw "Company: Position" title on the first ": ".
// Some positions themselves contain a colon (e.g. "Education Sub Saharan
// Africa: Consultancy: Design and Implementation of ACSL Impact Studies"),
// so only the first separator counts.
func splitTitle(raw string) (company, title string) {
	if before, after, ok := strings.Cut(raw, ": "); ok {
		return before, after
	}
	return "", raw
}

// jobID takes the trailing URL path segment from link (falling back to
// guid, which is normally identical) as the opaque, stable job identifier.
func jobID(link, guid string) string {
	raw := link
	if raw == "" {
		raw = guid
	}
	if u, err := url.Parse(raw); err == nil && u.Path != "" {
		return path.Base(u.Path)
	}
	return raw
}

// parseRSSTime parses the feeds' fixed "Mon, 02 Jan 2006 15:04:05 -0700"
// timestamp format. An unparseable or empty value yields the zero time
// rather than an error — expires_at in particular is not guaranteed to be
// present on every item type.
func parseRSSTime(s string) time.Time {
	t, err := time.Parse(time.RFC1123Z, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
