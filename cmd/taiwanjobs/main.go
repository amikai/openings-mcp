// Command taiwanjobs is a debug CLI for the TaiwanJobs (台灣就業通) open feed.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/amikai/openings-mcp/internal/provider/taiwanjobs"
)

const defaultBaseURL = "https://free.taiwanjobs.gov.tw"

func main() {
	rootFlags := ff.NewFlagSet("taiwanjobs")
	var (
		baseURL = rootFlags.StringLong("base-url", defaultBaseURL, "TaiwanJobs base URL")
		timeout = rootFlags.DurationLong("timeout", 60*time.Second, "request timeout")
		format  = rootFlags.StringEnumLong("format", "output format", "text", "json")
	)
	rootCmd := &ff.Command{
		Name:  "taiwanjobs",
		Usage: "taiwanjobs [FLAGS] search [FLAGS]",
		Flags: rootFlags,
	}

	searchFS := ff.NewFlagSet("search").SetParent(rootFlags)
	var (
		keyword = searchFS.StringLong("keyword", "", "client-side substring filter over title/company/body")
		zipno   = searchFS.StringLong("zipno", "", "Taiwan postal code filter, e.g. 110")
		jobno   = searchFS.StringLong("jobno", "", "job category code, 2-digit major or 6-digit minor")
		count   = searchFS.IntLong("count", taiwanjobs.DefaultCount, "rows to fetch before the keyword filter (max 1000)")
	)
	searchCmd := &ff.Command{
		Name:      "search",
		Usage:     "taiwanjobs search [--keyword TEXT] [--zipno CODE] [--jobno CODE] [--count N] [--format text|json]",
		ShortHelp: "fetch the TaiwanJobs open feed; JSON mirrors the parsed feed rows",
		Flags:     searchFS,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("search takes no positional arguments, got %v", args)
			}
			hc := &http.Client{Timeout: *timeout}
			c := taiwanjobs.NewClient(*baseURL, hc)
			resp, err := c.Jobs(ctx, &taiwanjobs.JobsRequest{
				Count:   *count,
				ZipNo:   *zipno,
				JobNo:   *jobno,
				Keyword: *keyword,
			})
			if err != nil {
				return err
			}
			if *format == "json" {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				enc.SetEscapeHTML(false)
				return enc.Encode(resp)
			}
			fmt.Printf("fetched %d rows, %d after keyword filter\n", resp.Fetched, len(resp.Jobs))
			for _, j := range resp.Jobs {
				fmt.Printf("- %s | %s | %s | %s %s-%s | deadline %s | %s\n",
					j.Title, j.Company, j.Location, j.SalaryType, j.SalaryLow, j.SalaryHigh, j.ApplyDeadline, j.URL)
			}
			return nil
		},
	}
	rootCmd.Subcommands = append(rootCmd.Subcommands, searchCmd)

	if err := rootCmd.ParseAndRun(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, ffhelp.Command(rootCmd))
		fmt.Fprintln(os.Stderr, "err:", err)
		os.Exit(1)
	}
}
