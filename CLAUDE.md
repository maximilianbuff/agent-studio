# agent-studio

Autonomous GitHub agent. Runs headless Claude sessions on a cron schedule to triage issues and open pull requests.

## Build & test

```sh
cd cli && go build -o ../studio .   # build binary
cd cli && go test ./...             # run all tests
```

## Project layout

```
cli/                    Go CLI + HTTP backend (single binary: studio)
  cmd/                  Cobra commands (install, uninstall, jobs, ...)
  internal/
    assets/             Embedded default files (prompts, runner script, config)
    crontab/            Crontab block read/write — pure functions, fully tested
    gitops/             Git operations on ~/.agent-studio/
    home/               AGENT_STUDIO_HOME resolution
    prereqs/            Dependency checker (git, gh, claude)
frontend/               Vue 3 app (optional web UI)
install.sh              One-liner binary installer
defaults/               Source for embedded assets (see cli/internal/assets/files/)
```

Full design: [ARCHITECTURE.md](ARCHITECTURE.md)

## Key conventions

- All state lives in `~/.agent-studio/` (override: `AGENT_STUDIO_HOME`)
- That directory is a git repo — every config/prompt write auto-commits
- Crontab is managed via a sentinel block (`BEGIN_AGENT_STUDIO` / `END_AGENT_STUDIO`)
- The runner script at `~/.agent-studio/bin/agent-studio-run` is referenced by full path — no PATH dependency for cron
- Per-job `flock` lockfile prevents overlapping cron runs

## Adding a new CLI command

1. Create `cli/cmd/<name>.go` with a `var <name>Cmd = &cobra.Command{...}`
2. Register it in `cli/cmd/root.go`: `rootCmd.AddCommand(<name>Cmd)`
3. Add tests in `cli/cmd/<name>_test.go` using `runStudio(t, studioHome, ...)`
