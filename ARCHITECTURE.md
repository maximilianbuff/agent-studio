# AgentStudio — Architecture

## Philosophy

Work happens independently of the UI. A system cron job spawns headless Claude sessions on a schedule. Those sessions interact directly with GitHub. The CLI (`studio`) is the primary control plane — always available, no server required. The backend and frontend are optional overlays for observation.

All state lives in a single git-tracked directory: `~/.agent-studio/`. Git is a hard dependency — it provides the audit trail, rollback, and cross-machine sync for free.

```
┌─────────────────────────────────────────────────────┐
│  system crontab  (machine-level, always on)         │
│                                                     │
│  7 */2 * * *   agent-studio-run scan                │
│  */2 * * * *   agent-studio-run worker              │
└────────────┬────────────────────────────────────────┘
             │ spawns
             ▼
┌─────────────────────────────────────────────────────┐
│  headless Claude session                            │
│  claude --dangerously-skip-permissions -p <prompt>  │
│                                                     │
│  scan:    gh issue list → score → queue.json        │
│  worker:  pick top issue → implement → gh pr create │
└────────────┬────────────────────────────────────────┘
             │ reads/writes
             ▼
┌─────────────────────────────────────────────────────┐
│  GitHub                                             │
│  - issues, PRs, branches, comments                  │
└─────────────────────────────────────────────────────┘

        ┌──────────────────────────────────────────────┐
        │  ~/.agent-studio/   (git repo)               │
        │                                              │
        │  config.json          ← committed            │
        │  prompts/scan.md      ← committed            │
        │  prompts/worker.md    ← committed            │
        │  queue.json           ← git-ignored          │
        │  logs/                ← git-ignored          │
        │  .gitignore                                  │
        └──────┬──────────────────────┬───────────────┘
               │                      │
        ┌──────▼──────┐     ┌─────────▼──────────────┐
        │   studio    │     │   studio serve          │
        │   (CLI)     │     │   (HTTP backend)        │
        │             │     │                         │
        │  primary    │     │  thin HTTP wrapper      │
        │  control    │     │  around same ops,       │
        │  plane      │     │  served to frontend     │
        └─────────────┘     └──────────┬──────────────┘
                                       │
                            ┌──────────▼─────────────┐
                            │  Frontend               │
                            │  (Vue 3 + Vite)         │
                            │  optional web UI        │
                            └────────────────────────┘
```

---

## State Directory — `~/.agent-studio/`

Single source of truth for all layers. Override the path with `AGENT_STUDIO_HOME`.

```
~/.agent-studio/
  config.json          — repos, labels, keywords, agent settings
  prompts/
    scan.md            — scan agent prompt
    worker.md          — worker agent prompt
  bin/
    agent-studio-run   — cron runner script (full path used in crontab)
  queue.json           — written by scan, consumed by worker  [git-ignored]
  *.lock               — per-job flock files (scan.lock, worker.lock, …) [git-ignored]
  logs/
    scan.log           — stdout/stderr of scan cron runs      [git-ignored]
    worker.log         — stdout/stderr of worker cron runs    [git-ignored]
  .gitignore           — excludes queue.json, *.lock, logs/
```

### Git version control

`~/.agent-studio/` is a git repository. `studio install` initialises it. Every CLI write to `config.json` or `prompts/` auto-commits with a structured message.

**Auto-commit message format:**
```
config: add repo owner/name
config: set min_score to 5
config: remove label wontfix
prompts: update scan.md
prompts: update worker.md
```

**`.gitignore` content:**
```
queue.json
logs/
```

### Cross-machine sync

Point a git remote at any host (GitHub, self-hosted) and use `studio push/pull` to keep configs and prompts in sync across machines.

```sh
studio remote set git@github.com:you/agent-studio-config.git
studio push
# on another machine:
studio pull
```

---

## CLI — `studio`

Single Go binary. Reads/writes `~/.agent-studio/` directly — no server required. `studio serve` starts the optional HTTP backend using the same internal packages.

### Install and setup

```
studio install            # bootstrap: check deps, git init, dirs, crontab block
studio install --dry-run  # show what would be created without doing it
```

`studio install` checks for required dependencies (`git`, `gh`, `claude`) and exits with a clear message if any are missing.

### Jobs

Jobs are fully generic — `scan` and `worker` are defaults, but any name works.

