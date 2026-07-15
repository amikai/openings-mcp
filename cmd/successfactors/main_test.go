package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSearchFilters(t *testing.T) {
	tests := []struct {
		name    string
		flags   searchFlags
		want    map[string]string
		wantErr string
	}{
		{
			name:  "dynamic tenant dimensions",
			flags: searchFlags{filters: []string{"customfield1=Professional", "businessUnit=Cloud"}},
			want:  map[string]string{"customfield1": "Professional", "businessUnit": "Cloud"},
		},
		{
			name:  "legacy flags remain supported",
			flags: searchFlags{department: "Engineering", careerStatus: "Professional", country: "DE"},
			want:  map[string]string{"department": "Engineering", "customfield3": "Professional", "country": "DE"},
		},
		{
			name:    "malformed",
			flags:   searchFlags{filters: []string{"department"}},
			wantErr: "must be name=value",
		},
		{
			name:    "duplicate dynamic key",
			flags:   searchFlags{filters: []string{"country=DE", "country=US"}},
			wantErr: "specified more than once",
		},
		{
			name:    "dynamic and legacy conflict",
			flags:   searchFlags{filters: []string{"country=US"}, country: "DE"},
			wantErr: `conflicts with "--country"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildSearchFilters(tt.flags)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
