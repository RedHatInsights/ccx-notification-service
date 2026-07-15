// Test script for go-implement.js Phase 1 and Phase 2 gate logic.
//
// Simulates Phase 1 and Phase 2 gate decisions with 8 test cases.
// Cost is now calculated before the null gate (tokens are spent even
// if the agent fails), and collectFeedback is included in displaySummary.

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------
const logs = []
function log(msg) { logs.push(msg) }

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
    ? '\n\n### Agent feedback\n\n' + items.map(f => '- **[' + f.phaseName + ']** ' + f.text).join('\n')
    : ''
}

// ---------------------------------------------------------------------------
// Phase 1 gate logic
// ---------------------------------------------------------------------------
// Cost is calculated before the null gate (the agent may have spent tokens).
function phase1Gate(impl, setupCost, implCost) {
  if (!impl) {
    return {
      error: 'Implementation phase failed',
      implement: null, tests: null, verify: null,
      setupCostUsd: setupCost, totalCostUsd: setupCost,
    }
  }

  const filesModified = impl.filesModified || []

  if (!filesModified.length) {
    return {
      error: 'No files modified',
      implement: { ...impl, costUsd: implCost }, tests: null, verify: null,
      setupCostUsd: setupCost, totalCostUsd: setupCost + implCost,
    }
  }

  return { passed: true, implCost }
}

// ---------------------------------------------------------------------------
// Phase 2 gate logic
// ---------------------------------------------------------------------------
// Cost is calculated before the null gate.
function phase2Gate(tests, impl, implCost, testsCost, setupCost) {
  if (!tests) {
    return {
      error: 'Unit test phase failed',
      implement: { ...impl, costUsd: implCost }, tests: null, verify: null,
      setupCostUsd: setupCost, totalCostUsd: setupCost + implCost + testsCost,
    }
  }

  if (!tests.testsPassed) {
    if (tests.failures && tests.failures.length) {
      // would log failures here
    }
    return {
      implement: { ...impl, costUsd: implCost },
      tests: { ...tests, costUsd: testsCost },
      verify: null,
      aborted: 'Tests did not pass, verification skipped',
      setupCostUsd: setupCost,
      totalCostUsd: setupCost + implCost + testsCost,
    }
  }

  return { passed: true, testsCost }
}

// ---------------------------------------------------------------------------
// Test harness
// ---------------------------------------------------------------------------
let passed = 0
let failed = 0

function assert(condition, label) {
  logs.length = 0
  if (condition) {
    console.log(`[PASS] ${label}`)
    passed++
  } else {
    console.log(`[FAIL] ${label}`)
    failed++
  }
}

function has(obj, key) {
  return Object.prototype.hasOwnProperty.call(obj, key)
}

const SETUP_COST = 0.05
const IMPL_COST = 0.12
const TESTS_COST = 0.08

// ---- Phase 1 tests ----

console.log('\n=== Phase 1 Gate Tests ===\n')

// Test 1: impl=null
{
  const r = phase1Gate(null, SETUP_COST, IMPL_COST)
  assert(
    r.error === 'Implementation phase failed' &&
    r.implement === null &&
    r.tests === null &&
    r.verify === null &&
    r.setupCostUsd === SETUP_COST &&
    r.totalCostUsd === SETUP_COST,
    'Test 1: impl=null returns error with null phases, totalCostUsd=setupCost'
  )
}

// Test 2: filesModified=undefined
{
  const impl = { summary: 'did stuff' }
  const r = phase1Gate(impl, SETUP_COST, IMPL_COST)
  assert(
    r.error === 'No files modified' &&
    r.implement !== null &&
    has(r.implement, 'costUsd') && r.implement.costUsd === IMPL_COST &&
    r.tests === null &&
    r.verify === null &&
    r.totalCostUsd === SETUP_COST + IMPL_COST,
    'Test 2: filesModified=undefined triggers no-files gate, impl spread with costUsd'
  )
}

// Test 3: filesModified=[]
{
  const impl = { filesModified: [], summary: 'nothing changed' }
  const r = phase1Gate(impl, SETUP_COST, IMPL_COST)
  assert(
    r.error === 'No files modified' &&
    r.implement.filesModified.length === 0 &&
    r.totalCostUsd === SETUP_COST + IMPL_COST,
    'Test 3: filesModified=[] triggers no-files gate'
  )
}

// Test 4: filesModified=["file.go"] with warnings
{
  const impl = {
    filesModified: ['file.go'],
    warnings: ['something looked odd'],
    summary: 'implemented feature',
  }
  const r = phase1Gate(impl, SETUP_COST, IMPL_COST)
  assert(
    r.passed === true &&
    !has(r, 'error'),
    'Test 4: filesModified=["file.go"] with warnings passes gate'
  )
}

// ---- Phase 2 tests ----

console.log('\n=== Phase 2 Gate Tests ===\n')

const implForP2 = {
  filesModified: ['differ/storage.go'],
  summary: 'added new query',
}

// Test 5: tests=null (cost still included)
{
  const r = phase2Gate(null, implForP2, IMPL_COST, TESTS_COST, SETUP_COST)
  assert(
    r.error === 'Unit test phase failed' &&
    r.implement.costUsd === IMPL_COST &&
    r.tests === null &&
    r.verify === null &&
    r.totalCostUsd === SETUP_COST + IMPL_COST + TESTS_COST,
    'Test 5: tests=null includes testsCost in total (tokens spent even on failure)'
  )
}

// Test 6: testsPassed=false with failures
{
  const tests = {
    testsPassed: false,
    testsWritten: 3,
    failures: ['TestFoo: expected 1 got 2'],
  }
  const r = phase2Gate(tests, implForP2, IMPL_COST, TESTS_COST, SETUP_COST)
  assert(
    !has(r, 'error') &&
    has(r, 'aborted') &&
    r.aborted === 'Tests did not pass, verification skipped' &&
    r.tests.costUsd === TESTS_COST &&
    r.tests.testsPassed === false &&
    r.totalCostUsd === SETUP_COST + IMPL_COST + TESTS_COST,
    'Test 6: testsPassed=false returns aborted with test cost'
  )
}

// Test 7: testsPassed=false without failures key
{
  const tests = { testsPassed: false, testsWritten: 1 }
  const r = phase2Gate(tests, implForP2, IMPL_COST, TESTS_COST, SETUP_COST)
  assert(
    has(r, 'aborted') &&
    r.tests !== null &&
    r.verify === null,
    'Test 7: testsPassed=false without failures still aborts'
  )
}

// Test 8: testsPassed=true
{
  const tests = { testsPassed: true, testsWritten: 5 }
  const r = phase2Gate(tests, implForP2, IMPL_COST, TESTS_COST, SETUP_COST)
  assert(
    r.passed === true &&
    !has(r, 'error') &&
    !has(r, 'aborted'),
    'Test 8: testsPassed=true passes gate to Phase 3'
  )
}

console.log('')
console.log(`=== Total: ${passed} passed, ${failed} failed out of ${passed + failed} ===`)
console.log('')
if (failed > 0) process.exit(1)
