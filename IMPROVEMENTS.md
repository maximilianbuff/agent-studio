# AgentStudio — Improvement Analysis

Observed after running the agent on `nurmaso/byrding` and `maximilianbuff/agent-studio`.
PRs analysed: #61–#75 (byrding), #12 (agent-studio).

---

## 1. PR Overlaps & Automated Versioning

### Problem

10 of the 15 recent PRs all target `main` directly:

```
PR#75  issue/2    → main   (watchState)
PR#71  issue/34   → main   (extension scaffold)
PR#70  issue/41   → main   (react HMR)
PR#69  issue/42   → main   (vue HMR)
PR#68  issue/45   → main   (generics)
...
```

When PR#61 merges, every subsequent PR touching overlapping files gets a merge conflict.

Manual version bump PRs (`chore: bump all packages to 0.1.0`, `chore: version packages 0.1.1`) compound this: they modify `package.json` / `jsr.json` in every package, colliding with any feature PR open at the same time.

### Fix: release-please

Replace manual version bumps with Google's `release-please` GitHub Action. It reads commit history following Conventional Commits (`feat:` → minor, `fix:` → patch, `feat!:` → major), auto-generates a release PR, and manages changelogs. No agent should ever write `chore: bump version`.

**Add to each managed repo** — `.github/workflows/release-please.yml`:

```yaml
name: Release Please
on:
  push:
    branches: [main]
permissions:
  contents: write
  pull-requests: write
jobs:
  release-please:
    runs-on: ubuntu-latest
    steps:
      - uses: googleapis/release-please-action@v4
        with:
          release-type: node
          token: ${{ secrets.GITHUB_TOKEN }}
```

For monorepos (byrding uses pnpm workspaces):

```yaml
        with:
          release-type: node
          config-file: release-please-config.json
          manifest-file: .release-please-manifest.json
```

**Rule to add to `worker.md`:** never commit `chore: bump version` — release-please handles this.

---

## 2. PR Stacking & Conflict Resolution

### What's Working

The agent correctly stacked PRs for the Chrome extension feature:

```
main ← issue/34 (#71) ← issue/35 (#72) ← issue/36 (#73) ← issue/37 (#74)
```

### What's Broken

Issues without explicit dependency weren't stacked — all went to `main`. Any two merging in sequence produces conflicts.

### Fix A: Smarter base-branch selection in worker.md

Before branching, worker checks for open PRs touching the same packages/directories:

```sh
gh pr list --repo <repo> --state open --json headRefName,files \
  | jq '.[] | select(.files[].path | startswith("<affected_dir>"))'
```

If found, branch off that PR's head instead of `main`. This guarantees a dependency chain without manual coordination.

### Fix B: Conflict + review check before new work (Phase 0)

Add to `worker.md` — run before claiming any issue:

```
PHASE 0 — CHECK OWN OPEN PRs
  gh pr list --author @me --state open --json number,mergeable,reviewDecision,comments

  For each PR:
  - mergeable == "CONFLICTING" → fix and force-push, then exit
  - unresolved review comments → address them, then exit
  - CI failing → investigate and fix, then exit

  Only claim new work if all own PRs are green.
```

### Fix C: in-progress label on claim

Worker adds label immediately on claiming an issue:

```sh
gh issue edit <number> --repo <repo> --add-label "in-progress"
```

Add `in-progress` to `skip_labels` in scan so it's never picked up twice.

---

## 3. Worker Concurrency & Interval

### Current Problems

- Worker runs every 2 minutes — too aggressive. Sessions overlap, workers can't learn from each other's output before the next one starts.
- Max concurrent workers is hardcoded to 1 via a single lockfile. Should be configurable.

### Changes

**Default interval: `*/10 * * * *`** (every 10 minutes). This gives each worker session time to complete most tasks before the next fires. Configurable via `studio config set worker_interval "*/10 * * * *"`.

**Max concurrent workers** — default 1, configurable via `studio config set max_workers 1`. The runner uses numbered lockfiles (`worker-1.lock`, `worker-2.lock`, ...) and claims the first free slot. If all slots are taken, exits cleanly.

**Applying a schedule change** — `studio config set worker_interval` also updates the live crontab immediately (no need to re-run install).

**Config keys (implemented):**

```sh
studio config set worker_interval "*/10 * * * *"
studio config set scan_interval "7 */2 * * *"
studio config set max_workers 1
```

---

## 4. Authentication: API Key vs Subscription

### Problem

Studio currently assumes `claude` CLI is authenticated via Claude.ai subscription (browser-based login). Users running in headless/server environments may prefer API key auth. Both modes should be supported and switchable via CLI.

### Config schema

```json
{
  "auth_mode": "subscription",
  "anthropic_api_key": ""
}
```

- `auth_mode: "subscription"` — uses existing `claude` CLI session (default, no key needed)
- `auth_mode: "api_key"` — sets `ANTHROPIC_API_KEY` env var before invoking claude

### CLI

```sh
studio config set auth_mode api_key
studio config set anthropic_api_key sk-ant-...   # stored in config.json (chmod 600)
studio config set auth_mode subscription          # clears key from env, uses session
```

