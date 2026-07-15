// go-implement - Three-phase agentic workflow for Go implementation issues
//
// Args (passed as JSON object):
//   issue       (required) - Jira issue key (e.g., "CCXDEV-12345")
//   designDoc   (optional) - Path to a design document
//   bddPaths    (optional) - Array of local paths to BDD feature files
//
// Examples:
//
//   /go-implement { "issue": "CCXDEV-12345" }
//
//   /go-implement { "issue": "CCXDEV-12345", "designDoc": "/path/to/design-doc.md" }
//
//   /go-implement { "issue": "CCXDEV-12345", "bddPaths": ["/path/to/example.feature"] }
//
//   /go-implement { "issue": "CCXDEV-12345", "designDoc": "/path/to/design-doc.md", "bddPaths": ["/path/to/example.feature", "/path/to/example_2.feature"] }
//
// Prerequisites:
//   - Sandbox enabled in .claude/settings.json (the workflow refuses to run without it)
//   - Jira MCP connected to your Claude Code session (required to fetch the Jira issue)
//   - Feature branch (the workflow refuses to run on main/master)
//   - Clean working tree (no uncommitted or staged changes, untracked files are fine)
//   - `pre-commit` installed (`pre-commit install`)
//   - `make before_commit` passes on a clean branch
//
// Phases:
//   1. Implement  - writes production code, runs make style
//   2. Unit Tests - writes tests from the spec and diff, runs make style + go test + make coverage
//   3. Verify     - independent review, runs make before_commit
//
// What it produces:
//   Uncommitted changes in the working tree (production code + unit tests) and a
//   verification report. The human engineer reviews the diff, organizes commits,
//   and creates the PR.
//
// Cost tracking:
//   Costs are estimated from output token deltas (budget.spent()) using the
//   pricing constants below. Input tokens ARE NOT tracked by the workflow
//   runtime, so they are estimated at INPUT_RATIO * output. These are
//   estimates, not billing data. Check /usage for actuals after the run.

export const meta = {
  name: 'go-implement',
  description: 'Implement a Go change in three phases: code, tests, verify. Pass args as JSON: { "issue": "CCXDEV-12345", "designDoc": "/path/to/design-doc.md", "bddPaths": ["/path/to/example.feature"] }. Only "issue" (a Jira key) is required. Including a design document and BDD specs will improve the output.',
  phases: [
    { title: 'Implement', detail: 'Write production code from the issue specification' },
    { title: 'Unit Tests', detail: 'Write tests with assertions from the spec, using implementation for structure' },
    { title: 'Verify', detail: 'Adversarial review of the full diff against the specification' },
  ],
}
// ---------------------------------------------------------------------------
// Cost estimation
// ---------------------------------------------------------------------------
// The agents don't have access to /usage or any other actual cost data.
//
// The workflow tracks cost per phase using budget.spent(), which only returns
// output tokens. The total is estimated by assuming input tokens are a multiple
// of output tokens (INPUT_RATIO).
//
// Why the need for an INPUT_RATIO:
// Claude Code agents read a lot of context (AGENTS.md, source files, diffs)
// but produce relatively little output. The 50:1 ratio was calibrated from
// /usage data across a few real issues (Opus 4 on Vertex AI). Cache reads
// dominate input costs. If estimates drift from /usage actuals, re-calibrate
// by comparing estimated vs actual costs.
//
// Pricing (as of 2026-07, Claude Opus 4 on Vertex AI):
// $5/MTok input, $25/MTok output (same as the Anthropic API).
const USD_PER_OUTPUT_MTOK = 25
const USD_PER_INPUT_MTOK = 5
const INPUT_RATIO = 50

// Estimate the dollar cost for a given number of output tokens.
// Input tokens are not tracked by the runtime, so they are estimated
// as OUTPUT_TOKENS * INPUT_RATIO (see the comment block above).
function estimateCost(outputTokens) {
  const estimatedInputTokens = outputTokens * INPUT_RATIO
  const usd =
    (outputTokens * USD_PER_OUTPUT_MTOK + estimatedInputTokens * USD_PER_INPUT_MTOK) / 1_000_000
  return usd
}

