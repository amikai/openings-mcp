package asus

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

func TestClient_Search(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client := NewClient(srv.URL, srv.Client())
	resp, err := client.Search(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected search error: %v", err)
	}
	if len(resp.Jobs) == 0 {
		t.Fatal("expected at least 1 job")
	}
	if resp.TotalPages <= 0 {
		t.Errorf("expected total pages > 0, got %d", resp.TotalPages)
	}
}

func TestClient_SearchFiltered(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client := NewClient(srv.URL, srv.Client())
	resp, err := client.Search(context.Background(), &SearchRequest{
		Keyword:    "AI",
		Categories: []string{"研究發展"},
		Location:   "TW",
	})
	if err != nil {
		t.Fatalf("unexpected search error: %v", err)
	}
	if len(resp.Jobs) == 0 {
		t.Fatal("expected filtered jobs")
	}
}

// TestClient_SearchFilterOptions checks that a search carries back the filter
// options of whichever locale the session is in, rather than a set this
// package froze in one language.
func TestClient_SearchFilterOptions(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	resp, err := NewClient(srv.URL, srv.Client()).Search(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected search error: %v", err)
	}
	if got, want := len(resp.Categories), 4; got != want {
		t.Fatalf("got %d categories, want %d", got, want)
	}
	if got, want := resp.Categories[0].Value, "研究發展"; got != want {
		t.Errorf("first category = %q, want %q", got, want)
	}
	if len(resp.Countries) == 0 || len(resp.Experiences) == 0 {
		t.Errorf("got %d countries and %d experience levels, want both non-empty",
			len(resp.Countries), len(resp.Experiences))
	}

	enClient := &http.Client{Transport: cultureCookie("en-US")}
	enResp, err := NewClient(srv.URL, enClient).Search(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected en-US search error: %v", err)
	}
	if got, want := enResp.Categories[0].Value, "Research and Development"; got != want {
		t.Errorf("first en-US category = %q, want %q", got, want)
	}
}

// cultureCookie is a transport that carries the culture cookie
// /Home/SetLanguage?culture=<culture> would set on a real session.
type cultureCookie string

func (c cultureCookie) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Cookie", "hrisweb.test=c="+string(c)+"|uic="+string(c))
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("round trip %s: %w", req.URL, err)
	}
	return resp, nil
}

func TestClient_SearchEmpty(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client := NewClient(srv.URL, srv.Client())
	resp, err := client.Search(context.Background(), &SearchRequest{
		Keyword: "xyz999foobar",
	})
	if err != nil {
		t.Fatalf("unexpected search error: %v", err)
	}
	if len(resp.Jobs) != 0 {
		t.Fatalf("expected 0 jobs, got %d", len(resp.Jobs))
	}
}

func TestClient_SearchInvalidPage(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client := NewClient(srv.URL, srv.Client())
	_, err := client.Search(context.Background(), &SearchRequest{
		Page: -1,
	})
	if err == nil {
		t.Fatal("expected error for page < 0, got nil")
	}
}

func TestClient_Detail(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client := NewClient(srv.URL, srv.Client())
	detail, err := client.Detail(context.Background(), "762c08de-1daa-4aa8-9668-d8a746ce24a8")
	if err != nil {
		t.Fatalf("unexpected detail error: %v", err)
	}
	if detail.ID != "762c08de-1daa-4aa8-9668-d8a746ce24a8" {
		t.Errorf("expected id %q, got %q", "762c08de-1daa-4aa8-9668-d8a746ce24a8", detail.ID)
	}
}

func TestClient_DetailNotFound(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client := NewClient(srv.URL, srv.Client())
	_, err := client.Detail(context.Background(), "invalid-sn")
	if err == nil {
		t.Fatal("expected error for non-existent job, got nil")
	}
}

func TestClient_GetCities(t *testing.T) {
	srv := NewMockServer()
	defer srv.Close()

	client := NewClient(srv.URL, srv.Client())
	cities, err := client.GetCities(context.Background(), "TW")
	if err != nil {
		t.Fatalf("unexpected getCities error: %v", err)
	}
	if len(cities) == 0 {
		t.Fatal("expected cities for TW, got 0")
	}
	if cities[0].Value != "TPE" {
		t.Errorf("expected first city value 'TPE', got %q", cities[0].Value)
	}
}
