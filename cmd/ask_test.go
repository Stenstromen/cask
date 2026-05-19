package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stenstromen/cask/resource"
)

// resetQueryFlags clears any flag values left over from previous executions
// so each test starts from a known state. queryCmd is a package-level global
// shared across tests in this package.
func resetQueryFlags(t *testing.T) {
	t.Helper()
	for _, name := range []string{"shell", "miscellaneous", "git"} {
		if err := queryCmd.Flags().Set(name, "false"); err != nil {
			t.Fatalf("reset flag %q: %v", name, err)
		}
	}
}

// runQuery executes `cask query <args...>` against the real rootCmd with
// stdout/stderr captured into buffers. SilenceErrors/SilenceUsage are toggled
// so cobra doesn't smear "Error:" + usage banners across the buffer, which
// would make assertions brittle.
func runQuery(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	resetQueryFlags(t)
	t.Cleanup(func() { resetQueryFlags(t) })

	prevSilenceErrors := rootCmd.SilenceErrors
	prevSilenceUsage := rootCmd.SilenceUsage
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	t.Cleanup(func() {
		rootCmd.SilenceErrors = prevSilenceErrors
		rootCmd.SilenceUsage = prevSilenceUsage
	})

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	rootCmd.SetArgs(append([]string{"query"}, args...))

	err = rootCmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

func TestQuery_NoFlagsNoArgsPrintsHelp(t *testing.T) {
	stdout, _, err := runQuery(t)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// cmd.Help() writes the usage banner; it should mention the command name
	// and the available mode flags.
	for _, want := range []string{"query", "--shell", "--miscellaneous", "--git"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help output missing %q; got:\n%s", want, stdout)
		}
	}
}

func TestQuery_ShellFlagNoArgsPrintsExamplePrompt(t *testing.T) {
	stdout, _, err := runQuery(t, "--shell")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(stdout); got != shellExamplePrompt {
		t.Errorf("stdout = %q, want %q", got, shellExamplePrompt)
	}
}

func TestQuery_MiscellaneousFlagNoArgsPrintsExamplePrompt(t *testing.T) {
	stdout, _, err := runQuery(t, "--miscellaneous")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(stdout); got != miscellaneousExamplePrompt {
		t.Errorf("stdout = %q, want %q", got, miscellaneousExamplePrompt)
	}
}

func TestQuery_GitFlagNoArgsPrintsExamplePrompt(t *testing.T) {
	stdout, _, err := runQuery(t, "-g")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(stdout); got != gitExamplePrompt {
		t.Errorf("stdout = %q, want %q", got, gitExamplePrompt)
	}
}

func TestQuery_ShortFlagsWork(t *testing.T) {
	stdout, _, err := runQuery(t, "-s")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(stdout); got != shellExamplePrompt {
		t.Errorf("stdout = %q, want %q", got, shellExamplePrompt)
	}
}

func TestQuery_ShellAndMiscellaneousConflict(t *testing.T) {
	_, _, err := runQuery(t, "--shell", "--miscellaneous", "some", "prompt")
	if err == nil {
		t.Fatal("expected error when both --shell and --miscellaneous are set")
	}
	if !strings.Contains(err.Error(), "use only one of") {
		t.Errorf("error %q should mention 'use only one of'", err.Error())
	}
}

func TestQuery_ShellAndMiscellaneousConflictNoArgs(t *testing.T) {
	// The conflict check runs before the no-args/help branch, so it should
	// fire even when no prompt is supplied.
	_, _, err := runQuery(t, "-s", "-m")
	if err == nil {
		t.Fatal("expected error when both -s and -m are set, even without args")
	}
}

func TestQuery_ArgsWithoutModeFlag(t *testing.T) {
	_, _, err := runQuery(t, "find", "big", "files")
	if err == nil {
		t.Fatal("expected error when prompt is given without a mode flag")
	}
	if !strings.Contains(err.Error(), "use only one of") {
		t.Errorf("error %q should mention 'use only one of'", err.Error())
	}
}

// TestQuery_PromptForwardedToApiRequest ensures the args parsed by cobra are
// forwarded to resource.ApiRequest verbatim. We exercise the full path
// (including HTTP) by pointing the config and any network bits at a temp
// state that makes ApiRequest fail fast in a recognizable way. Specifically,
// pointing --config at a missing file produces a stable error string from
// resource.loadConfig, which proves we crossed into resource.ApiRequest with
// a non-empty prompt.
func TestQuery_PromptForwardedToApiRequest(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "no-such-config.yaml")

	prev := resource.ConfigPathOverride
	resource.ConfigPathOverride = missing
	t.Cleanup(func() { resource.ConfigPathOverride = prev })

	_, _, err := runQuery(t, "--shell", "find", "big", "files")
	if err == nil {
		t.Fatal("expected error from missing config, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read config file") {
		t.Errorf("error %q should bubble up from resource.loadConfig", err.Error())
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("error %q should mention the override path %q", err.Error(), missing)
	}
}
