package cmd

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

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

		// Miscellaneous answers are prose, not a list of commands: print the
		// text wrapped to a sane width and don't offer to copy anything.
		if mode == resource.ModeMiscellaneous {
			answer := strings.Join(items, " ")
			fmt.Fprintln(cmd.OutOrStdout(), wrapText(answer, wrapWidth(cmd)))
			return nil
		}

		for i, item := range items {
			fmt.Fprintf(cmd.OutOrStdout(), "%d. %s\n", i+1, item)
		}

		return copyPrompt(cmd, items)
	},
}

// wrapWidth picks a wrapping width: capped at 80 columns so output doesn't run
// across very wide terminals, and shrunk to fit narrow ones.
func wrapWidth(cmd *cobra.Command) int {
	const max = 80
	width := max
	if out, ok := cmd.OutOrStdout().(*os.File); ok {
		if w, _, err := term.GetSize(int(out.Fd())); err == nil && w > 0 && w < width {
			width = w
		}
	}
	return width
}

// wrapText soft-wraps s at word boundaries so no line exceeds width runes.
// Words longer than width are left intact on their own line.
func wrapText(s string, width int) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	lineLen := 0
	for i, w := range words {
		wl := utf8.RuneCountInString(w)
		if i == 0 {
			b.WriteString(w)
			lineLen = wl
			continue
		}
		if width > 0 && lineLen+1+wl > width {
			b.WriteByte('\n')
			b.WriteString(w)
			lineLen = wl
			continue
		}
		b.WriteByte(' ')
		b.WriteString(w)
		lineLen += 1 + wl
	}
	return b.String()
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
