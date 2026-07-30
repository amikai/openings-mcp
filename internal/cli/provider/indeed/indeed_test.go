package indeed

import (
	"testing"

	indeedprovider "github.com/amikai/openings-mcp/internal/provider/indeed"
	"github.com/stretchr/testify/assert"
)

func TestFormatCompensation(t *testing.T) {
	tests := []struct {
		name string
		comp *indeedprovider.Compensation
		want string
	}{
		{
			name: "range",
			comp: &indeedprovider.Compensation{MinAmount: 22.5, MaxAmount: 27.5, Currency: "USD", Interval: "HOUR"},
			want: "22.5-27.5 USD (HOUR)",
		},
		{
			name: "at least",
			comp: &indeedprovider.Compensation{MinAmount: 20, Currency: "USD", Interval: "HOUR"},
			want: ">= 20 USD (HOUR)",
		},
		{
			name: "at most",
			comp: &indeedprovider.Compensation{MaxAmount: 30, Currency: "USD", Interval: "HOUR"},
			want: "<= 30 USD (HOUR)",
		},
		{
			name: "exactly",
			comp: &indeedprovider.Compensation{MinAmount: 17.5, MaxAmount: 17.5, Currency: "USD", Interval: "HOUR"},
			want: "17.5 USD (HOUR)",
		},
		{
			name: "undisclosed amounts",
			comp: &indeedprovider.Compensation{Currency: "USD", Interval: "YEAR"},
			want: "undisclosed USD (YEAR)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatCompensation(tt.comp))
		})
	}
}