```
studio jobs list                           # table: job | schedule | enabled | last-run | status
studio jobs add <job> --schedule <expr>    # create prompt stub, add crontab entry
studio jobs remove <job>                   # remove crontab entry + prompt file
studio jobs run <job>                      # spawn immediately, stream output to terminal
studio jobs enable <job>                   # uncomment/add crontab entry
studio jobs disable <job>                  # comment out entry (preserves schedule expression)
studio jobs logs <job>                     # print last 50 lines of job log
studio jobs logs <job> --tail              # tail -f the log
```

`jobs list` output example:
```
JOB      SCHEDULE      ENABLED  LAST RUN          STATUS
scan     7 */2 * * *   yes      2026-05-13 14:07  ok
worker   */2 * * * *   yes      2026-05-13 14:08  ok
triage   0 9 * * *     yes      2026-05-13 09:00  ok
```

`studio jobs add` creates a minimal prompt stub at `~/.agent-studio/prompts/<job>.md` and opens `$EDITOR` so you can fill it in immediately. Auto-commits the new prompt.

### Prompts

```
studio prompts show <job>         # print prompt to stdout
studio prompts edit <job>         # open $EDITOR, auto-commit on save
studio prompts set <job> < file   # pipe in a new prompt, auto-commit
```

### Config

```
studio config show                         # pretty-print config.json
studio config set my_login <login>         # set top-level scalar field
studio config set min_score <n>

studio config repos list
studio config repos add <owner/repo> [--weight N]
studio config repos remove <owner/repo>
studio config repos enable <owner/repo>
studio config repos disable <owner/repo>

studio config labels set <label> <score>   # add or update
studio config labels remove <label>
studio config keywords set <word> <score>
studio config keywords remove <word>

studio config skip-labels add <label>
studio config skip-labels remove <label>
```

Every subcommand that writes auto-commits with a structured message.

### Queue

```
studio queue show          # print queue.json sorted by score
studio queue clear         # truncate to empty items list
```

### History and rollback

```
studio history             # git log --oneline in ~/.agent-studio
studio diff                # git diff (uncommitted changes)
studio rollback <sha>      # git checkout <sha> -- config.json prompts/
```

### Remote sync

```
studio remote set <url>    # git remote add origin <url> + push --set-upstream
studio remote show         # print current remote
studio push                # git push
studio pull                # git pull --rebase
```

### Backend

```
studio serve               # start HTTP server on :8000 (for web UI)
studio serve --port 9000
```

---

## Cron Layer

### Runner script — `agent-studio-run`

Lives at `~/.agent-studio/bin/agent-studio-run` — git-tracked alongside config and prompts. The crontab references it by full path, so there is no PATH dependency for cron.

`studio` (the interactive CLI) is the only binary that needs to be on the user's PATH (installed to `~/.local/bin/studio` or equivalent via `make install`).

```sh
#!/usr/bin/env bash
set -euo pipefail
JOB="${1:?job name required}"
STUDIO_HOME="${AGENT_STUDIO_HOME:-$HOME/.agent-studio}"
PROMPT_FILE="$STUDIO_HOME/prompts/${JOB}.md"
LOCK_FILE="$STUDIO_HOME/${JOB}.lock"

# Cron strips most of the user environment — source profile to get claude and gh.
# shellcheck disable=SC1090
[[ -f "$HOME/.profile" ]] && source "$HOME/.profile"

if [[ ! -f "$PROMPT_FILE" ]]; then
  echo "agent-studio-run: prompt not found: $PROMPT_FILE" >&2
  exit 1
fi

# Per-job lock: if a previous run is still active, skip this invocation.
exec 9>"$LOCK_FILE"
if ! flock -n 9; then
  echo "agent-studio-run: $JOB already running, skipping" >&2
  exit 0
fi

exec claude --dangerously-skip-permissions -p "$(cat "$PROMPT_FILE")"
```

The lock is per-job (`scan.lock`, `worker.lock`, `triage.lock`, …) so concurrent jobs of different types run freely while preventing overlap within the same job type.

### Crontab block

The CLI reads/writes only the block between sentinel comments. Entries outside the block are never touched.

```
# BEGIN_AGENT_STUDIO — do not edit this block manually
# AGENT_STUDIO_JOB=scan
7 */2 * * *  ~/.agent-studio/bin/agent-studio-run scan >> ~/.agent-studio/logs/scan.log 2>&1
# AGENT_STUDIO_JOB=worker
*/2 * * * *  ~/.agent-studio/bin/agent-studio-run worker >> ~/.agent-studio/logs/worker.log 2>&1
# END_AGENT_STUDIO
```

