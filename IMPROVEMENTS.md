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

When PR#61 merges, every subsequent PR touches overlapping files → merge conflicts guaranteed.

Manual version bump PRs (`chore: bump all packages to 0.1.0`, `chore: version packages 0.1.1`) compound this: they modify `package.json` / `jsr.json` in every package, which collides with any feature PR touching the same packages.

### Root Cause

- Worker prompt doesn't check whether open PRs touch the same files before picking a new issue.
- Version bumping is done manually by the agent as a separate commit/PR — it shouldn't be.

### Proposed Fix: release-please

Replace manual version bumps with Google's `release-please` GitHub Action. It:
- Reads commit history following Conventional Commits (`feat:` → minor, `fix:` → patch, `feat!:` → major)
- Auto-generates a "Release PR" that aggregates all pending version bumps and changelogs
- No agent should ever write `chore: bump version` — that's now the pipeline's job

**Implementation** — add `.github/workflows/release-please.yml` to each managed repo:

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
          release-type: node          # or "simple" for non-JS repos
          token: ${{ secrets.GITHUB_TOKEN }}
```

For monorepos (byrding uses pnpm workspaces):

```yaml
        with:
          release-type: node
          config-file: release-please-config.json
          manifest-file: .release-please-manifest.json
```

**Agent rule to add to `worker.md`:** never commit `chore: bump version` — rely on release-please.

---

## 2. PR Stacking & Conflict Resolution

### What's Working

The agent correctly stacked PRs for the Chrome extension feature:

```
main ← issue/34 (#71) ← issue/35 (#72) ← issue/36 (#73) ← issue/37 (#74)
```

It detected that issues 34→35→36→37 were sequential dependencies and built a chain.

### What's Broken

Issues without explicit dependency in title/epic weren't stacked — they all went to `main`. Any two of them merging in sequence produces conflicts.

### Fix A: Smarter base-branch selection in worker.md

Before branching, worker should run:

```sh
# Find any open PR that touches the same packages/directories
gh pr list --repo <repo> --state open --json headRefName,files \
  | jq '.[] | select(.files[].path | startswith("<affected_dir>"))'
```

If found, branch off that PR's head, not `main`. Set the new PR's base to that branch. This guarantees stacking without conflicts at merge time.

### Fix B: Conflict + review check before picking new work

Add **Phase 0** to `worker.md` — run before claiming any new issue:

```
PHASE 0 — CHECK OWN OPEN PRs
  gh pr list --author @me --state open --json number,mergeable,reviewRequests,comments
  
  For each PR:
  - If mergeable == "CONFLICTING": fix conflicts first, force-push, exit
  - If review comments exist and are unresolved: address them first, exit
  - If CI is failing: investigate and fix first, exit
  
  Only proceed to claim new work if all own PRs are green.
```

### Fix C: Mutual exclusion via issue labels

When worker claims an issue, immediately add `in-progress` label:

```sh
gh issue edit <number> --repo <repo> --add-label "in-progress"
```

Scan should skip issues with `in-progress` label (add to `skip_labels` in config or handle in scan.md).

---

## 3. Token Efficiency

### Problem

Every cron invocation spawns a fresh Claude session. First thing it does: read CLAUDE.md, explore the repo, understand the codebase. This is ~5–15k tokens per session just to re-establish context that was known last session.

### Strategy A: Compact repo map (highest ROI)

Use **repomix** (or similar) to pre-generate a compact single-file representation of the repo before invoking Claude:

```sh
# In agent-studio-run, before exec claude:
npx repomix --output /tmp/repo-context.txt --ignore "node_modules,dist,*.lock"
```

Pass it as part of the prompt or write it to a known location the agent reads first.

**Alternatives analysed:**

| Tool | What it does | Token reduction |
|------|-------------|-----------------|
| [repomix](https://github.com/yamadashy/repomix) | Full repo → single XML/markdown file, respects .gitignore | ~40% via deduplication |
| [caveman](https://github.com/JuliusBrussee/caveman) | Strips comments, whitespace, renames identifiers to minimal tokens | ~60% — lossy, risky for code generation tasks |
| [code2prompt](https://github.com/mufeedvh/code2prompt) | Repo → prompt with file tree + contents, Handlebars templates | ~30%, good for selective inclusion |
| `git diff HEAD~10` summary | Only send changed files since last session | situational, good for fix sessions |

**Recommendation:** repomix for full-repo tasks, `git diff` summary for patch/fix sessions. Caveman is too lossy for tasks that require writing code — identifiers get mangled.

### Strategy B: Persistent session knowledge

Add `~/.agent-studio/context/<repo>.md` — a running summary the agent appends to after each session:

```markdown
# nurmaso/byrding — agent context
Last updated: 2026-05-15

## Architecture
- pnpm monorepo: core, react, vue, vite, devtools-extension
- Build: tsup, published to JSR + npm
- Tests: vitest

## Known patterns
- Store generics: MergedStore<TState, TActions>
- Plugin interface: onInit, onAction, onStateChange

## Open PRs (agent-authored)
- #71 issue/34 — extension scaffold (base for #72–74)
- #75 issue/2 — watchState vanilla

## Completed this week
- #61 vitest setup, #62 HMR registry, #63 createMockStore
```

Worker reads this at session start instead of re-exploring. Worker appends a summary at session end.

### Strategy C: Selective file inclusion

Rather than full-repo dumps, worker should only read files relevant to the issue:

```sh
# Before reading any code, map the issue to affected directories
gh issue view <n> --json body,title | grep -oE '@byrding/[a-z-]+' | sort -u
# Then only read those package directories
```

This alone can cut per-session input tokens by 50–80% for scoped issues.

---

## 4. Issue In-Progress Tracking

### Current State

The scan agent skips issues where `branch issue/<n> already exists`. This works but has a race: two worker sessions can start simultaneously before either creates the branch.

### Gap Analysis — Open PRs

From the 30 most recent PRs, these are open and potentially overlapping (all target `main`):

| PR | Issue | Area | Conflicts with |
|----|-------|------|----------------|
| #75 | #2 watchState | core | #68 (generics touch same files) |
| #70 | #41 react HMR | react pkg | #68 (react MergedStore generics) |
| #69 | #42 vue HMR | vue pkg | #68 (vue MergedStore generics) |
| #68 | #45/#46 generics | react+vue | #70, #69 |
| #67 | #50 vite plugin | vite pkg | standalone — low risk |
| #66 | #33 renderStore | react testing | #70 (react pkg) |
| #65 | #49 getContext | core | #64 (same file) |
| #64 | #48 getContext impl | core | #65 |
| #63 | #32 createMockStore | core testing | standalone |
| #62 | #40 HMR registry | core | #65, #64 |

High-conflict cluster: **#64, #65, #62** all touch core registry files. **#68, #69, #70** all touch react/vue packages.

### Fix

- Add `in-progress` label on claim (see §2 Fix C)
- Scan skips `in-progress` issues
- Worker Phase 0 detects conflicts before new work

---

## 5. Token Quota Management

### Problem

Worker runs every 2 minutes. Each run costs tokens even for "no work" checks (~500 tokens). Full sessions cost 10k–100k tokens. No throttling → account can hit rate limits or monthly caps, blocking manual sessions.

### Option A: Pre-flight API usage check

Before invoking Claude, query usage and abort if above threshold:

```sh
# Anthropic usage API (if available)
USAGE=$(curl -s -H "x-api-key: $ANTHROPIC_API_KEY" \
  https://api.anthropic.com/v1/usage | jq '.daily_tokens_used')
LIMIT=$(curl -s ... | jq '.daily_limit')
RATIO=$(echo "$USAGE / $LIMIT" | bc -l)
if (( $(echo "$RATIO > $THRESHOLD" | bc -l) )); then
  echo "Usage at ${RATIO}% — skipping run"
  exit 0
fi
```

Anthropic does not yet expose a public real-time usage endpoint, so this is forward-looking.

### Option B: Local usage cache (recommended, implementable now)

SQLite db at `~/.agent-studio/usage.db` tracking:

```sql
CREATE TABLE sessions (
  id INTEGER PRIMARY KEY,
  job TEXT,
  repo TEXT,
  started_at TEXT,
  ended_at TEXT,
  estimated_tokens INTEGER,
  reset_at TEXT        -- when this quota window resets
);

CREATE TABLE quota (
  window TEXT PRIMARY KEY,   -- 'daily', 'monthly'
  limit_tokens INTEGER,
  threshold REAL,            -- 0.0–1.0, default 0.8
  next_reset TEXT            -- ISO8601
);
```

Before each run, check `SUM(estimated_tokens) / limit_tokens` for current window. If above threshold, compute `next_reset` and schedule the cron job to fire at exactly that time (`studio jobs reschedule worker <time>`).

### Option C: Scheduled reschedule on threshold hit

When usage threshold is exceeded, worker exits and calls:

```sh
studio jobs reschedule worker "$(date -d '+1 hour' --iso-8601=seconds)"
```

This adds a one-shot `at` job (or modifies crontab with a specific time) for the reset time, rather than running blindly every 2 minutes.

### CLI surface

```sh
studio config set token_threshold 0.80        # 0.0–1.0
studio config set daily_token_limit 500000
studio config set monthly_token_limit 5000000
studio quota show                              # current usage vs limits
studio quota reset                             # clear local cache
```

### Recommendation

Implement Option B now (local cache, no external API dependency). Add Option A later once Anthropic exposes usage endpoint. Wire Option C as the scheduling strategy when threshold is hit.

---

## Implementation Priority

| # | Item | Effort | Impact |
|---|------|--------|--------|
| 1 | release-please in managed repos | S | high — eliminates version bump conflicts |
| 2 | Worker Phase 0 conflict check | S | high — stops cascading PR failures |
| 3 | `in-progress` label on claim | S | medium — prevents duplicate work |
| 4 | repomix pre-context in runner | M | high — 30–50% token reduction |
| 5 | context/<repo>.md session summary | M | medium — improves session quality |
| 6 | Local usage DB + threshold check | M | high — prevents quota exhaustion |
| 7 | Smarter base-branch selection | L | medium — reduces merge conflicts |
| 8 | `studio quota` CLI commands | S | low — visibility only |
