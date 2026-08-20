# peak ▲

The official CLI for [Himalaya](https://8848lab.org) — by 8848 Lab.

Deploy and manage your infrastructure from the terminal.

```bash
$ peak deploy
$ peak logs --follow
$ peak status my-project
$ peak login
```

---

## Project Structure

```
peak/
├── cmd/peak/            # Entrypoint (main.go)
├── internal/
│   ├── cli/             # All cobra commands
│   │   ├── root.go      # Root command + Execute()
│   │   ├── deploy.go    # `peak deploy` with bubbletea UI, project-link resolution
│   │   ├── logs.go      # `peak logs` — polls build/container logs, --follow
│   │   ├── status.go    # `peak status` — polls deployment status
│   │   ├── resolve.go   # resolveDeploymentID — shared by logs/status
│   │   └── login.go     # `peak login` — device-code auth flow
│   ├── api/
│   │   └── client.go    # Himalaya API client (device auth, projects, deployments)
│   └── archive/
│       └── archive.go   # tar.gz archiving of the current project for deploy
└── pkg/
    └── config/
        └── config.go    # Token storage (~/.peak/) + per-directory project link (.peak/project.json)
```

## Stack

- [`cobra`](https://github.com/spf13/cobra) — CLI commands
- [`bubbletea`](https://github.com/charmbracelet/bubbletea) — animated terminal UI
- [`lipgloss`](https://github.com/charmbracelet/lipgloss) — terminal styling
- [`bubbles`](https://github.com/charmbracelet/bubbles) — spinner, input components

## Getting Started

```bash
# Install dependencies
go mod tidy

# Build
go build -o peak ./cmd/peak

# Run
./peak deploy
./peak deploy --env staging
./peak logs --follow
./peak status my-project
```

## Roadmap

- [x] `peak login` — device-code auth flow against Himalaya (start/poll, token saved to `~/.peak/token`)
- [x] `peak deploy` — archives the project, uploads it, and polls the Himalaya API for build/deploy progress
- [x] `peak logs` — polls the Himalaya API for build logs, switching to container logs once the deployment is `ready`; `--follow` re-polls every 2s and prints only new output
- [x] `peak status` — polls the Himalaya API for real deployment status, colored by state (ready/failed/in-progress)
- [ ] `peak env set KEY=VALUE` — manage environment variables
- [ ] `peak rollback` — roll back to previous deployment

Log and status updates are polling-based (2s interval), not SSE streaming, and auth is a device-code flow rather than OAuth — see `internal/cli/logs.go`, `internal/cli/status.go`, and `internal/cli/login.go`.