// Format a dollar amount for display, e.g. 0.095 -> "$0.0950"
function formatCost(usd) {
  return '$' + usd.toFixed(4)
}

// ---------------------------------------------------------------------------
// Agent feedback
// ---------------------------------------------------------------------------
// Collect and log feedback from all phases that returned results.
// Each phase is instructed to give a `feedback` array with agent commentary
// on the workflow / spec instructions themselves.
function logFeedback(phases) {
  // Object.entries converts { implement: ..., tests: ..., verify: ... } into
  // an array of [name, result] pairs. flatMap collects feedback from each phase
  // into a single list, prefixed with the phase name.
  const feedbackLines = Object.entries(phases)
    .flatMap(([phaseName, result]) => {
      const items = (result && result.feedback) || []
      return items.map(f => `[${phaseName}] ${f}`)
    })
  if (feedbackLines.length) {
    log('')
    log('Agent feedback on workflow instructions:')
    feedbackLines.forEach(f => log('  ' + f))
  }
}

// ---------------------------------------------------------------------------
// Schemas
// ---------------------------------------------------------------------------
// Each agent is required to return structured JSON matching these schemas.
// The workflow runtime validates the output and retries if it doesn't match.
// This gives typed results (filesModified, testsPassed, verdict) instead
// of free-text responses.

const FEEDBACK_FIELD = {
  type: 'array',
  items: { type: 'string' },
  description: 'Feedback about the instructions you were given - not about the code. Report if any instruction was unclear, missing a step, wrong, or if you had to take a different approach than what was described and why.',
}

// Jira issue description fetched via MCP.
// Example: { issueBody: "Summary: Fix cooldown logic\n\nAs a user, I want..." }
const JIRA_ISSUE_SCHEMA = {
  type: 'object',
  required: ['issueBody'],
  properties: {
    issueBody: { type: 'string', description: 'The full issue summary and description, or empty string if fetch failed. Handled later as a failure state.' },
  },
}

// Sandbox enforcement check: did curl to an unlisted domain get blocked?
// Example (sandbox active):  { curlExitCode: 7, blocked: true }
// Example (sandbox missing): { curlExitCode: 0, blocked: false, httpStatus: "200" }
const SANDBOX_CANARY_SCHEMA = {
  type: 'object',
  required: ['curlExitCode', 'blocked'],
  properties: {
    curlExitCode: { type: 'integer', description: 'Exit code of the curl command' },
    httpStatus: { type: 'string', description: 'HTTP status code returned by curl, if any' },
    blocked: { type: 'boolean', description: 'True if curl failed to connect (exit code != 0 or connection refused/timeout)' },
  },
}

// Git state check before the workflow starts.
// Example: { branch: "feature-aggregator-db", cleanTree: true, headSha: "a1b2c3d4..." }
const GIT_PREFLIGHT_SCHEMA = {
  type: 'object',
  required: ['branch', 'cleanTree', 'headSha'],
  properties: {
    branch: { type: 'string', description: 'Current branch name' },
    cleanTree: { type: 'boolean', description: 'True if git status --porcelain produced no output' },
    headSha: { type: 'string', description: 'Full 40-char HEAD SHA' },
    dirtyFiles: { type: 'string', description: 'Output of git status --porcelain if not clean' },
  },
}

// Phase 1 result: what files were changed and any concerns.
// Example: { filesModified: ["differ/storage.go", "differ/differ.go"], summary: "Added aggregator DB connection", warnings: [] }
const IMPLEMENT_SCHEMA = {
  type: 'object',
  required: ['filesModified', 'summary'],
  properties: {
    filesModified: {
      type: 'array',
      items: { type: 'string' },
      description: 'List of files created or modified',
    },
    summary: {
      type: 'string',
      description: 'Brief description of what was implemented',
    },
    warnings: {
      type: 'array',
      items: { type: 'string' },
      description: 'Ambiguities or concerns encountered during implementation',
    },
    feedback: FEEDBACK_FIELD,
  },
}

