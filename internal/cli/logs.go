package cli

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/8848lab/peak/internal/api"
	"github.com/8848lab/peak/pkg/config"
)

// logSource identifies which log stream a fetchLogs call read from, so the
// --follow loop knows to reset its print offset when the source changes
// (build logs and container logs are unrelated strings, not a single
// continuously-growing stream).
type logSource int

const (
	sourceBuild logSource = iota
	sourceContainer
)

var logsCmd = &cobra.Command{
	Use:   "logs [deployment-id]",
	Short: "Show logs for a deployment",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var arg string
		if len(args) > 0 {
			arg = args[0]
		}
		follow, _ := cmd.Flags().GetBool("follow")

		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if cfg.Token == "" {
			return fmt.Errorf("not logged in — run `peak login` first")
		}
		client := api.NewClient(cfg.APIBaseURL, cfg.Token)

		deploymentID, err := resolveDeploymentID(client, arg)
		if err != nil {
			return err
		}

		dim := lipgloss.NewStyle().Foreground(mutedText)
		tag := lipgloss.NewStyle().Foreground(orange).Bold(true)
		fmt.Printf("%s  logs for %s\n\n", tag.Render("▲ peak"), deploymentID)

		fetchLogs := func() (string, logSource, error) {
			deployment, err := client.GetDeployment(deploymentID)
			if err != nil {
				return "", sourceBuild, err
			}
			logs := ""
			if deployment.Logs != nil {
				logs = *deployment.Logs
			}
			if deployment.Status == "ready" {
				if containerLogs, err := client.GetContainerLogs(deploymentID); err == nil && containerLogs != "" {
					return containerLogs, sourceContainer, nil
				}
			}
			return logs, sourceBuild, nil
		}

		if !follow {
			logs, _, err := fetchLogs()
			if err != nil {
				if aerr := authError(err); aerr != nil {
					return aerr
				}
				return fmt.Errorf("could not fetch logs: %w", err)
			}
			fmt.Println(logs)
			return nil
		}

		fmt.Println(dim.Render("  (--follow enabled, press ctrl+c to stop)"))
		var lastLen int
		var lastSource logSource = sourceBuild
		for {
			logs, source, err := fetchLogs()
			if err != nil {
				if aerr := authError(err); aerr != nil {
					return aerr
				}
				return fmt.Errorf("could not fetch logs: %w", err)
			}
			if source != lastSource {
				lastLen = 0
				lastSource = source
			}
			if len(logs) > lastLen {
				fmt.Print(logs[lastLen:])
				lastLen = len(logs)
			}
			time.Sleep(2 * time.Second)
		}
	},
}

func init() {
	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
}
