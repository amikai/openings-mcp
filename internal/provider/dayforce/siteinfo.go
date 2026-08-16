package dayforce

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/PuerkitoBio/goquery"
)

// SiteInfo is a board's identity, resolved from the SSR candidate portal
// page's embedded Next.js state rather than a JSON endpoint — there isn't
// one. It carries JobBoardId, which detail and posting-attribute calls need
// but which never appears in the page's own URL.
type SiteInfo struct {
	ClientNamespace string `json:"clientNamespace"`
	JobBoardCode    string `json:"jobBoardCode"`
	CultureCode     string `json:"cultureCode"`
	JobBoardId      int    `json:"jobBoardId"`
}

// nextData is the shape of the candidate portal page's __NEXT_DATA__ script
// tag: Next.js/react-query's own dehydrated state, not a purpose-built API
// response. Only the fields this package reads are declared. Queries are a
// list, not a map, so the site-info entry is found by scanning queryKey.
type nextData struct {
	Props struct {
		PageProps struct {
			DehydratedState struct {
				Queries []struct {
					QueryKey []json.RawMessage `json:"queryKey"`
					State    struct {
						Data json.RawMessage `json:"data"`
					} `json:"state"`
				} `json:"queries"`
			} `json:"dehydratedState"`
		} `json:"pageProps"`
	} `json:"props"`
}

// SiteInfo fetches the SSR candidate portal page for ns/xref and resolves
// its board identity, chiefly JobBoardId for non-roster boards whose
// jobBoardId isn't already known from a search row. culture must be a
// culture the board actually serves (an unsupported one 404s the page
// itself, rather than the silent-empty-search trap).
func (c *BoardClient) SiteInfo(ctx context.Context, ns, xref, culture string) (*SiteInfo, error) {
	url := fmt.Sprintf("%s/%s/%s/%s", c.baseURL, culture, ns, xref)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build dayforce site-info request: %w", err)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch dayforce site-info page: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch dayforce site-info page: unexpected status %d", resp.StatusCode)
	}

	return parseSiteInfo(resp.Body)
}

func parseSiteInfo(r io.Reader) (*SiteInfo, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, fmt.Errorf("parse dayforce site-info page: %w", err)
	}

	sel := doc.Find("script#__NEXT_DATA__")
	if sel.Length() == 0 {
		return nil, errors.New("dayforce site-info: __NEXT_DATA__ script not found")
	}

	var data nextData
	if err := json.Unmarshal([]byte(sel.Text()), &data); err != nil {
		return nil, fmt.Errorf("parse dayforce __NEXT_DATA__: %w", err)
	}

	for _, query := range data.Props.PageProps.DehydratedState.Queries {
		if len(query.QueryKey) == 0 {
			continue
		}
		var key string
		if err := json.Unmarshal(query.QueryKey[0], &key); err != nil || key != "site-info" {
			continue
		}
		var info SiteInfo
		if err := json.Unmarshal(query.State.Data, &info); err != nil {
			return nil, fmt.Errorf("parse dayforce site-info query data: %w", err)
		}
		if info.JobBoardId <= 0 {
			return nil, errors.New("dayforce site-info: missing jobBoardId")
		}
		return &info, nil
	}
	return nil, errors.New("dayforce site-info: no site-info query in __NEXT_DATA__")
}
