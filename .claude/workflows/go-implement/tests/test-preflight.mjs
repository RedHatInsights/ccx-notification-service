// Test script for pre-flight failure paths in go-implement.js
//
// Extracts the guard logic from three sections of the workflow:
//   1. Jira fetch
//   2. Sandbox canary
//   3. Preflight checks
//
// Each section is wrapped in a function that accepts mocked agent() and log()
// and returns the same {error} or success shape as the real workflow.

// ---------------------------------------------------------------------------
// Extracted guard functions
// ---------------------------------------------------------------------------

/**
 * Simulates the Jira fetch guard.
 * @param {Function} agentFn - mock for agent(), returns the jiraResult
 * @param {Function} logFn   - mock for log()
 * @returns {{ error: string } | { issueBody: string }}
 */
async function jiraFetchGuard(agentFn, logFn) {
  const issueKey = 'TEST-123'

  logFn('Fetching Jira issue ' + issueKey + '...')
  const jiraResult = await agentFn()

  if (!jiraResult || !jiraResult.issueBody) {
    logFn('Error: Could not fetch Jira issue ' + issueKey + '. Make sure the Jira MCP is connected.')
    return { error: 'Failed to fetch Jira issue ' + issueKey }
  }

  const issue = jiraResult.issueBody
  return { issueBody: issue }
}

/**
 * Simulates the sandbox canary guard.
 * @param {Function} agentFn - mock for agent(), returns the canary result
 * @param {Function} logFn   - mock for log()
 * @returns {{ error: string } | { ok: true }}
 */
async function canaryGuard(agentFn, logFn) {
  logFn('Checking sandbox enforcement...')
  const canary = await agentFn()

  if (!canary) {
    logFn('ERROR: Sandbox canary agent failed to run.')
    return { error: 'Sandbox canary check failed - the agent did not return a result' }
  }

  if (!canary.blocked) {
    logFn('ERROR: Sandbox is not active! This workflow requires sandbox enforcement.')
    logFn('Enable it in .claude/settings.json: set "sandbox.enabled" to true, then restart Claude Code.')
    return { error: 'Sandbox not enabled. Set sandbox.enabled=true in .claude/settings.json and restart Claude Code.' }
  }

  logFn('Sandbox confirmed: network access to non-allowed domains is blocked.')
  return { ok: true }
}

/**
 * Simulates the preflight checks guard.
 * @param {Function} agentFn - mock for agent(), returns the preflight result
 * @param {Function} logFn   - mock for log()
 * @returns {{ error: string } | { branch: string, baseSha: string }}
 */
async function preflightGuard(agentFn, logFn) {
  const preflight = await agentFn()

  if (!preflight) {
    logFn('Pre-flight check failed or was skipped.')
    return { error: 'Pre-flight check failed' }
  }

  if (['main', 'master'].includes(preflight.branch)) {
    logFn('Error: Refusing to run on protected branch "' + preflight.branch + '". Create a feature branch first.')
    return { error: 'Cannot run on ' + preflight.branch + '. Create a feature branch first' }
  }

  if (!preflight.cleanTree) {
    logFn('Error: Working tree is not clean. Commit or stash changes before running this workflow.')
    logFn('  Dirty files: ' + (preflight.dirtyFiles || '(unknown)'))
    return { error: 'Working tree is not clean' }
  }

  const baseSha = preflight.headSha
  logFn('Setup OK: branch=' + preflight.branch + ' baseSha=' + baseSha.substring(0, 8))
  return { branch: preflight.branch, baseSha }
}

// ---------------------------------------------------------------------------
// Test harness
// ---------------------------------------------------------------------------

let passed = 0
let failed = 0

function createLog() {
  const messages = []
  const fn = (msg) => messages.push(msg)
  fn.messages = messages
  return fn
}