const tokensBeforeSetup = budget.spent()

// ---------------------------------------------------------------------------
// Args
// ---------------------------------------------------------------------------
// The workflow runtime passes args as either a JSON string or a parsed object,
// depending on how the user invoked it. Both forms are handled.

let parsed
try {
  parsed = typeof args === 'string' ? JSON.parse(args) : args
} catch (e) {
  log('Error: args is not valid JSON: ' + e.message)
  return { error: 'Invalid JSON args: ' + e.message }
}

if (!parsed || !parsed.issue) {
  log('Error: args.issue is required. Pass a Jira issue key (e.g., { "issue": "CCXDEV-12345" }).')
  return { error: 'args.issue is required - pass a Jira issue key like "CCXDEV-12345"' }
}

const issueKey = parsed.issue
const designDocPath = parsed.designDoc || ''
const bddPathsRaw = parsed.bddPaths || []
const bddPaths = Array.isArray(bddPathsRaw) ? bddPathsRaw : [bddPathsRaw]

// ---------------------------------------------------------------------------
// Fetch Jira issue
// ---------------------------------------------------------------------------
// The workflow requires the Jira MCP to be connected. The agent fetches the
// issue body using the getJiraIssue MCP tool.

log('Fetching Jira issue ' + issueKey + '...')
const jiraResult = await agent(
  `Fetch the Jira issue ${issueKey} using the getJiraIssue MCP tool. Return the issue summary and the full description body as a single string. Format it as:

Summary: <summary>

<description body>

If the MCP tool is not available or the issue cannot be found, return an empty string for issueBody.`,
  {
    label: 'fetch-jira',
    effort: 'low',
    schema: JIRA_ISSUE_SCHEMA,
  }
)

if (!jiraResult || !jiraResult.issueBody) {
  log('Error: Could not fetch Jira issue ' + issueKey + '. Make sure the Jira MCP is connected.')
  return { error: 'Failed to fetch Jira issue ' + issueKey }
}

const issue = jiraResult.issueBody

// ---------------------------------------------------------------------------
// Shared prompt fragments
// ---------------------------------------------------------------------------
// These are injected into every agent prompt via template literals.
//   - specContext combines all the provided specifications and gives each agent the same context to work from.
//   - FEEDBACK_PROMPT tells each agent how to use the feedback output field.

// The Jira issue body is always present (fetched above).
const specSections = [
  `## Issue Specification (${issueKey})

${issue}`,
]

// Design doc is optional. Points the agent to a file path to read.
if (designDocPath) {
  specSections.push(`## Design Document

Read the design document at \`${designDocPath}\` for architecture and implementation
guidance. Use it as the primary guide for approach and structure. Reference it liberally.`)
}

// BDD feature files are optional. Listed as paths for the agent to read.
if (bddPaths.length) {
  specSections.push(`## BDD Scenarios

These feature files describe expected user-facing behavior:
${bddPaths.map(p => '- \`' + p + '\`').join('\n')}`)
}

// Join all possible sections of the spec into the main specification prompt injected into every agent.
const specContext = specSections.join('\n\n')

// FEEDBACK_PROMPT tells each agent to report issues with the workflow instructions, spec
// contradictions, design doc gaps, or unclear BDD scenarios. Logged at the
// end of the run for a human to review.
//
// In theory, this creates a feedback loop where agents help improve the
// prompts that control them. What could possibly go wrong.
const FEEDBACK_PROMPT = `## Workflow feedback

Your structured output includes a \`feedback\` array of strings. Add one entry per issue you encountered with these instructions - not about the code. For example: an instruction that was unclear or ambiguous, a step that was missing, a prescribed approach that did not work, the Jira spec contradicting the design document or BDD scenarios (if provided), or anything else that would help improve these instructions. Leave it empty if everything worked as described.`

