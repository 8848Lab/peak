# peak ▲

The official CLI for [Himalaya](https://8848lab.org) — deploy and manage your infrastructure from the terminal.

Built with Go + [Charm](https://charm.sh). Part of [8848 Lab](https://8848lab.org).

```
$ peak login
$ peak deploy
$ peak logs --follow
$ peak status my-project
```

---

## Install

```bash
npm install -g @8848lab/peak@beta
```

Requires Node.js to install. The CLI itself is a native Go binary — Node is only used for distribution.

---

## Commands

| Command | Description |
|---|---|
| `peak login` | Authenticate with Himalaya via device-code flow |
| `peak deploy` | Archive and deploy the current project |
| `peak deploy --env staging` | Deploy to a specific environment |
| `peak logs` | Fetch build and container logs |
| `peak logs --follow` | Stream logs, re-polling every 2s |
| `peak status <project>` | Check deployment status |

---

## Project Structure

```
peak/
├── cmd/peak/            # Entrypoint (main.go)
├── internal/
│   ├── cli/             # Cobra commands
│   │   ├── root.go      # Root command + Execute()
│   │   ├── deploy.go    # peak deploy — Bubbletea UI, project-link resolution
│   │   ├── logs.go      # peak logs — polls build/container logs, --follow
│   │   ├── status.go    # peak status — polls deployment status
│   │   ├── resolve.go   # resolveDeploymentID — shared by logs/status
│   │   └── login.go     # peak login — device-code auth flow
│   ├── api/
│   │   └── client.go    # Himalaya API client
│   └── archive/
│       └── archive.go   # tar.gz project archiving for deploy
└── pkg/
    └── config/
        └── config.go    # Token storage (~/.peak/) + project linking (.peak/project.json)
```

---

## Stack

- [`cobra`](https://github.com/spf13/cobra) — CLI commands
- [`bubbletea`](https://github.com/charmbracelet/bubbletea) — animated terminal UI
- [`lipgloss`](https://github.com/charmbracelet/lipgloss) — terminal styling
- [`bubbles`](https://github.com/charmbracelet/bubbles) — spinner, input components

---

## Local Development

```bash
go mod tidy
go build -o peak ./cmd/peak
./peak --help
```

---

## Roadmap

- [x] `peak login` — device-code auth, token saved to `~/.peak/token`
- [x] `peak deploy` — archives project, uploads, polls for build/deploy progress
- [x] `peak logs` — polls build logs, switches to container logs on `ready`; `--follow` streams
- [x] `peak status` — real-time deployment status, coloured by state
- [ ] `peak env set KEY=VALUE` — manage environment variables
- [ ] `peak rollback` — roll back to previous deployment

---

## License

MIT © [8848 Lab](https://8848lab.org)
