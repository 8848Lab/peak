# @8848lab/peak ▲

The official CLI for [Himalaya](https://8848lab.org) — deploy and manage your infrastructure from the terminal.

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

**Supported platforms:** Linux (x64, arm64), macOS (x64, arm64), Windows (x64)

---

## Commands

| Command | Description |
|---|---|
| `peak login` | Authenticate with Himalaya |
| `peak deploy` | Deploy the current project |
| `peak deploy --env staging` | Deploy to a specific environment |
| `peak logs` | View build and container logs |
| `peak logs --follow` | Stream logs live |
| `peak status <project>` | Check deployment status |

---

## About

Peak is the companion CLI for [Himalaya](https://8848lab.org), the deployment platform by [8848 Lab](https://8848lab.org).

- GitHub: [github.com/8848Lab/peak](https://github.com/8848Lab/peak)
- License: MIT
