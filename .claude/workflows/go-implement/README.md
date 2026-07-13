# Claude Code Workflows

## go-implement

A workflow that implements a Jira issue in three steps: write the code, write the tests, then review everything. Each step runs as a separate Claude Code agent. The human engineer reviews the final result and creates the PR.

### Prerequisites

Before running the workflow, make sure the following are in place. The workflow will fail or produce poor results if any of these are missing.

**Git state:**
- You are on a feature branch. The workflow refuses to run on `main` or `master`.
- The working tree is clean. No uncommitted or staged changes (untracked files are fine). Commit or stash first.

**Toolchain (the standard repo setup):**
- Go 1.25+ installed and on your PATH (version is defined in `go.mod`).
- `pre-commit` installed and hooks set up (`pre-commit install`). This handles golangci-lint, shellcheck, and abcgo automatically (versions pinned in `.pre-commit-config.yaml`).
- The repo builds successfully: `make build` passes.
- Linters pass: `make style` passes. This runs the pre-commit hooks, so make sure they're installed first.
- `mockery` is installed via `go install` by `make gen-mocks` if missing.
- `addlicense` is installed via `go get` by `make license` if missing.

**Sandbox (required for workflows):**
- The project ships with a sandbox configuration in `.claude/settings.json`. It is **disabled by default** for interactive use but **must be enabled before running workflows**. The workflow performs a canary check at startup and refuses to run if the sandbox is not active.
- To enable: set `"enabled": true` in the `sandbox` section of `.claude/settings.json`, then restart Claude Code.
- **macOS**: works out of the box (uses the built-in Seatbelt framework).
- **Linux**: install `bubblewrap` and `socat` (`sudo dnf install bubblewrap socat` on Fedora, `sudo apt install bubblewrap socat` on Ubuntu/Debian).
- If the sandbox dependencies are missing, Claude Code falls back to regular permission prompts (`failIfUnavailable` is false). The canary check will detect this and block the workflow.
- **First run**: you need to accept the workspace trust dialog once in an interactive Claude Code session. Run `claude` in the repo, accept the trust dialog, then exit. This is a one-time step per machine.
- If your `$GOPATH` is not the default `~/go`, add your GOPATH to `allowWrite` in `.claude/settings.local.json` (not checked into git).
- The sandbox also blocks access to sensitive credential environment variables (GitHub/GitLab tokens, AWS keys, SSH agent, kubeconfig) and restricts network access to Go module proxies and GitHub only. See `.claude/settings.json` for the full list.

**Jira MCP (required):**
- The Jira MCP server (`mcp-atlassian` or equivalent) must be connected to your Claude Code session. The workflow fetches the issue body from Jira using the issue key you provide.
- Jira MCP works regardless of sandbox settings (MCP tools are not affected by the Bash sandbox).

**Optional but recommended:**
- A design document for the feature (path passed as `designDoc`).
- BDD feature files from [insights-behavioral-spec](https://github.com/RedHatInsights/insights-behavioral-spec) cloned locally (passed as `bddPaths`).

### How to run

Invoke `/go-implement` from a Claude Code session with a JSON args object. The workflow needs at least a Jira issue key. The issue body is fetched automatically via the Jira MCP. For best results, also provide a design document path and BDD feature file paths.

```
# Just the issue key:
/go-implement { "issue": "CCXDEV-12345" }

# With a design doc:
/go-implement { "issue": "CCXDEV-12345", "designDoc": "/path/to/design-doc.md" }

# With BDD specs (clone insights-behavioral-spec first):
/go-implement { "issue": "CCXDEV-12345", "bddPaths": ["/path/to/example.feature"] }

# All three (ideal):
/go-implement { "issue": "CCXDEV-12345", "designDoc": "/path/to/design-doc.md", "bddPaths": ["/path/to/example.feature", "/path/to/example_2.feature"] }
```

### What happens

**Pre-flight.** Before the main phases, the workflow verifies that the sandbox is active (canary network check), fetches the Jira issue body via MCP, and checks git state (branch name, clean working tree, base SHA).

**Step 1: Implement.** Reads the specification and design document, writes the production code, then runs `make style` (which includes golangci-lint, shellcheck, and ABC complexity checks). Fixes any lint or compilation issues it finds, retrying up to 3 times before giving up. Also runs `go mod tidy`, `make gen-mocks`, and `make license` as needed.

**Step 2: Unit Tests.** Reads the diff from Step 1, reads existing tests for patterns, and writes new tests. Test scenarios come from the specification, design document, and BDD feature files. Runs `make style` to lint the new test files, `go test` to run them, `make coverage` to check coverage, and `make license` to add headers. If a test fails, the agent tries to fix it up to 3 times. If a test correctly asserts spec behavior but the implementation doesn't match, the test is kept and flagged as "implementation may not match spec" rather than weakened.

**Step 3: Verify.** An independent review of code, design, correctness. Reads the full diff, walks through every acceptance criteria, looks for bugs and missing edge cases, checks test quality, then runs `make before_commit`. Produces a verdict (pass, fail, or pass_with_notes) with findings categorized by severity (critical, major, minor, note). Does not modify any code. The verification report is logged to the console, so you can continue the session interactively.

If Step 2's tests fail, Step 3 is skipped to save cost.

### Agent feedback

Each phase can report feedback about the workflow instructions themselves. This is separate from the code review findings -- it covers things like unclear instructions, contradictions between the Jira spec and the design document, missing steps, or approaches that didn't work and had to be changed.

The feedback is collected from all phases and shown at the end of the workflow output. Use it to improve the prompts and instructions for future runs. If a specific instruction keeps getting flagged, it's worth revising in the workflow script.

### What you get

Uncommitted changes in the working tree. No commits are created. You review the diff, organize it into commits following the team's conventions, and create the PR.

A cost estimate is logged at the end of each step and as a total. These are estimates based on output tokens. Run `/usage` for actual costs.

### Constraints

- No git commits are created. All changes stay in the working tree.
- No code is pushed anywhere. The workflow is local-only.
- The verification step is read-only. It reports issues but does not fix them.
- The human engineer is always the final gate before anything is committed or pushed.

The project's `.claude/settings.json` enforces permission rules. `sudo` is always blocked. File operations (Read, Edit, Write) and read-only git commands are always auto-approved. When the sandbox is enabled, all other Bash commands are also auto-approved since the sandbox constrains them (network, filesystem, and credential restrictions). See the file for the full list.