async function runTest(testNum, description, testFn) {
  const log = createLog()
  try {
    const { pass, detail } = await testFn(log)
    if (pass) {
      passed++
      console.log(`[PASS] Test ${testNum}: ${description}`)
      if (detail) console.log(`       ${detail}`)
    } else {
      failed++
      console.log(`[FAIL] Test ${testNum}: ${description}`)
      console.log(`       ${detail}`)
      if (log.messages.length) {
        console.log(`       Logged: ${log.messages.join(' | ')}`)
      }
    }
  } catch (err) {
    failed++
    console.log(`[FAIL] Test ${testNum}: ${description}`)
    console.log(`       Exception: ${err.message}`)
  }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

console.log('=== Testing go-implement.js pre-flight failure paths ===')
console.log('')
console.log('--- Jira Fetch Guard ---')

// Test 1: null result from agent
await runTest(1, 'Jira: null agent result returns error', async (log) => {
  const result = await jiraFetchGuard(() => null, log)
  const hasError = !!result.error
  const correctMsg = result.error && result.error.includes('Failed to fetch Jira issue')
  const loggedError = log.messages.some(m => m.includes('Could not fetch Jira issue'))
  return {
    pass: hasError && correctMsg && loggedError,
    detail: hasError
      ? `error="${result.error}", logged=${loggedError}`
      : 'Expected error but got: ' + JSON.stringify(result),
  }
})

// Test 2: empty issueBody
await runTest(2, 'Jira: empty issueBody returns error', async (log) => {
  const result = await jiraFetchGuard(() => ({ issueBody: '' }), log)
  const hasError = !!result.error
  const correctMsg = result.error && result.error.includes('Failed to fetch Jira issue')
  return {
    pass: hasError && correctMsg,
    detail: hasError
      ? `error="${result.error}"`
      : 'Expected error but got: ' + JSON.stringify(result),
  }
})

// Test 3: valid issueBody
await runTest(3, 'Jira: valid issueBody returns success', async (log) => {
  const body = 'Summary: Fix the widget\n\nDetailed description here.'
  const result = await jiraFetchGuard(() => ({ issueBody: body }), log)
  const isSuccess = !result.error && result.issueBody === body
  return {
    pass: isSuccess,
    detail: isSuccess
      ? `issueBody="${result.issueBody.substring(0, 40)}..."`
      : 'Expected success with issueBody but got: ' + JSON.stringify(result),
  }
})

console.log('')
console.log('--- Sandbox Canary Guard ---')

// Test 4: null result from agent
await runTest(4, 'Canary: null agent result returns error', async (log) => {
  const result = await canaryGuard(() => null, log)
  const hasError = !!result.error
  const correctMsg = result.error && result.error.includes('canary check failed')
  const loggedError = log.messages.some(m => m.includes('canary agent failed to run'))
  return {
    pass: hasError && correctMsg && loggedError,
    detail: hasError
      ? `error="${result.error}", logged=${loggedError}`
      : 'Expected error but got: ' + JSON.stringify(result),
  }
})

// Test 5: blocked=false (sandbox not active)
await runTest(5, 'Canary: blocked=false returns sandbox-not-active error', async (log) => {
  const result = await canaryGuard(() => ({ curlExitCode: 0, httpStatus: '200', blocked: false }), log)
  const hasError = !!result.error
  const correctMsg = result.error && result.error.includes('Sandbox not enabled')
  const loggedError = log.messages.some(m => m.includes('Sandbox is not active'))
  return {
    pass: hasError && correctMsg && loggedError,
    detail: hasError
      ? `error="${result.error}", logged=${loggedError}`
      : 'Expected error but got: ' + JSON.stringify(result),
  }
})

// Test 6: blocked=true (sandbox is active)
await runTest(6, 'Canary: blocked=true returns success', async (log) => {
  const result = await canaryGuard(() => ({ curlExitCode: 7, blocked: true }), log)
  const isSuccess = !result.error && result.ok === true
  const loggedOk = log.messages.some(m => m.includes('Sandbox confirmed'))
  return {
    pass: isSuccess && loggedOk,
    detail: isSuccess
      ? `ok=${result.ok}, logged=${loggedOk}`
      : 'Expected success but got: ' + JSON.stringify(result),
  }
})

console.log('')
console.log('--- Preflight Guard ---')

// Test 7: null result from agent
await runTest(7, 'Preflight: null agent result returns error', async (log) => {
  const result = await preflightGuard(() => null, log)
  const hasError = !!result.error
  const correctMsg = result.error && result.error.includes('Pre-flight check failed')
  const loggedError = log.messages.some(m => m.includes('Pre-flight check failed'))
  return {
    pass: hasError && correctMsg && loggedError,
    detail: hasError
      ? `error="${result.error}", logged=${loggedError}`
      : 'Expected error but got: ' + JSON.stringify(result),
  }
})

// Test 8: branch=main
await runTest(8, 'Preflight: branch=main returns protected branch error', async (log) => {
  const result = await preflightGuard(() => ({
    branch: 'main',
    cleanTree: true,
    headSha: 'a'.repeat(40),
  }), log)
  const hasError = !!result.error
  const correctMsg = result.error && result.error.includes('Cannot run on main')
  const loggedError = log.messages.some(m => m.includes('Refusing to run on protected branch'))
  return {
    pass: hasError && correctMsg && loggedError,
    detail: hasError
      ? `error="${result.error}", logged=${loggedError}`
      : 'Expected error but got: ' + JSON.stringify(result),
  }
})

// Test 9: branch=master
await runTest(9, 'Preflight: branch=master returns protected branch error', async (log) => {
  const result = await preflightGuard(() => ({
    branch: 'master',
    cleanTree: true,
    headSha: 'b'.repeat(40),
  }), log)
  const hasError = !!result.error
  const correctMsg = result.error && result.error.includes('Cannot run on master')
  const loggedError = log.messages.some(m => m.includes('Refusing to run on protected branch "master"'))
  return {
    pass: hasError && correctMsg && loggedError,
    detail: hasError
      ? `error="${result.error}", logged=${loggedError}`
      : 'Expected error but got: ' + JSON.stringify(result),
  }
})

// Test 10: dirty tree with dirtyFiles
await runTest(10, 'Preflight: dirty tree with dirtyFiles lists them', async (log) => {
  const dirtyFiles = 'M  differ/differ.go\n?? scratch.txt'
  const result = await preflightGuard(() => ({
    branch: 'feature/test',
    cleanTree: false,
    headSha: 'c'.repeat(40),
    dirtyFiles,
  }), log)
  const hasError = !!result.error
  const correctMsg = result.error && result.error.includes('Working tree is not clean')
  const loggedDirty = log.messages.some(m => m.includes('differ/differ.go'))
  return {
    pass: hasError && correctMsg && loggedDirty,
    detail: hasError
      ? `error="${result.error}", dirtyFilesLogged=${loggedDirty}`
      : 'Expected error but got: ' + JSON.stringify(result),
  }
})

// Test 11: dirty tree without dirtyFiles (falls back to "(unknown)")
await runTest(11, 'Preflight: dirty tree without dirtyFiles shows (unknown)', async (log) => {
  const result = await preflightGuard(() => ({
    branch: 'feature/test',
    cleanTree: false,
    headSha: 'd'.repeat(40),
    // no dirtyFiles field
  }), log)
  const hasError = !!result.error
  const correctMsg = result.error && result.error.includes('Working tree is not clean')
  const loggedUnknown = log.messages.some(m => m.includes('(unknown)'))
  return {
    pass: hasError && correctMsg && loggedUnknown,
    detail: hasError
      ? `error="${result.error}", unknownLogged=${loggedUnknown}`
      : 'Expected error but got: ' + JSON.stringify(result),
  }
})

// Test 12: clean feature branch (success path)
await runTest(12, 'Preflight: clean feature branch returns success', async (log) => {
  const sha = 'abcdef01'.repeat(5)  // 40 chars
  const result = await preflightGuard(() => ({
    branch: 'feature/CCXDEV-12345',
    cleanTree: true,
    headSha: sha,
  }), log)
  const isSuccess = !result.error && result.branch === 'feature/CCXDEV-12345' && result.baseSha === sha
  const loggedOk = log.messages.some(m => m.includes('Setup OK') && m.includes('feature/CCXDEV-12345'))
  return {
    pass: isSuccess && loggedOk,
    detail: isSuccess
      ? `branch="${result.branch}", baseSha="${result.baseSha.substring(0, 8)}...", logged=${loggedOk}`
      : 'Expected success but got: ' + JSON.stringify(result),
  }
})

// ---------------------------------------------------------------------------
// Summary
// ---------------------------------------------------------------------------

console.log('')
console.log('=== Results ===')
console.log(`Total: ${passed + failed} | Passed: ${passed} | Failed: ${failed}`)

if (failed > 0) {
  process.exit(1)
}