The runner script checks `auth_mode` from config and exports `ANTHROPIC_API_KEY` if set. The key is never logged.

### Token quota note

Anthropic does not expose a real-time usage API for subscription accounts. For API key users, the usage endpoint is available but rate-limited. Quota management is therefore implemented as a configurable **local threshold** rather than live API polling:

```sh
studio config set daily_token_budget 500000     # soft limit, agent-estimated
studio config set token_threshold 0.80          # pause above 80% of budget
```

The runner estimates tokens consumed per session (prompt length × heuristic multiplier), accumulates in a local cache, and skips runs above threshold until the daily window resets. This is approximate but effective for preventing runaway consumption.

---

## 5. Token Efficiency

### Problem

Every cron invocation spawns a fresh Claude session. First ~5–15k tokens re-establish context that was known last session.

### Strategy A: Compact repo map via repomix (highest ROI)

Pre-generate a compact single-file repo representation before invoking Claude:

```sh
# In agent-studio-run, before exec claude:
npx repomix --output /tmp/repo-context.txt --ignore "node_modules,dist,*.lock"
```

**Tools compared:**

| Tool | What it does | Token reduction |
|------|-------------|-----------------|
| [repomix](https://github.com/yamadashy/repomix) | Full repo → XML/markdown, respects .gitignore | ~40% |
| [caveman](https://github.com/JuliusBrussee/caveman) | Strips comments, whitespace, renames identifiers | ~60% — lossy, risky for code-gen tasks |
| [code2prompt](https://github.com/mufeedvh/code2prompt) | Selective file inclusion with Handlebars templates | ~30%, best for scoped tasks |
| `git diff HEAD~10` | Only changed files since last session | best for fix/patch sessions |

**Recommendation:** repomix for full-repo orientation, code2prompt for scoped issue work. Caveman is too lossy for tasks that produce code — identifiers get mangled and Claude can't write correct references.

### Strategy B: Persistent session knowledge

`~/.agent-studio/context/<owner>-<repo>.md` — running summary appended after each session:

```markdown
# nurmaso/byrding — agent context
Last updated: 2026-05-15T11:00Z

## Architecture
- pnpm monorepo: core, react, vue, vite, devtools-extension
- Build: tsup, published to JSR + npm
- Tests: vitest, co-located with source

## Patterns
- Store generics: MergedStore<TState, TActions>
- Plugin interface: onInit, onAction, onStateChange

## Open PRs (agent-authored)
- #71 issue/34 → main (extension scaffold, base for #72–74)
- #75 issue/2 → main (watchState vanilla)

## Completed this week
- #61 vitest, #62 HMR registry, #63 createMockStore
```

Worker reads this at session start. Worker appends a 5-line summary at session end. Cost: ~500 tokens to read a summary vs ~8k to re-explore the repo.

### Strategy C: Selective file inclusion

Map the issue to affected directories before reading any code:

```sh
gh issue view <n> --json body,title | grep -oE '@byrding/[a-z-]+' | sort -u
```

Then only read those package directories. Alone reduces per-session input tokens 50–80% for scoped issues.

---

## 6. Issue In-Progress Tracking

### Current State

Scan skips issues where `branch issue/<n> already exists`. Works but has a race: two workers can start simultaneously before either creates the branch.

### Open PR Conflict Map (current)

| PR | Issue | Area | Conflicts with |
|----|-------|------|----------------|
| #75 | #2 watchState | core | #68 (generics, same files) |
| #70 | #41 react HMR | react | #68 (react MergedStore) |
| #69 | #42 vue HMR | vue | #68 (vue MergedStore) |
| #68 | #45/#46 generics | react+vue | #70, #69 |
| #67 | #50 vite plugin | vite | standalone |
| #66 | #33 renderStore | react testing | #70 |
| #65 | #49 getContext | core | #64 |
| #64 | #48 getContext impl | core | #65 |
| #63 | #32 createMockStore | core testing | standalone |
| #62 | #40 HMR registry | core | #65, #64 |

High-conflict cluster: **#64, #65, #62** all touch core registry. **#68, #69, #70** all touch react/vue.

### Fix

- `in-progress` label on claim (see §2 Fix C) — prevents race
- Phase 0 conflict check before new work

---

## Implementation Priority

| # | Item | Effort | Impact |
|---|------|--------|--------|
| 1 | Worker interval → 10 min, configurable | S | high — prevents session overlap |
| 2 | Max concurrent workers, numbered lockfiles | S | medium — enables parallelism |
| 3 | `auth_mode` + `anthropic_api_key` in config | S | high — unblocks API key users |
| 4 | release-please in managed repos | S | high — eliminates version bump conflicts |
| 5 | `config set worker_interval` updates live crontab | M | high — UX |
| 6 | Worker Phase 0 conflict check | S | high — stops cascading failures |
| 7 | `in-progress` label on claim | S | medium — prevents duplicate work |
| 8 | context/<repo>.md session summary | M | medium — token savings |
| 9 | repomix pre-context in runner | M | high — 30–50% token reduction |
| 10 | Smarter base-branch selection | L | medium — reduces merge conflicts |
