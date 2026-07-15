// Test script for go-implement.js Phase 1 and Phase 2 gate logic.
//
// Extracts cost functions, logFeedback,
// then simulates Phase 1 and Phase 2
// gate logic with 8 test cases.

// ---------------------------------------------------------------------------
// Extracted logFeedback
// ---------------------------------------------------------------------------
const logs = []
function log(msg) { logs.push(msg) }

function logFeedback(phases) {
  const all = Object.entries(phases)
    .flatMap(([name, result]) => {
      const items = (result && result.feedback) || []
      return items.map(f => `[${name}] ${f}`)
    })
  if (all.length) {
    log('')
    log('Agent feedback on workflow instructions:')
    all.forEach(f => log('  ' + f))
  }
}

// ---------------------------------------------------------------------------
// Simulate Phase 1 gate logic
// ---------------------------------------------------------------------------
// Parameters:
//   impl       - the agent result (null if failed)
//   setupCost  - cost from setup phase
//   implCost   - cost from impl phase (computed externally for testing)
// Returns the same shape the workflow would return at that gate.
function phase1Gate(impl, setupCost, implCost) {
  if (!impl) {
    return {
      error: 'Implementation phase failed',
      implement: null,
      tests: null,
      verify: null,
      setupCostUsd: setupCost,
      totalCostUsd: setupCost,
    }
  }


  const filesModified = impl.filesModified || []

  if (!filesModified.length) {
    return {
      error: 'No files modified',
      implement: { ...impl, costUsd: implCost },
      tests: null,
      verify: null,
      setupCostUsd: setupCost,
      totalCostUsd: setupCost + implCost,
    }
  }

  // Passed gate -- return a sentinel so tests can distinguish
  return { passed: true, implCost }
}

// ---------------------------------------------------------------------------
// Simulate Phase 2 gate logic
// ---------------------------------------------------------------------------
// Parameters:
//   tests      - the agent result (null if failed)
//   impl       - the impl result (spread into output)
//   implCost   - cost from Phase 1
//   testsCost  - cost from Phase 2 (computed externally)
//   setupCost  - cost from setup phase
function phase2Gate(tests, impl, implCost, testsCost, setupCost) {
  if (!tests) {
    return {
      error: 'Unit test phase failed',
      implement: { ...impl, costUsd: implCost },
      tests: null,
      verify: null,
      setupCostUsd: setupCost,
      totalCostUsd: setupCost + implCost,
    }
  }

  if (!tests.testsPassed) {
    if (tests.failures && tests.failures.length) {
      // would log failures here
    }
    logFeedback({ implement: impl, tests })
    return {
      implement: { ...impl, costUsd: implCost },
      tests: { ...tests, costUsd: testsCost },
      verify: null,
      aborted: 'Tests did not pass, verification skipped',
      setupCostUsd: setupCost,
      totalCostUsd: setupCost + implCost + testsCost,
    }
  }

  // Passed gate
  return { passed: true, testsCost }
}

// ---------------------------------------------------------------------------
// Test harness
// ---------------------------------------------------------------------------
let passed = 0
let failed = 0

function assert(condition, label) {
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

// Fixed costs for reproducibility
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
    has(r, 'setupCostUsd') && r.setupCostUsd === SETUP_COST &&
    has(r, 'totalCostUsd') && r.totalCostUsd === SETUP_COST,
    'Test 1: impl=null returns error with null phases, totalCostUsd=setupCost'
  )
}

// Test 2: filesModified=undefined (impl exists but no filesModified key)
{
  const impl = { summary: 'did stuff' }  // no filesModified property
  const r = phase1Gate(impl, SETUP_COST, IMPL_COST)
  assert(
    r.error === 'No files modified' &&
    r.implement !== null &&
    has(r.implement, 'costUsd') && r.implement.costUsd === IMPL_COST &&
    r.implement.summary === 'did stuff' &&
    r.tests === null &&
    r.verify === null &&
    r.setupCostUsd === SETUP_COST &&
    r.totalCostUsd === SETUP_COST + IMPL_COST,
    'Test 2: filesModified=undefined triggers no-files gate, impl spread with costUsd'
  )
}

