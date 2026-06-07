package cmd

import (
	"fmt"
	"os"

	"github.com/stenstromen/cask/resource"

	"github.com/adhocore/chin"
	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	shellExamplePrompt         = "find all files larger than 100MB in the current directory"
	miscellaneousExamplePrompt = "what is the difference between docker compose and docker stack?"
	gitExamplePrompt           = "how to revert the last commit?"
)

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Suggest shell commands or ask technical questions",
	Long: `Use -s/--shell for shell command suggestions, or -m/--miscellaneous for general technical questions, or -g/--git for git command suggestions.

Run without a prompt to print help, or with -s or -m or -g alone to see an example prompt.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		spinner := chin.New()
		shell, _ := cmd.Flags().GetBool("shell")
		miscellaneous, _ := cmd.Flags().GetBool("miscellaneous")
		git, _ := cmd.Flags().GetBool("git")
		if shell && miscellaneous {
			return fmt.Errorf("use only one of -s/--shell or -m/--miscellaneous or -g/--git")
		}

		if len(args) == 0 {
			switch {
			case shell:
				fmt.Fprintln(cmd.OutOrStdout(), shellExamplePrompt)
				return nil
			case miscellaneous:
				fmt.Fprintln(cmd.OutOrStdout(), miscellaneousExamplePrompt)
				return nil
			case git:
				fmt.Fprintln(cmd.OutOrStdout(), gitExamplePrompt)
				return nil
			default:
				return cmd.Help()
			}
		}

		if !shell && !miscellaneous && !git {
			return fmt.Errorf("use only one of -s/--shell or -m/--miscellaneous or -g/--git")
		}

		go spinner.Start()

		mode := resource.ModeShell
		if miscellaneous {
			mode = resource.ModeMiscellaneous
		}
		if git {
			mode = resource.ModeGit
		}

		items, err := resource.ApiRequest(mode, args)
		spinner.Stop()
		if err != nil {
			return err
		}

		for i, item := range items {
			fmt.Fprintf(cmd.OutOrStdout(), "%d. %s\n", i+1, item)
		}

		return copyPrompt(cmd, items)
	},
}

// copyPrompt lets the user press a number key to copy the matching item to the
// clipboard. It is a no-op when stdin is not an interactive terminal (e.g. when
// output is piped or during tests) so non-interactive usage still works.
func copyPrompt(cmd *cobra.Command, items []string) error {
	if len(items) == 0 {
		return nil
	}

	in, ok := cmd.InOrStdin().(*os.File)
	if !ok || !term.IsTerminal(int(in.Fd())) {
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nPress 1-%d to copy to clipboard (any other key to exit): ", len(items))

	key, err := readKey(in)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout())

	choice := int(key - '0')
	if choice < 1 || choice > len(items) {
		return nil
	}

	if err := clipboard.WriteAll(items[choice-1]); err != nil {
		return fmt.Errorf("failed to copy to clipboard: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Copied: %s\n", items[choice-1])
	return nil
}

// readKey puts the terminal in raw mode and reads a single keypress so the
// user doesn't have to hit Enter. The terminal state is always restored.
func readKey(f *os.File) (byte, error) {
	oldState, err := term.MakeRaw(int(f.Fd()))
	if err != nil {
		return 0, err
	}
	defer func() { _ = term.Restore(int(f.Fd()), oldState) }()

	buf := make([]byte, 1)
	if _, err := f.Read(buf); err != nil {
		return 0, err
	}
	return buf[0], nil
}

func init() {
	rootCmd.AddCommand(queryCmd)
	queryCmd.Flags().BoolP("shell", "s", false, "Suggest shell commands for a goal")
	queryCmd.Flags().BoolP("miscellaneous", "m", false, "Ask a general technical question")
	queryCmd.Flags().BoolP("git", "g", false, "Suggest git commands for a goal")
}