// ---------------------------------------------------------------------------
// Sandbox canary check
// ---------------------------------------------------------------------------
// Try to reach a domain that is NOT in the sandbox allowedDomains list.
// If the request succeeds, the sandbox is not active and the workflow exits.

log('Checking sandbox enforcement...')
const canary = await agent(
  `Run this exact command with no modifications - no leading whitespace, no extra flags, no wrapping with echo or other commands:

curl -s -o /dev/null -w "%{http_code}" --connect-timeout 3 --max-time 5 https://google.com

Copy-paste the command exactly as shown above. Report the exit code and the HTTP status code. If curl fails with a connection error or timeout, that counts as blocked.`,
  {
    label: 'sandbox-canary',
    effort: 'low',
    schema: SANDBOX_CANARY_SCHEMA,
  }
)

if (!canary) {
  log('ERROR: Sandbox canary agent failed to run.')
  return { error: 'Sandbox canary check failed - the agent did not return a result' }
}

if (!canary.blocked) {
  log('ERROR: Sandbox is not active! This workflow requires sandbox enforcement.')
  log('Enable it in .claude/settings.json: set "sandbox.enabled" to true, then restart Claude Code.')
  return { error: 'Sandbox not enabled. Set sandbox.enabled=true in .claude/settings.json and restart Claude Code.' }
}

log('Sandbox confirmed: network access to non-allowed domains is blocked.')

// ---------------------------------------------------------------------------
// Pre-flight checks
// ---------------------------------------------------------------------------
// A lightweight agent checks the git state. baseSha is captured here rather
// than trusting the implementation agent to report it, because all subsequent
// git diffs depend on this being correct.

const preflight = await agent(
  `Run these checks and report the results. Do not fix anything, just report.

1. Run \`git rev-parse --abbrev-ref HEAD\` and report the branch name.
2. Run \`git status --porcelain\` and report whether the working tree is clean (empty output = clean).
3. Run \`git rev-parse HEAD\` and report the full 40-character SHA.

Return all three values.`,
  {
    label: 'preflight',
    effort: 'low',
    schema: GIT_PREFLIGHT_SCHEMA,
  }
)

if (!preflight) {
  log('Pre-flight check failed or was skipped.')
  return { error: 'Pre-flight check failed' }
}

if (['main', 'master'].includes(preflight.branch)) {
  log('Error: Refusing to run on protected branch "' + preflight.branch + '". Create a feature branch first.')
  return { error: 'Cannot run on ' + preflight.branch + '. Create a feature branch first' }
}

if (!preflight.cleanTree) {
  log('Warning: Working tree is not clean')
  log('  Dirty files: ' + (preflight.dirtyFiles || '(unknown)'))
}

// Store the starting SHA. All git diffs in later phases use this as the base.
const baseSha = preflight.headSha
const setupCost = estimateCost(budget.spent() - tokensBeforeSetup)
log('Setup OK: branch=' + preflight.branch + ' baseSha=' + baseSha.substring(0, 8) + ' | cost~' + formatCost(setupCost))

// ---------------------------------------------------------------------------
// Phase 1: Implement
// ---------------------------------------------------------------------------
// Writes the production code. This is the only phase that modifies non-test
// files, so it also runs make style and fixes any issues it finds.
// budget.spent() deltas track cost per phase (see Cost estimation above).

phase('Implement')
log('Phase 1: Writing production code...')