`studio jobs enable/disable` rewrites this block by reading the full crontab, replacing the block, and piping to `crontab -`.

---

## Claude Session Behaviour

### Scan session (`prompts/scan.md`)

Goal: find open issues and PRs across configured repos, score them, write a prioritised work queue.

```
You are AgentStudio's scan agent. Run autonomously. Do not ask for confirmation.

CONFIG: Read ${AGENT_STUDIO_HOME:-~/.agent-studio}/config.json for:
  - repos[]         list of owner/repo to scan
  - labels{}        label → score mapping
  - keywords{}      keyword → score mapping
  - skip_labels[]   labels that disqualify an item
  - min_score       minimum score to include
  - my_login        GitHub login to check assignments

PHASE 1 — ISSUES
For each repo in config.repos where enabled == true:
  gh issue list --repo <repo> --state open --limit 100 \
    --json number,title,body,labels,assignees,url

  Score each issue:
  - Skip if any label matches skip_labels
  - Skip if assignees is non-empty and does not include my_login
  - Skip if a branch named issue/<number> already exists
  - Skip if a PR already exists that closes this issue
  - Add label scores, keyword matches (title + body), repo priority_weight

PHASE 2 — PRs NEEDING ATTENTION
  gh search prs --author @me --state open --limit 100 \
    --json number,title,url,repository

  For each PR, check reviews and unanswered comments.
  Score: changes_requested × 20 + unanswered_inline × 5 + unaddressed_discussion × 10

PHASE 3 — WRITE QUEUE
Write results to ${AGENT_STUDIO_HOME:-~/.agent-studio}/queue.json:
{
  "scanned_at": "<ISO8601>",
  "items": [
    { "type": "issue"|"pr_review", "repo": "...", "number": 0,
      "title": "...", "url": "...", "score": 0 },
    ...
  ]
}
Sort by score descending.
```

### Worker session (`prompts/worker.md`)

Goal: pick the highest-scored item from the queue, work on it end-to-end, remove it from the queue.

```
You are AgentStudio's worker agent. Run autonomously. Do not ask for confirmation.

PHASE 1 — CLAIM WORK
Read ${AGENT_STUDIO_HOME:-~/.agent-studio}/queue.json.
Pick the highest-scored item that does not already have an open PR or active branch.
If queue is empty or all items are already in progress, exit cleanly.
Remove the item from queue.json (write updated file back immediately).

PHASE 2 — WORK
If type == "issue":     follow the Issue Implementation Protocol below.
If type == "pr_review": follow the PR Review Protocol below.

═══ Issue Implementation Protocol ═══════════════════════════
  - gh issue view <number> --repo <repo> --json body,comments
  - Clone or update repo to /tmp/agent-studio-work/<repo>
  - Read CLAUDE.md if present
  - Create branch: issue/<number>
  - Implement fix. Follow existing conventions. Tests required.
  - Quality gates: tests pass, type check clean, lint clean
  - Self-review: git diff HEAD — no debug logs, no regressions
  - gh pr create --repo <repo> --title "fix(#<n>): <title>" \
      --head issue/<n> --body "Closes #<n>\n\n<summary>"

═══ PR Review Protocol ══════════════════════════════════════
  - gh pr view <number> --repo <repo> --comments
  - gh api repos/<repo>/pulls/<number>/comments
  - Checkout branch, address every review comment
  - Quality gates (same as above)
  - git push origin <branch>
  - Reply to each inline comment: "Fixed in <sha>: <what changed>"
  - gh pr comment <number> --repo <repo> --body "<summary>"
```

---

## GitHub State Model

Status is inferred from GitHub state — no separate task database.

| Status    | GitHub signal |
|-----------|---------------|
| `queued`  | In queue.json, no branch and no open PR yet |
| `running` | Branch `issue/<n>` exists, no PR open, pushed within last 30 min |
| `done`    | Open or merged PR referencing `Closes #<n>`, opened by bot login |
| `stalled` | Branch exists, last push > 30 min ago, no PR |
| `skipped` | Not in queue, no branch, no PR |

For PR reviews: `running` = last push to branch < 30 min ago; `done` = bot replied to all reviewers.

---

## Backend (`studio serve`)

Lightweight Go HTTP server. No internal workers. Stateless except for `~/.agent-studio/`. Reuses the same internal packages as the CLI — it is a thin HTTP adapter, not a separate implementation.

