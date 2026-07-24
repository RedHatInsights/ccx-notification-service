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
// Collects feedback from all phases, logs it to the workflow progress output,
// and returns a markdown section for displaySummary. Returns empty string if
// no feedback was reported by any phase.
function collectFeedback(phases) {
  const items = Object.entries(phases)
    .flatMap(([phaseName, result]) => {
      const feedback = (result && result.feedback) || []
      return feedback.map(f => ({ phaseName, text: f }))
    })
  if (items.length) {
    log('')
    log('Agent feedback on workflow instructions:')
    items.forEach(f => log('  [' + f.phaseName + '] ' + f.text))
  }
  return items.length
    ? '\n\n### Agent feedback on workflow instructions\n\n' + items.map(f => '- **[' + f.phaseName + ']** ' + f.text).join('\n')
    : ''
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

// Phase 2 result: test files written, pass/fail status, and any failures that couldn't be fixed.
// Example: { testFiles: ["differ/storage_test.go"], testsWritten: 5, testsPassed: true, summary: "Added 5 tests for ReadAggregatorReport", failures: [] }
const UNIT_TEST_SCHEMA = {
  type: 'object',
  required: ['testFiles', 'testsWritten', 'testsPassed', 'summary'],
  properties: {
    testFiles: {
      type: 'array',
      items: { type: 'string' },
      description: 'Test files created or modified',
    },
    testsWritten: {
      type: 'integer',
      description: 'Number of test functions written',
    },
    testsPassed: {
      type: 'boolean',
      description: 'Whether all tests passed on the final run',
    },
    testOutput: {
      type: 'string',
      description: 'Last go test output (abbreviated if long)',
    },
    summary: {
      type: 'string',
      description: 'Brief description of what was tested',
    },
    failures: {
      type: 'array',
      items: { type: 'string' },
      description: 'Descriptions of any test failures that could not be fixed',
    },
    feedback: FEEDBACK_FIELD,
  },
}

// Phase 3 result: independent review verdict, findings by severity, and make before_commit output.
// make before_commit runs: style (shellcheck + abcgo + golangci-lint), unit tests, license headers, and coverage check.
// Example: { verdict: "pass_with_notes", deviations: [{issue: "missing nil check", severity: "minor"}], beforeCommitPassed: true, report: "..." }
const VERIFICATION_SCHEMA = {
  type: 'object',
  required: ['verdict', 'deviations', 'beforeCommitPassed', 'report'],
  properties: {
    verdict: {
      type: 'string',
      enum: ['pass', 'fail', 'pass_with_notes'],
    },
    deviations: {
      type: 'array',
      items: {
        type: 'object',
        required: ['issue', 'severity'],
        properties: {
          issue: { type: 'string' },
          severity: { type: 'string', enum: ['critical', 'major', 'minor', 'note'] },
          detail: { type: 'string' },
        },
      },
    },
    beforeCommitPassed: { type: 'boolean', description: 'Whether make before_commit passed (style + tests + license + coverage)' },
    beforeCommitOutput: {
      type: 'string',
      description: 'Output of make before_commit (abbreviated if long)',
    },
    report: {
      type: 'string',
      description: 'Full verification report text',
    },
    feedback: FEEDBACK_FIELD,
  },
}

// snapshot token count before any agents run, used to calculate per-phase costs
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
- If a function would start with a boolean guard that skips the entire body, place the check at the call site instead. The function should do one thing; the caller decides whether to call it.

## Before finishing, verify

1. If you changed an interface, mocks are regenerated (you ran \`make gen-mocks\`).
2. \`make style\` passes with no errors.
3. \`make license\` was run after all other steps.
4. Every acceptance criteria from the specification has a corresponding code change.
5. No git commits were created. All changes are in the working tree only.

${FEEDBACK_PROMPT}`,
  { label: 'implement', schema: IMPLEMENT_SCHEMA }
)

// Return results early and end the workflow here if implementation phase failed or was skipped.
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

// Return results early and end the workflow here if implementation phase produced no results.
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

// ---------------------------------------------------------------------------
// Phase 2: Unit Tests
// ---------------------------------------------------------------------------
// Writes tests with assertions derived from the spec, but reads the implementation
// for function signatures, types, and mock setup. Can only modify test files,
// export_test.go, and regenerated mocks.

phase('Unit Tests')
log('Phase 2: Writing unit tests...')

const tokensBeforeTests = budget.spent()
const tests = await agent(
  `You are writing unit tests for a Go code change that was just implemented in this repository. The project conventions from AGENTS.md are already in your context.

${specContext}

## How to approach testing

- Derive your **test scenarios and expected values** from the issue specification, acceptance criteria, design document (if provided), and BDD feature files (if provided). Together these define what the code should do.
- You must also read the implementation code to understand function signatures, types, package structure, SQL queries, and mock setup. You need this to write tests that compile and follow the repo's patterns.
- The distinction: assert against spec-defined behavior, but use implementation knowledge for test structure and setup.
- Reference the **Unit tests** section loaded as part of the AGENTS.md which describes how to run the tests.
- If a test correctly asserts spec behavior but the test still fails, that likely means the implementation does not match the spec. In that case, report it as a failure with "implementation may not match spec". Do not modify production code and do not weaken the assertion to make it pass.

## Instructions

1. Run \`git diff ${baseSha}\` to see what was implemented and which files changed (these are uncommitted working tree changes).
2. Read one or two existing test files in the same packages to learn the patterns (assertion libraries, mock setup, test naming, comment style). Match the style you find.
3. If the diff shows a new or changed interface, run \`make gen-mocks\` to ensure mocks are up to date before writing tests.
4. Write unit tests that verify each acceptance criteria from the issue specification. If BDD feature files are provided, use them as additional source of expected behavior. They describe expected user-facing behavior and edge cases that the acceptance criteria may not cover explicitly. Follow the \`export_test.go\` and mock patterns from AGENTS.md.
5. Run \`make style\` to check linting. Fix any issues it reports and re-run until it passes. If you cannot resolve an issue after 3 attempts, stop and describe it in the failures list.
6. Run \`go test\` for the affected packages. Capture the output.
7. If a test fails, you may fix the test code. After 3 edit-and-rerun cycles on the same failing test, stop and add a description of what failed and why to the failures list. Keep the failing test in the file (do not delete it).
8. If some tests pass and others could not be fixed, keep all tests, both passing and failing.
9. Run \`make test\` to run the full test suite.
10. Run \`make coverage\` to test the coverage. You should maintain or improve the coverage, but there might be uncoverable statements, check whether other test cases cover these scenarios or not.
11. Ensure allowed test and mock files have license headers. Do not modify production files; report any missing production-file headers for Phase 1 to address.

## Constraints

- Do not create any git commits. Leave all changes in the working tree.
- Do not run \`git stash\`, \`git checkout\`, \`git reset\`, \`git restore\`, \`git clean\`, or any other command that modifies or reverts tracked files. The implementation code exists only as uncommitted working tree modifications.
- Do not modify production code (non-test files). You may only change test files, regenerated mock files, or \`export_test.go\` for testing unexported functions.
- Do not silently skip or delete a failing test.
- Focus on the packages that appear in the diff. Do not explore unrelated packages.
- Do not explicitly initialize struct fields to their Go zero values (\`false\`, \`nil\`, \`0\`, \`""\`) in test setup. Write \`d := differ.Differ{}\` not \`d := differ.Differ{SomeFlag: false, Storage: nil}\`.
- Do not write tests that are subsets of other tests. If one test already exercises the same code path with the same assertions, a second test covering a subset adds no value.

## Before finishing, verify

1. Every acceptance criteria from the specification has at least one corresponding test.
2. \`make style\` passes with no errors.
3. \`go test\` passes for all affected packages (or every failure is documented in the failures list with a description).
4. \`make coverage\` passes, or explains why it didn't pass (uncoverable statements, non-existing mocks).
5. \`make license\` was run after all other steps.
6. Test assertions check spec-defined expected values, not values copied from the implementation output.
7. No production code was modified.
8. No git commits were created.

${FEEDBACK_PROMPT}`,
  { label: 'unit-tests', schema: UNIT_TEST_SCHEMA }
)

// Calculate cost even if the agent failed (tokens were still spent).
const testsCost = estimateCost(budget.spent() - tokensBeforeTests)

if (!tests) {
  log('Phase 2 failed or was skipped.')
  return {
    error: 'Unit test phase failed',
    displaySummary: `## Workflow stopped

**Error:** Phase 2 (Unit Tests) agent did not return a result. Phase 1 completed: ${filesModified.length} file(s) modified.

**Summary:** ${impl.summary}${collectFeedback({ implement: impl })}

### Estimated costs

| Phase | Cost |
|-------|------|
| Setup | ${formatCost(setupCost)} |
| Implement | ${formatCost(implCost)} |
| Tests | ${formatCost(testsCost)} |
| **Total** | **${formatCost(setupCost + implCost + testsCost)}** |

*Estimates based on output tokens. Run \`/usage\` for actuals.*`,
    implement: { ...impl, costUsd: implCost }, tests: null, verify: null,
    setupCostUsd: setupCost, totalCostUsd: setupCost + implCost + testsCost,
  }
}
log(
  'Phase 2 complete: ' +
    tests.testsWritten +
    ' test(s), passed: ' +
    tests.testsPassed +
    ' | cost~' + formatCost(testsCost)
)
if (tests.testFiles && tests.testFiles.length) {
  log('  test files: ' + tests.testFiles.join(', '))
}

// Gate: skip the verification phase if tests failed return early. Running make before_commit
// on broken code wastes an entire agent's budget discovering known failures.
if (!tests.testsPassed) {
  if (tests.failures && tests.failures.length) {
    tests.failures.forEach(f => log('  failure: ' + f))
  }
  if (tests.testOutput) {
    log('  go test output:')
    log(tests.testOutput)
  }
  log('Tests did not pass. Skipping verification to save cost.')
  const failureList = (tests.failures && tests.failures.length)
    ? '\n\n### Test failures\n\n' + tests.failures.map(f => '- ' + f).join('\n')
    : ''
  return {
    aborted: 'Tests did not pass, verification skipped',
    displaySummary: `## Workflow stopped

**Tests did not pass. Verification was skipped.**

| Phase | Result |
|-------|--------|
| Implement | ${filesModified.length} file(s) |
| Tests | ${tests.testsWritten} test(s), FAILED |

**Summary:** ${impl.summary}${failureList}${collectFeedback({ implement: impl, tests })}

### Estimated costs

| Phase | Cost |
|-------|------|
| Setup | ${formatCost(setupCost)} |
| Implement | ${formatCost(implCost)} |
| Tests | ${formatCost(testsCost)} |
| **Total** | **${formatCost(setupCost + implCost + testsCost)}** |

*Estimates based on output tokens. Run \`/usage\` for actuals.*`,
    implement: { ...impl, costUsd: implCost },
    tests: { ...tests, costUsd: testsCost },
    verify: null,
    setupCostUsd: setupCost, totalCostUsd: setupCost + implCost + testsCost,
  }
}

// ---------------------------------------------------------------------------
// Phase 3: Verify
// ---------------------------------------------------------------------------
// Read-only code review. Does not modify any files. Runs make before_commit
// as an independent check and produces a pass/fail verdict with findings.

phase('Verify')
log('Phase 3: Adversarial verification...')

const tokensBeforeVerify = budget.spent()
const verify = await agent(
  `You are an independent reviewer. Your job is to find problems, not to confirm everything is fine. Be skeptical. Default to flagging concerns rather than assuming correctness. The project conventions from AGENTS.md are already in your context.

${specContext}

## Context

The working tree should contain ${tests.testsWritten} test function(s) that ${tests.testsPassed ? 'passed' : 'failed'} when last run. Verify this independently — do not assume it is accurate.

All changes exist only as **uncommitted working tree modifications** — there is no commit, no stash, no backup. Any git command that modifies tracked files (\`git stash\`, \`git checkout\`, \`git reset\`, \`git restore\`, \`git clean\`) will **permanently destroy** the implementation code with no way to recover it.

${tests.failures && tests.failures.length ? 'The following test failures were already identified before your review. If \`make before_commit\` fails on any of these, treat them as known issues rather than new findings:\n' + tests.failures.map(f => '- ' + f).join('\n') : ''}

## Severity definitions

- **critical**: bug or correctness issue that must be fixed before merge
- **major**: significant concern (missing edge case, weak test, spec gap)
- **minor**: style issue or minor improvement
- **note**: observation, no action required

## Instructions

1. Run \`git diff ${baseSha}\` to see the full diff (these are uncommitted working tree changes, implementation + tests).
2. Walk through each acceptance criteria from the specification, the design document (if provided) and BDD scenarios (if provided). Check whether all the criteria are met by the code AND covered by a test. Flag any unmet criteria.
3. Look for missing edge cases, especially scenarios from the specification, design document, or BDD feature files that the code does not handle.
4. Look for bugs: logic errors, off-by-one mistakes, nil pointer risks, race conditions, resource leaks, or security issues.
5. Check test quality: do the test assertions check spec-defined expected values, or do they just mirror what the implementation returns? Flag assertions that would pass even if the code were broken (e.g., asserting the return value matches whatever the function happens to return, rather than what the spec says it should return). If the spec does not define exact expected values, note this limitation.
6. Run \`make before_commit\` to check style, tests, license headers, and coverage. Report the output and whether it passed or failed (for beforeCommitPassed).
7. If \`make before_commit\` fails, determine whether the failure is pre-existing or caused by this change by using **read-only** methods only: check whether the failing file appears in \`git diff ${baseSha}\`, inspect the error message, or use \`git show ${baseSha}:<file>\` to view the original file content. Never checkout, stash, or revert files to test the baseline. Note the distinction between new and pre-existing failures in your report.

## Constraints

- Do not fix any code. This is a read-only review.
- Do not silently skip a failing check.
- Do not create any git commits.
- Do not run \`git stash\`, \`git checkout\`, \`git reset\`, \`git restore\`, \`git clean\`, or any other command that modifies or reverts tracked files. Use only read-only git commands (\`git diff\`, \`git status\`, \`git log\`, \`git show\`).

## Before finishing, verify

1. You checked every acceptance criteria against the Jira issue, the implementation and the respective tests, as well as the design doc (if provided) and BDD specs (if provided).
2. You ran \`make before_commit\` and reported the full output.
3. Every finding has a severity (critical, major, minor, or note).
4. Your verdict is one of: pass, fail, or pass_with_notes.

${FEEDBACK_PROMPT}`,
  { label: 'verify', schema: VERIFICATION_SCHEMA }
)

// Calculate cost even if the agent failed (tokens were still spent).
const verifyCost = estimateCost(budget.spent() - tokensBeforeVerify)

if (!verify) {
  log('Phase 3 failed or was skipped.')
  const testResult = tests.testsPassed ? 'passed' : 'FAILED'
  return {
    error: 'Verification phase failed',
    displaySummary: `## Workflow stopped

**Error:** Phase 3 (Verify) agent did not return a result. Phases 1 and 2 completed.

| Phase | Result |
|-------|--------|
| Implement | ${filesModified.length} file(s) |
| Tests | ${tests.testsWritten} test(s), ${testResult} |

**Summary:** ${impl.summary}${collectFeedback({ implement: impl, tests })}

### Estimated costs

| Phase | Cost |
|-------|------|
| Setup | ${formatCost(setupCost)} |
| Implement | ${formatCost(implCost)} |
| Tests | ${formatCost(testsCost)} |
| Verify | ${formatCost(verifyCost)} |
| **Total** | **${formatCost(setupCost + implCost + testsCost + verifyCost)}** |

*Estimates based on output tokens. Run \`/usage\` for actuals.*`,
    implement: { ...impl, costUsd: implCost },
    tests: { ...tests, costUsd: testsCost },
    verify: null,
    setupCostUsd: setupCost, totalCostUsd: setupCost + implCost + testsCost + verifyCost,
  }
}

log('Phase 3 complete. Verdict: ' + verify.verdict + ', make before_commit: ' + (verify.beforeCommitPassed ? 'passed' : 'FAILED') + ' | cost~' + formatCost(verifyCost))
if (verify.deviations && verify.deviations.length) {
  verify.deviations.forEach(d => log('  [' + d.severity + '] ' + d.issue))
}

// ---------------------------------------------------------------------------
// Summary
// ---------------------------------------------------------------------------
// The workflow logs a summary and returns structured results to the main
// Claude Code session. The user sees something like:
//
//   ## Workflow results
//
//   **Added aggregator DB connection to the differ package**
//
//   | Phase | Result |
//   |-------|--------|
//   | Implement | 3 file(s) |
//   | Tests | 5 test(s), passed |
//   | Verify | pass_with_notes, before_commit: passed |
//
//   ### Verification report
//
//   <full review with findings by severity>
//
//   ### Estimated costs
//
//   | Phase | Cost |
//   |-------|------|
//   | Setup | $0.0475 |
//   | Implement | $0.4275 |
//   | Tests | $0.6650 |
//   | Verify | $0.5700 |
//   | **Total** | **$1.7100** |

const totalCost = setupCost + implCost + testsCost + verifyCost

log('---')
log('Workflow complete.')
log('')
log('Summary: ' + impl.summary)
log('')
log('  Implement: ' + filesModified.length + ' file(s)')
log('  Tests:     ' + tests.testsWritten + ' test(s), ' + (tests.testsPassed ? 'passed' : 'FAILED'))
log('  Verify:    ' + verify.verdict + ', before_commit: ' + (verify.beforeCommitPassed ? 'passed' : 'FAILED'))
log('')
log('Verification report:')
log(verify.report)

log('')
log('Estimated costs (check /usage for actuals):')
log('  Setup:     ' + formatCost(setupCost))
log('  Implement: ' + formatCost(implCost))
log('  Tests:     ' + formatCost(testsCost))
log('  Verify:    ' + formatCost(verifyCost))
log('  Total:     ' + formatCost(totalCost))

const testResult = tests.testsPassed ? 'passed' : 'FAILED'
const beforeCommitResult = verify.beforeCommitPassed ? 'passed' : 'FAILED'
const feedbackSection = collectFeedback({ implement: impl, tests, verify })

// This is the final return statement that the main Claude session receives back
// if the workflow finishes successfully.
// Claude is instructed to always display the full `displaySummary`, but it also
// returns the full responses from each agent (...impl, ...tests, ...verify),
// so Claude can access all the data it needs after the workflow finishes.
return {
  displaySummary: `## Workflow results

**${impl.summary}**

| Phase | Result |
|-------|--------|
| Implement | ${filesModified.length} file(s) |
| Tests | ${tests.testsWritten} test(s), ${testResult} |
| Verify | ${verify.verdict}, before_commit: ${beforeCommitResult} |

### Verification report

${verify.report}${feedbackSection}

### Estimated costs

| Phase | Cost |
|-------|------|
| Setup | ${formatCost(setupCost)} |
| Implement | ${formatCost(implCost)} |
| Tests | ${formatCost(testsCost)} |
| Verify | ${formatCost(verifyCost)} |
| **Total** | **${formatCost(totalCost)}** |

*Estimates based on output tokens. Run \`/usage\` for actuals.*`,
  implement: { ...impl, costUsd: implCost },
  tests: { ...tests, costUsd: testsCost },
  verify: { ...verify, costUsd: verifyCost },
  setupCostUsd: setupCost,
  totalCostUsd: totalCost,
}
