package cli

import (
	"fmt"
	"os"

	"github.com/8848lab/peak/internal/api"
	"github.com/8848lab/peak/pkg/config"
)

// resolveDeploymentID returns arg unchanged if non-empty, or — when no arg
// was given — the most recent deployment of the current directory's linked
// project (see config.LoadProjectLink).
func resolveDeploymentID(client *api.Client, arg string) (string, error) {
	if arg != "" {
		return arg, nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	link, err := config.LoadProjectLink(wd)
	if err != nil {
		return "", fmt.Errorf("no deployment ID given and no linked project in this directory — run `peak deploy` first or pass a deployment ID")
	}

	deployments, err := client.ListDeployments(link.ProjectID)
	if err != nil {
		return "", fmt.Errorf("could not list deployments: %w", err)
	}
	if len(deployments) == 0 {
		return "", fmt.Errorf("no deployments found for this project yet")
	}
	return deployments[0].ID, nil
}
