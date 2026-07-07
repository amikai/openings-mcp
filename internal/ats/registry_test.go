package ats

import (
	"context"
	"strings"
	"testing"
)

// fakeAdapter satisfies Adapter with a canned roster; search methods are
// never reached by registry tests.
type fakeAdapter struct {
	name   string
	roster []CompanyInfo
}

func (f *fakeAdapter) Name() string          { return f.name }
func (f *fakeAdapter) Roster() []CompanyInfo { return f.roster }
func (f *fakeAdapter) Search(context.Context, string, SearchParams) (*SearchResult, error) {
	return nil, nil
}
func (f *fakeAdapter) Filters(context.Context, string) (FilterSet, error) { return nil, nil }
func (f *fakeAdapter) Detail(context.Context, string, string) (*JobDetail, error) {
	return nil, nil
}

func testRegistry(t *testing.T) *Registry {
	t.Helper()
	r, err := NewRegistry(
		&fakeAdapter{name: "workday", roster: []CompanyInfo{
			{Slug: "nvidia", Name: "NVIDIA Corp"},
			{Slug: "workday", Name: "Workday, Inc."},
		}},
		&fakeAdapter{name: "lever", roster: []CompanyInfo{
			{Slug: "palantir", Name: "Palantir Technologies"},
		}},
	)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	return r
}

func TestResolveBySlug(t *testing.T) {
	r := testRegistry(t)
	a, slug, err := r.Resolve("nvidia")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if a.Name() != "workday" || slug != "nvidia" {
		t.Errorf("got (%s, %s), want (workday, nvidia)", a.Name(), slug)
	}
}

func TestResolveByDisplayName(t *testing.T) {
	r := testRegistry(t)
	// Case, punctuation, and spaces must not matter.
	for _, input := range []string{"NVIDIA Corp", "nvidia corp", "Workday, Inc.", "workday inc"} {
		if _, _, err := r.Resolve(input); err != nil {
			t.Errorf("Resolve(%q): %v", input, err)
		}
	}
}

func TestResolveUnknownTeaches(t *testing.T) {
	r := testRegistry(t)
	_, _, err := r.Resolve("palantir tech")
	if err == nil {
		t.Fatal("want error for unknown company")
	}
	msg := err.Error()
	if !strings.Contains(msg, "palantir") {
		t.Errorf("suggestions should contain %q, got: %s", "palantir", msg)
	}
	if !strings.Contains(msg, "3 companies") {
		t.Errorf("error should state supported count, got: %s", msg)
	}
}

func TestResolveEmpty(t *testing.T) {
	r := testRegistry(t)
	if _, _, err := r.Resolve("  "); err == nil {
		t.Fatal("want error for empty company")
	}
}

func TestNewRegistryRejectsDuplicateSlug(t *testing.T) {
	_, err := NewRegistry(
		&fakeAdapter{name: "workday", roster: []CompanyInfo{{Slug: "acme", Name: "Acme (Workday)"}}},
		&fakeAdapter{name: "lever", roster: []CompanyInfo{{Slug: "acme", Name: "Acme (Lever)"}}},
	)
	if err == nil {
		t.Fatal("want error for duplicate slug across adapters")
	}
}
