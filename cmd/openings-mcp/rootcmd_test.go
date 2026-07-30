package main

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// expectedSubcommands is every subcommand `openings-mcp` must expose. A
// forgotten AddCommand compiles fine and leaves the tool unreachable, which
// is the same failure mode as a provider whose package is complete but never
// wired into the server — so the list is spelled out rather than derived
// from what newRootCmd happens to register.
var expectedSubcommands = []string{
	"104",
	"amazon",
	"apple",
	"ashby",
	"avature",
	"bamboohr",
	"cake",
	"eightfold",
	"engage",
	"flowxtra",
	"foxconn",
	"google",
	"greenhouse",
	"herp",
	"himalayas",
	"hrmos",
	"icims",
	"indeed",
	"jobicy",
	"jobindex",
	"join",
	"lever",
	"linkedin",
	"meta",
	"mokahr",
	"mtk",
	"mynavi",
	"nodesk",
	"nvidia",
	"oracle",
	"quanta",
	"realtek",
	"recruitee",
	"remotefirstjobs",
	"remoteok",
	"remotive",
	"rippling",
	"server",
	"smartrecruiters",
	"successfactors",
	"synopsys",
	"teamtailor",
	"tsmc",
	"ultipro",
	"verify-companies",
	"weworkremotely",
	"workable",
	"workday",
	"workingnomads",
}

func TestRootCmdRegistersEverySubcommand(t *testing.T) {
	registered := map[string]bool{}
	for _, cmd := range newRootCmd().Commands() {
		registered[cmd.Name()] = true
	}

	for _, name := range expectedSubcommands {
		assert.True(t, registered[name], "subcommand %q is not registered on the root command", name)
	}
}

// TestRootCmdSubcommandNamesAreUnique guards the failure cobra hides: two
// commands sharing a Use resolve to whichever was added first, so the second
// is silently dead.
func TestRootCmdSubcommandNamesAreUnique(t *testing.T) {
	seen := map[string]int{}
	for _, cmd := range newRootCmd().Commands() {
		seen[cmd.Name()]++
	}

	for name, count := range seen {
		assert.Equal(t, 1, count, "subcommand %q is registered %d times", name, count)
	}
}

// TestServerFlagsAreNotInheritedByProviderCommands pins the flag scoping: the
// MCP server's flags are local to the two commands that can start it, so the
// provider debug commands neither advertise nor accept them.
func TestServerFlagsAreNotInheritedByProviderCommands(t *testing.T) {
	serverFlags := []string{"log-file", "log-level", "enable-command-logging", "dump-cache-ttl"}

	root := newRootCmd()
	for _, name := range serverFlags {
		assert.NotNil(t, root.Flags().Lookup(name), "root command lost the %q flag", name)
	}

	serverCmd := findSubcommand(t, root, "server")
	for _, name := range serverFlags {
		assert.NotNil(t, serverCmd.Flags().Lookup(name), "server command lost the %q flag", name)
	}

	// workday stands in for the 47 provider debug commands: none of them
	// reads these, so inheriting them would only mislead --help.
	workdayCmd := findSubcommand(t, root, "workday")
	for _, name := range serverFlags {
		assert.Nil(t, workdayCmd.InheritedFlags().Lookup(name),
			"provider command inherits the server-only %q flag", name)
	}
}

func findSubcommand(t *testing.T, parent *cobra.Command, name string) *cobra.Command {
	t.Helper()
	for _, cmd := range parent.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}
	require.FailNowf(t, "subcommand not found", "no %q under %q", name, parent.Name())
	return nil
}
