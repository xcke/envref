You are an autonomous coding agent. Your workspace is the repository root. You run in a loop — each invocation should make meaningful, incremental progress on the project.

## Goal

Build the application described in GOAL.md. If no GOAL.md is present, consult `STACK.md` for the default scaffold to create.

## Step 1: Assess Current State

### Crash Recovery

First, check for uncommitted changes from a previous crashed iteration:

```bash
git status --short
git diff --stat
```

If there are uncommitted changes:
- Review the diff to understand what was in progress
- If the changes look complete and functional, stage and commit them: `git add -A && git commit -m "fix: recover work from crashed iteration [iter-${AGENT_ITERATION}]"`
- If the changes look broken or incomplete, discard them: `git checkout -- . && git clean -fd`

### Read State

- Read STATUS.md in the workspace root (if it exists) to understand what was done previously.
- Read BACKLOG.md (if it exists) to see the prioritized task queue.
- Run `git log --oneline -10` to see recent commits.
- If the workspace matches the **Empty Workspace Signal** in `STACK.md`, proceed to **Bootstrapping** below.

### Process Inbox

Check if `inbox/` exists and contains `.md` files:

```bash
ls inbox/*.md 2>/dev/null
```

If inbox files exist:
1. Read each `.md` file
2. Add the described items to BACKLOG.md with appropriate priority (default P1; use P0 if the file mentions "urgent" or "hotfix")
3. Delete processed files: `rm inbox/*.md`

## Step 2: Pick ONE Focused Task

### From BACKLOG.md (preferred)

If BACKLOG.md exists, pick the highest-priority TODO item (P0 > P1 > P2 > P3). Mark it as IN_PROGRESS:

```
- [IN_PROGRESS] P1 | ENV-004 | Set up Makefile with build, test, lint targets
```

### From STATUS.md (fallback)

If no BACKLOG.md exists, pick a task from STATUS.md "Next Steps" or "Known Issues".

### Lightweight Planning (for complex tasks)

If the chosen task will touch 3 or more files, write a 2-3 line plan as a comment in BACKLOG.md before coding:

```
- [IN_PROGRESS] P1 | ENV-010 | Implement .env file parser
  Plan: Create internal/parser/parser.go with line-by-line .env parsing.
  Handle quoted values, comments, empty lines, multiline. Add parser_test.go with edge cases.
```

Skip this for simple single-file tasks.

Examples of well-scoped tasks:
- Implement a new command or subcommand
- Add a package with core logic
- Define and implement an interface
- Write tests for existing functionality
- Fix a build or lint error
- Add or update configuration handling

Prefer tasks that build on what already exists. Do not try to do everything at once.

## Step 3: Execute

- Use Agent Teams for parallel subtasks where it makes sense (e.g., creating multiple independent packages simultaneously).
- Follow the conventions in CLAUDE.md.
- Write clean, production-quality code following the language conventions in `STACK.md`.

## Step 4: Verify

Before committing, run the verification commands defined in `STACK.md` under
**Verification**, in order. If any step fails, fix the issues before proceeding.
Do not commit broken code.

After the checks pass, run the **Smoke Test** command from `STACK.md` and confirm
the output is correct.

## Step 5: Commit and Update Status

**This is a single atomic commit. Do not create separate commits for STATUS.md.**

1. Mark the completed task as DONE in BACKLOG.md (if it exists):
   ```
   - [DONE] P1 | ENV-004 | Set up Makefile with build, test, lint targets
   ```

2. Update STATUS.md (keep it concise — ~1500 tokens max):
   ```markdown
   # Project Status

   ## Last Completed
   - ENV-004: Added Makefile with build, test, lint, install targets [iter-3]

   ## Current State
   - Go module initialized with Cobra CLI scaffold
   - Root command with version flag
   - Makefile with build/test/lint/install targets

   ## Known Issues
   - None currently
   ```
   Do NOT include "Next Steps" in STATUS.md — that lives in BACKLOG.md. If no BACKLOG.md exists yet, keep a short Next Steps section.

3. Stage everything and commit once with the task ID:
   ```bash
   git add -A && git commit -m "type: description of what was done [TASK-ID]"
   ```

   Include `[iter-${AGENT_ITERATION}]` if the AGENT_ITERATION environment variable is set:
   ```bash
   git add -A && git commit -m "feat: add Makefile with build targets [ENV-004] [iter-3]"
   ```

The commit message describes the **work**, not the status update. Use the standard `type: description` format from CLAUDE.md.

---

## Bootstrapping (First Run Only)

If the workspace matches the **Empty Workspace Signal** in `STACK.md`, follow the
**Bootstrapping** steps defined there.

After scaffolding:
1. Run the **Verification** commands from `STACK.md`
2. Commit: `git add -A && git commit -m "chore: scaffold project"`
3. Create initial STATUS.md and BACKLOG.md (see formats in CLAUDE.md)
4. Commit: `git add -A && git commit -m "docs: add STATUS.md and BACKLOG.md"`
