package cmd

import (
	"fmt"

	"github.com/stenstromen/cask/resource"

	"github.com/spf13/cobra"
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

		mode := resource.ModeShell
		if miscellaneous {
			mode = resource.ModeMiscellaneous
		}
		if git {
			mode = resource.ModeGit
		}
		return resource.ApiRequest(mode, args)
	},
}

func init() {
	rootCmd.AddCommand(queryCmd)
	queryCmd.Flags().BoolP("shell", "s", false, "Suggest shell commands for a goal")
	queryCmd.Flags().BoolP("miscellaneous", "m", false, "Ask a general technical question")
	queryCmd.Flags().BoolP("git", "g", false, "Suggest git commands for a goal")
}