// Test 3: filesModified=[] (empty array)
{
  const impl = { filesModified: [], summary: 'nothing changed' }
  const r = phase1Gate(impl, SETUP_COST, IMPL_COST)
  assert(
    r.error === 'No files modified' &&
    r.implement !== null &&
    has(r.implement, 'costUsd') && r.implement.costUsd === IMPL_COST &&
    r.implement.filesModified.length === 0 &&
    r.tests === null &&
    r.verify === null &&
    r.setupCostUsd === SETUP_COST &&
    r.totalCostUsd === SETUP_COST + IMPL_COST,
    'Test 3: filesModified=[] triggers no-files gate with spread impl'
  )
}

// Test 4: filesModified=["file.go"] with warnings -- should pass the gate
{
  const impl = {
    filesModified: ['file.go'],
    warnings: ['something looked odd'],
    summary: 'implemented feature',
  }
  const r = phase1Gate(impl, SETUP_COST, IMPL_COST)
  assert(
    r.passed === true &&
    r.implCost === IMPL_COST &&
    !has(r, 'error') &&
    !has(r, 'implement') &&
    !has(r, 'tests') &&
    !has(r, 'verify'),
    'Test 4: filesModified=["file.go"] with warnings passes gate'
  )
}

// ---- Phase 2 tests ----

console.log('\n=== Phase 2 Gate Tests ===\n')

const implForP2 = {
  filesModified: ['differ/storage.go'],
  summary: 'added new query',
}

// Test 5: tests=null
{
  const r = phase2Gate(null, implForP2, IMPL_COST, TESTS_COST, SETUP_COST)
  assert(
    r.error === 'Unit test phase failed' &&
    r.implement !== null &&
    has(r.implement, 'costUsd') && r.implement.costUsd === IMPL_COST &&
    r.implement.summary === 'added new query' &&
    r.tests === null &&
    r.verify === null &&
    has(r, 'setupCostUsd') && r.setupCostUsd === SETUP_COST &&
    has(r, 'totalCostUsd') && r.totalCostUsd === SETUP_COST + IMPL_COST,
    'Test 5: tests=null returns error, impl spread with costUsd, tests/verify null'
  )
}

// Test 6: testsPassed=false WITH failures array
{
  const tests = {
    testsPassed: false,
    testsWritten: 3,
    failures: ['TestFoo: expected 1 got 2', 'TestBar: nil pointer'],
  }
  const r = phase2Gate(tests, implForP2, IMPL_COST, TESTS_COST, SETUP_COST)
  assert(
    !has(r, 'error') &&
    has(r, 'aborted') && r.aborted === 'Tests did not pass, verification skipped' &&
    r.implement !== null && r.implement.costUsd === IMPL_COST &&
    r.tests !== null && r.tests.costUsd === TESTS_COST &&
    r.tests.testsPassed === false &&
    r.tests.failures.length === 2 &&
    r.verify === null &&
    r.setupCostUsd === SETUP_COST &&
    r.totalCostUsd === SETUP_COST + IMPL_COST + TESTS_COST,
    'Test 6: testsPassed=false with failures returns aborted, both phases costed'
  )
}

// Test 7: testsPassed=false WITHOUT failures array
{
  const tests = {
    testsPassed: false,
    testsWritten: 1,
  }
  const r = phase2Gate(tests, implForP2, IMPL_COST, TESTS_COST, SETUP_COST)
  assert(
    has(r, 'aborted') && r.aborted === 'Tests did not pass, verification skipped' &&
    r.implement !== null && r.implement.costUsd === IMPL_COST &&
    r.tests !== null && r.tests.costUsd === TESTS_COST &&
    !has(r.tests, 'failures') &&
    r.verify === null &&
    r.setupCostUsd === SETUP_COST &&
    r.totalCostUsd === SETUP_COST + IMPL_COST + TESTS_COST,
    'Test 7: testsPassed=false without failures still aborts, no failures key'
  )
}

// Test 8: testsPassed=true -- should pass the gate
{
  const tests = {
    testsPassed: true,
    testsWritten: 5,
    failures: [],
  }
  const r = phase2Gate(tests, implForP2, IMPL_COST, TESTS_COST, SETUP_COST)
  assert(
    r.passed === true &&
    r.testsCost === TESTS_COST &&
    !has(r, 'error') &&
    !has(r, 'aborted') &&
    !has(r, 'implement') &&
    !has(r, 'tests') &&
    !has(r, 'verify'),
    'Test 8: testsPassed=true passes gate to Phase 3'
  )
}

// ---------------------------------------------------------------------------
// Summary
// ---------------------------------------------------------------------------
console.log(`\n=== Total: ${passed} passed, ${failed} failed out of ${passed + failed} ===\n`)
process.exit(failed > 0 ? 1 : 0)
