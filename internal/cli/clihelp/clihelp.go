// Package clihelp holds the flag helpers shared by every debug CLI under
// internal/cli, so a flag that means the same thing in 40 commands is
// declared — and validated — the same way in all of them.
package clihelp

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/pflag"
)

// choice is a pflag.Value that accepts only one of a fixed set of strings.
// Rejecting at parse time is what ff's StringEnumLong did; deferring the
// check to the RunE body lets a typo through as a silently-ignored value.
type choice struct {
	target *string
	valid  []string
}

func (c *choice) String() string { return *c.target }

func (c *choice) Type() string { return "string" }

func (c *choice) Set(v string) error {
	if !slices.Contains(c.valid, v) {
		return fmt.Errorf("must be one of: %s", strings.Join(c.quotedValid(), ", "))
	}
	*c.target = v
	return nil
}

// quotedValid renders the valid set for an error message, spelling the empty
// value as "" rather than as a gap the reader has to guess at.
func (c *choice) quotedValid() []string {
	out := make([]string, 0, len(c.valid))
	for _, v := range c.valid {
		if v == "" {
			out = append(out, `""`)
			continue
		}
		out = append(out, v)
	}
	return out
}

// ChoiceVar defines a flag that accepts only one of valid, defaulting to def.
// A value outside valid fails the parse, so RunE never sees one.
func ChoiceVar(fs *pflag.FlagSet, target *string, name, def string, valid []string, usage string) {
	*target = def
	fs.Var(&choice{target: target, valid: valid}, name, UsageWithChoices(usage, valid))
}

// FormatVar defines the --format flag every debug CLI shares.
func FormatVar(fs *pflag.FlagSet, target *string) {
	ChoiceVar(fs, target, "format", "text", []string{"text", "json"}, "output format")
}

// UsageWithChoices appends a "one of: ..." list to base, because pflag's help
// output never introspects a Value's accepted set on its own.
func UsageWithChoices(base string, choices []string) string {
	return fmt.Sprintf("%s, one of: %s", base, strings.Join(choices, " | "))
}

// SortedKeys returns table's keys in sorted order — the label set a
// provider's ID lookup table accepts.
func SortedKeys[V any](table map[string]V) []string {
	keys := make([]string, 0, len(table))
	for k := range table {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// WithUnset prefixes labels with "" so a filter flag can default to unset
// (no filter) instead of silently defaulting to the first real label.
func WithUnset(labels []string) []string {
	return append([]string{""}, labels...)
}
