package asus

import (
	"context"
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
		Categories: []Category{CategoryRD},
		Location:   "TW",
	})
	if err != nil {
		t.Fatalf("unexpected search error: %v", err)
	}
	if len(resp.Jobs) == 0 {
		t.Fatal("expected filtered jobs")
	}
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