### API

```
GET  /api/status            — queue length, last scan time, running count
GET  /api/activity          — recent GitHub PRs/issues (shells out to gh)
GET  /api/queue             — current queue.json contents

GET  /api/jobs              — parsed crontab entries (AgentStudio block only)
POST /api/jobs/:job/enable  — enable cron entry
POST /api/jobs/:job/disable — disable cron entry
POST /api/jobs/:job/run     — spawn agent-studio-run <job> immediately

GET  /api/prompts/:job      — read prompt file
PUT  /api/prompts/:job      — write prompt file (auto-commits)

GET  /api/config            — read config.json
PUT  /api/config            — write config.json (auto-commits)

GET  /api/logs/:job         — last 200 lines of job log
GET  /api/history           — git log --oneline output
```

### GitHub activity query

`GET /api/activity` shells out to:
```sh
gh search prs --author <my_login> --limit 50 \
  --json number,title,url,state,repository,createdAt,mergedAt
```

Status inference runs in the backend from this data + queue.json.

---

## Frontend

Vue 3 + Vite + Tailwind. Three views. Talks only to `studio serve`.

### Dashboard

- **Status bar**: queue depth, last scan time, next scan (computed from cron expression + log mtime)
- **Activity feed**: recent PRs/issues, repo, title, status badge, time ago
- **Running now**: items with `running` status

### Schedule

- **Jobs table**: job name, schedule (human-readable), enabled toggle, last run, Run Now button
- **Prompt editor**: select job → textarea with prompt markdown, Save button
- **Log viewer**: last N lines of job log, auto-refreshing

### Config

- Repos list (add/remove, priority weight, enabled toggle)
- Label scores (add/edit/delete)
- Keyword scores (add/edit/delete)
- Skip labels
- `my_login` and `min_score` fields

---

## File Layout

```
agent-studio/             — this repo
  cli/                    — Go CLI + backend (single binary: studio)
    main.go               — cobra root command, dispatch
    cmd/
      install.go          — studio install
      jobs.go             — studio jobs *
      prompts.go          — studio prompts *
      config.go           — studio config *
      queue.go            — studio queue *
      history.go          — studio history / diff / rollback
      remote.go           — studio remote / push / pull
      serve.go            — studio serve (HTTP backend)
    internal/
      state/              — read/write config.json, prompts, queue
      crontab/            — parse/edit crontab block
      gitops/             — auto-commit, push, pull, log
      github/             — gh CLI wrappers
  frontend/               — Vue 3 app
  defaults/
    bin/
      agent-studio-run    — runner script (copied to ~/.agent-studio/bin/ on install)
    prompts/
      scan.md             — default scan prompt (copied on install)
      worker.md           — default worker prompt (copied on install)
    config.json           — starter config (copied on install)
  Makefile                — build and install targets
  ARCHITECTURE.md         — this document
```

### Makefile targets

```makefile
install:   ## Build studio and install to ~/.local/bin/studio
	go build -o ~/.local/bin/studio ./cli

build:     ## Build studio binary locally (./studio)
	go build -o studio ./cli

uninstall: ## Remove studio binary from ~/.local/bin/
	rm -f ~/.local/bin/studio
```

---

## Bootstrap (`studio install`)

```
Prerequisites checked:
  ✓ git
  ✓ gh (GitHub CLI, authenticated)
  ✓ claude

Steps:
  1. mkdir -p ~/.agent-studio/{bin,prompts,logs}
  2. write ~/.agent-studio/.gitignore  (queue.json, logs/)
  3. git init ~/.agent-studio && initial commit of .gitignore
  4. copy defaults/config.json            → ~/.agent-studio/config.json       (skip if exists)
  5. copy defaults/prompts/scan.md        → ~/.agent-studio/prompts/scan.md   (skip if exists)
  6. copy defaults/prompts/worker.md      → ~/.agent-studio/prompts/worker.md (skip if exists)
  7. copy defaults/bin/agent-studio-run   → ~/.agent-studio/bin/agent-studio-run + chmod +x
  8. git add -A && git commit -m "chore: add default config, prompts, and runner"
  9. add AgentStudio crontab block (studio jobs enable scan worker)
     — uses full path ~/.agent-studio/bin/agent-studio-run, no PATH dependency
```

After install, `studio jobs list` should show both jobs enabled and ready.
