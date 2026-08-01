package workable

import (
	"errors"
	"fmt"
	"strings"

	"github.com/urfave/cli/v3"

	provider "github.com/amikai/openings-mcp/internal/provider/workable"
)

// rejectArgs mirrors the ff/v4 CLI's refusal to silently ignore positional
// arguments: urfave/cli v3 accepts and discards them.
func rejectArgs(cmd *cli.Command) error {
	if cmd.NArg() == 0 {
		return nil
	}
	return fmt.Errorf("%s takes no positional arguments, got %v", cmd.Name, cmd.Args().Slice())
}

// normalizeCompany requires --company to be a curated company — same policy
// as cmd/smartrecruiters — and returns the roster's account subdomain.
func normalizeCompany(company string) (string, error) {
	if company == "" {
		return "", errors.New("--company is required")
	}
	c, ok := provider.CompaniesByAccount[strings.ToLower(company)]
	if !ok {
		return "", fmt.Errorf("company %q not found; run 'openings-cli workable companies' to see supported companies", company)
	}
	return c.Account, nil
}

// newClient builds the generated client against --base-url, which defaults
// to the live API and is overridden by tests to point at
// provider.NewMockServer().
func newClient(cmd *cli.Command) (*provider.Client, error) {
	return provider.NewClient(cmd.String("base-url"))
}
