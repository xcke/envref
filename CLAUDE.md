# Project Instructions

## Environment

- **Workspace:** Repository root — all project files live here
- **Agent Prompt:** `AGENT_PROMPT.md` — defines the agent loop and workflow
- **Status Tracking:** `STATUS.md` — current state snapshot (kept concise)
- **Task Queue:** `BACKLOG.md` — prioritized task list
- **Observer Inbox:** `inbox/` — drop `.md` files here to inject work

## Stack

- **Stack Definition:** `STACK.md` — tech stack, verification commands, language
  conventions, and bootstrapping steps. Read this before executing any tasks.

## Workflow

1. Check for dirty tree (crash recovery), read `STATUS.md` and `BACKLOG.md`, process inbox
2. Pick one focused task from BACKLOG.md (highest priority TODO)
3. Execute the task — plan first if touching 3+ files
4. Verify the work using the commands defined in `STACK.md`
5. Update BACKLOG.md + STATUS.md, then commit everything in a **single atomic commit**

## Git Conventions

Commit messages follow the format: `type: description [TASK-ID] [iter-N]`

- `type: description` — what was done (not "update STATUS.md")
- `[TASK-ID]` — task ID from BACKLOG.md (e.g., `ENV-010`)
- `[iter-N]` — iteration number (from `$AGENT_ITERATION` env var)

Types: `feat`, `fix`, `refactor`, `docs`, `style`, `test`, `chore`

Examples:
```
feat: implement .env file parser [ENV-010] [iter-5]
fix: handle BOM and CRLF in parser [ENV-016] [iter-12]
chore: scaffold Go project with Cobra CLI
```

## BACKLOG.md Format

```markdown
# Backlog

- [STATUS] PRIORITY | ID | Title
```

- **Status:** `TODO`, `IN_PROGRESS`, `DONE`, `BLOCKED`
- **Priority:** `P0` (hotfix/urgent), `P1` (high), `P2` (normal), `P3` (low)
- **ID:** Sequential, prefixed per project (e.g., `ENV-001`, `ENV-002`)

Example:
```markdown
# Backlog

- [DONE] P1 | ENV-001 | Initialize Go module
- [DONE] P1 | ENV-002 | Set up project directory structure
- [IN_PROGRESS] P1 | ENV-003 | Add Cobra CLI scaffold with root command
- [TODO] P1 | ENV-004 | Set up Makefile with build, test, lint targets
- [TODO] P2 | ENV-005 | Add .goreleaser.yml for cross-platform releases
- [BLOCKED] P2 | ENV-006 | Set up CI pipeline | blocked by ENV-004
```

The agent picks the highest-priority `TODO` item each iteration. The observer can reprioritize by editing BACKLOG.md directly or dropping a `.md` file in `inbox/`.

## STATUS.md Format

Keep STATUS.md under ~1500 tokens. It tracks **current state**, not history.

```markdown
# Project Status

## Last Completed
- ENV-003: Added Cobra CLI scaffold with root command and version flag [iter-3]

## Current State
- Go module `github.com/xcke/envref` initialized
- Cobra root command with `--version` flag
- Directory structure: cmd/envref/, internal/, pkg/

## Known Issues
- None currently
```

Do NOT duplicate "Next Steps" here — that lives in BACKLOG.md.

## Principles

- Write clean, idiomatic code in the project's primary language — simple, explicit, no magic
- Follow existing project conventions and patterns
- Don't try to do everything at once — one well-scoped task per iteration
- Don't commit broken code — verify before committing
- One atomic commit per iteration — work + status update together
- Keep STATUS.md concise — history lives in git log
- Handle errors explicitly — never ignore returned errors
- Prefer composition over inheritance (interfaces over embedding chains)
