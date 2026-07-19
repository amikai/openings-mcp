package weworkremotely

import "testing"

func TestFilterJobs(t *testing.T) {
	jobs := []Job{
		{
			ID: "a", Company: "Acme Corp", Title: "Senior Go Engineer",
			Category: "Back-End Programming", Region: "USA Only", Country: "", State: "Texas",
			Skills: "Go, Kubernetes", Type: "Full-Time", Description: "Build backend services in Go.",
		},
		{
			ID: "b", Company: "Widget Inc", Title: "Product Designer",
			Category: "Design", Region: "Anywhere in the World", Country: "", State: "",
			Skills: "Figma", Type: "Contract", Description: "Design product experiences.",
		},
		{
			ID: "c", Company: "Acme Corp", Title: "Support Specialist",
			Category: "Customer Support", Region: "Anywhere in the World", Country: "🇺🇸 United States of America", State: "",
			Skills: "", Type: "Full-Time", Description: "Help customers with billing questions.",
		},
	}

	tests := []struct {
		name string
		opts FilterOptions
		want []string
	}{
		{"no filter", FilterOptions{}, []string{"a", "b", "c"}},
		{"keyword matches title", FilterOptions{Keyword: "go engineer"}, []string{"a"}},
		{"keyword matches description", FilterOptions{Keyword: "billing"}, []string{"c"}},
		{"keyword matches skills", FilterOptions{Keyword: "figma"}, []string{"b"}},
		{"category exact case-insensitive", FilterOptions{Category: "design"}, []string{"b"}},
		{"category no match", FilterOptions{Category: "Product"}, nil},
		{"company substring", FilterOptions{Company: "acme"}, []string{"a", "c"}},
		{"type exact", FilterOptions{Type: "contract"}, []string{"b"}},
		{"region matches Region field", FilterOptions{Region: "usa"}, []string{"a"}},
		{"region matches State field", FilterOptions{Region: "texas"}, []string{"a"}},
		{"region matches Country field", FilterOptions{Region: "united states"}, []string{"c"}},
		{"combined filters AND together", FilterOptions{Company: "acme", Type: "full-time"}, []string{"a", "c"}},
		{"combined filters no match", FilterOptions{Company: "acme", Type: "contract"}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterJobs(jobs, tt.opts)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d jobs, want %d (%v)", len(got), len(tt.want), tt.want)
			}
			for i, j := range got {
				if j.ID != tt.want[i] {
					t.Errorf("job[%d].ID = %q, want %q", i, j.ID, tt.want[i])
				}
			}
		})
	}
}

func TestFilterJobs_doesNotMutateInput(t *testing.T) {
	jobs := []Job{{ID: "a", Title: "X"}, {ID: "b", Title: "Y"}}
	_ = FilterJobs(jobs, FilterOptions{Keyword: "x"})
	if len(jobs) != 2 || jobs[0].ID != "a" || jobs[1].ID != "b" {
		t.Fatalf("input slice was mutated: %+v", jobs)
	}
}
