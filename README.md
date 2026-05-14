# agent-studio

Autonomous GitHub agent. Runs headless Claude sessions on a schedule to triage issues and open pull requests — no manual intervention required.

## How it works

A system cron job fires every few minutes and spawns a headless Claude session. The session reads your configuration, scans GitHub for work, and opens pull requests. You stay in control via the `studio` CLI.

```
cron → agent-studio-run → claude --dangerously-skip-permissions → GitHub
```

All state lives in `~/.agent-studio/` — a plain git repository you own.

## Requirements

- Linux or macOS (amd64 or arm64)
- [git](https://git-scm.com/)
- [gh](https://cli.github.com/) — authenticated (`gh auth login`)
- [claude](https://claude.ai/code) — Claude Code CLI
- [Go](https://go.dev/dl/) 1.21+ — only needed if no pre-built binary is available for your platform

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/maximilianbuff/agent-studio/main/install.sh | sh
```

Then set up the state directory and cron jobs:

```sh
studio install
```

## Quick start

```sh
# 1. Point it at your repos
studio config repos add owner/repo

# 2. Set your GitHub login
studio config set my_login your-github-username

# 3. Check jobs are running
studio jobs list

# 4. Watch the logs
studio jobs logs scan --tail
```

## CLI reference

### Setup

```sh
studio install             # set up ~/.agent-studio/ and register cron jobs
studio install --dry-run   # preview without making changes
studio install --force     # overwrite default files (use when updating)
studio uninstall           # remove cron jobs
studio uninstall --purge   # remove cron jobs and delete ~/.agent-studio/
```

### Jobs

```sh
studio jobs list                          # show all jobs, schedule, and status
studio jobs run <job>                     # run a job immediately
studio jobs add <job> --schedule <expr>   # create a new job
studio jobs remove <job>                  # remove a job
studio jobs enable <job>                  # re-enable a disabled job
studio jobs disable <job>                 # pause a job (keeps schedule)
studio jobs logs <job>                    # show recent log output
studio jobs logs <job> --tail             # follow log output
```

### Prompts

```sh
studio prompts show <job>      # print the prompt
studio prompts edit <job>      # open $EDITOR (auto-commits on save)
```

### Config

```sh
studio config show

studio config set my_login <github-username>
studio config set min_score <n>

studio config repos list
studio config repos add <owner/repo> [--weight N]
studio config repos remove <owner/repo>

studio config labels set <label> <score>
studio config labels remove <label>

studio config keywords set <word> <score>
studio config keywords remove <word>
```

### Queue

```sh
studio queue show    # see what the worker will pick up next
studio queue clear   # empty the queue
```

### History

```sh
studio history          # git log of config/prompt changes
studio diff             # uncommitted changes
studio rollback <sha>   # restore config/prompts to a previous state
```

### Sync across machines

```sh
studio remote set git@github.com:you/agent-studio-config.git
studio push
# on another machine:
studio pull
```

## State directory

Everything lives in `~/.agent-studio/` — a git repository:

```
~/.agent-studio/
  config.json          ← committed — repos, labels, keywords
  prompts/
    scan.md            ← committed — scan agent prompt
    worker.md          ← committed — worker agent prompt
  bin/
    agent-studio-run   ← committed — cron runner script
  queue.json           ← git-ignored — current work queue
  logs/                ← git-ignored — cron output
```

Override the location with `AGENT_STUDIO_HOME`.

## Updating

```sh
curl -fsSL https://raw.githubusercontent.com/maximilianbuff/agent-studio/main/install.sh | sh
studio install --force   # refresh default prompts and runner script
```

## Uninstalling

```sh
studio uninstall --purge --yes   # remove cron jobs + delete ~/.agent-studio/
rm ~/.local/bin/studio           # remove the binary
```
