package cli

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs [project]",
	Short: "Tail live logs from your deployment",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project := "your-project"
		if len(args) > 0 {
			project = args[0]
		}

		follow, _ := cmd.Flags().GetBool("follow")

		dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b6b6b"))
		tag := lipgloss.NewStyle().Foreground(orange).Bold(true)

		fmt.Printf("%s  tailing logs for %s\n\n",
			tag.Render("▲ peak"),
			lipgloss.NewStyle().Bold(true).Render(project+".8848.app"),
		)

		// TODO: connect to Himalaya API and stream logs via SSE
		// For now, placeholder
		fmt.Println(dim.Render("  waiting for log stream..."))
		if follow {
			fmt.Println(dim.Render("  (--follow enabled, press ctrl+c to stop)"))
		}

		return nil
	},
}

func init() {
	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
}
