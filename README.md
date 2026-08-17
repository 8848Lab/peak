# peak ▲

The official CLI for [Himalaya](https://8848lab.org) — by 8848 Labs.

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
├── cmd/peak/          # Entrypoint (main.go)
├── internal/
│   ├── cli/           # All cobra commands
│   │   ├── root.go    # Root command + Execute()
│   │   ├── deploy.go  # `peak deploy` with bubbletea UI
│   │   ├── logs.go    # `peak logs`
│   │   ├── status.go  # `peak status`
│   │   └── login.go   # `peak login`
│   └── api/
│       └── client.go  # Himalaya API client
└── pkg/
    └── config/
        └── config.go  # Config + token storage (~/.peak/)
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

- [ ] `peak deploy` — trigger deploy, stream progress from Himalaya API
- [ ] `peak logs` — SSE log streaming
- [ ] `peak status` — real health data from API
- [ ] `peak login` — OAuth flow against Himalaya
- [ ] `peak env set KEY=VALUE` — manage environment variables
- [ ] `peak rollback` — roll back to previous deployment