const tokensBeforeImpl = budget.spent()
const impl = await agent(
  `You are implementing a Go code change in this repository. The project conventions from AGENTS.md are already in your context.

${specContext}

## Scope

- You may make additional changes beyond the steps below if they are needed to implement the specification correctly (e.g., small refactors, extracting helpers).
- You must run through the full checklist at the end regardless of what you changed.

## Instructions

1. Read the files you are going to modify in full before writing anything. Also read any files referenced in the issue specification. You may explore other files if you have a specific reason (understanding a type, checking an interface, reading a dependency), but stay focused on the task.
2. Implement the production code changes described in the acceptance criteria. If a design document is provided above, use it as the primary guide for architecture and approach. Follow the existing code style and patterns in the repo.
3. If you added new imports or dependencies, run \`go mod tidy\`.
4. If you added or modified any interface, run \`make gen-mocks\` (see AGENTS.md for details).
5. Run \`make style\` to check linting. Fix any issues it reports and re-run \`make style\` until it passes. If you cannot resolve an issue after 3 attempts, stop and describe the issue and the \`make style\` error output in warnings.
6. Run \`make license\` to add license headers to any new \`.go\` files. This must be last because earlier steps (gen-mocks, style fixes) may create new files.

## Constraints

- Do not write unit tests. They will be written separately.
- Do not run \`make before_commit\`, \`make test\`, or \`make coverage\`.
- Do not create any git commits. Leave all changes in the working tree.
- If the specification is ambiguous on any point, note it in warnings rather than guessing.
- If a dependency from another issue is not yet merged or available, stop and include an explanation in warnings. Report whatever files you did manage to create in filesModified.

## Before finishing, verify

1. If you changed an interface, mocks are regenerated (you ran \`make gen-mocks\`).
2. \`make style\` passes with no errors.
3. \`make license\` was run after all other steps.
4. Every acceptance criteria from the specification has a corresponding code change.
5. No git commits were created. All changes are in the working tree only.

${FEEDBACK_PROMPT}`,
  { label: 'implement', schema: IMPLEMENT_SCHEMA }
)

if (!impl) {
  log('Phase 1 failed or was skipped.')
  return {
    error: 'Implementation phase failed',
    displaySummary: '## Workflow stopped\n\n**Error:** Implementation phase failed.\n\nThe implement agent did not return a result. Check the workflow logs for details.',
    implement: null, tests: null, verify: null,
    setupCostUsd: setupCost, totalCostUsd: setupCost,
  }
}

const implCost = estimateCost(budget.spent() - tokensBeforeImpl)
const filesModified = impl.filesModified || []
log(
  'Phase 1 complete: ' +
    filesModified.length +
    ' file(s) modified: ' +
    filesModified.join(', ') +
    ' | cost~' + formatCost(implCost)
)
if (impl.warnings && impl.warnings.length) {
  impl.warnings.forEach(w => log('  warning: ' + w))
}

// Gate: termination check to prevent the following phases from running
if (!filesModified.length) {
  log('Phase 1 produced no file changes. Nothing to test or verify.')
  return {
    error: 'No files modified',
    displaySummary: `## Workflow stopped

**Error:** Phase 1 (Implement) completed but produced no file changes.

**Summary:** ${impl.summary}${impl.warnings && impl.warnings.length ? '\n\n**Warnings:**\n' + impl.warnings.map(w => '- ' + w).join('\n') : ''}

### Estimated costs

| Phase | Cost |
|-------|------|
| Setup | ${formatCost(setupCost)} |
| Implement | ${formatCost(implCost)} |
| **Total** | **${formatCost(setupCost + implCost)}** |

*Estimates based on output tokens. Run \`/usage\` for actuals.*`,
    implement: { ...impl, costUsd: implCost }, tests: null, verify: null,
    setupCostUsd: setupCost, totalCostUsd: setupCost + implCost,
  }
}

// TODO: Phase 2 (Unit Tests) and Phase 3 (Verify) will be added in follow-up commits.
log('Phase 1 complete. Phases 2 and 3 not yet implemented.')
logFeedback({ implement: impl })

return {
  displaySummary: `## Workflow results (Phase 1 only)

**${impl.summary}**

| Phase | Result |
|-------|--------|
| Implement | ${filesModified.length} file(s) |

### Estimated costs

| Phase | Cost |
|-------|------|
| Setup | ${formatCost(setupCost)} |
| Implement | ${formatCost(implCost)} |
| **Total** | **${formatCost(setupCost + implCost)}** |

*Estimates based on output tokens. Run \`/usage\` for actuals.*`,
  implement: { ...impl, costUsd: implCost },
  tests: null,
  verify: null,
  setupCostUsd: setupCost,
  totalCostUsd: setupCost + implCost,
}
