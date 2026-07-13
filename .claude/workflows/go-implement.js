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
// Schemas
// ---------------------------------------------------------------------------
// Agent return schemas. The workflow runtime validates responses against these
// and retries if they don't match.

// Jira issue description fetched via MCP.
// Example: { issueBody: "Summary: Fix cooldown logic\n\nAs a user, I want..." }
const JIRA_ISSUE_SCHEMA = {
  type: 'object',
  required: ['issueBody'],
  properties: {
    issueBody: { type: 'string', description: 'The full issue summary and description, or empty string if fetch failed. Handled later as a failure state.' },
  },
}

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
// bddPaths can be passed as a single string or an array; normalize to array
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
// Spec context injected into every agent prompt. Combines the Jira issue
// body, design doc path, and BDD feature file paths (last two if provided).
// Each agent sees the same context so they're all working from the same spec.

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

const specContext = specSections.join('\n\n')

// TODO: Sandbox canary check, pre-flight git checks, and Phases 1-3
// (Implement, Unit Tests, Verify) will be added in follow-up commits.
log('Setup complete. issue=' + issueKey + ', specContext built (' + specContext.length + ' chars)')
return {
  issueKey,
  specContext,
}
