// Test suite for logFeedback() from go-implement.js (lines 95-109)
//
// Uses the actual phase key names from the workflow: implement, tests, verify.

// --- Extract the function under test, injecting a mock log() ---

let captured = []

function log(...args) {
  captured.push(args.join(' '))
}

function logFeedback(phases) {
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

// --- Test harness ---

let passed = 0
let failed = 0

function test(name, fn) {
  captured = []
  try {
    fn()
    passed++
    console.log(`[PASS] ${name}`)
  } catch (e) {
    failed++
    console.log(`[FAIL] ${name}`)
    console.log(`       ${e.message}`)
  }
}

function assert(cond, msg) {
  if (!cond) throw new Error(msg)
}

function assertDeep(actual, expected, msg) {
  const a = JSON.stringify(actual)
  const e = JSON.stringify(expected)
  if (a !== e) throw new Error(`${msg}\n         expected: ${e}\n         actual:   ${a}`)
}

// --- Tests ---

test('1. No feedback from any phase', () => {
  logFeedback({
    implement: { feedback: [] },
    tests:     { feedback: [] },
    verify:    { feedback: [] },
  })
  assertDeep(captured, [], 'should produce no output when all feedback arrays are empty')
})

test('2. One phase with feedback', () => {
  logFeedback({
    implement: { feedback: ['improve the prompt'] },
    tests:     { feedback: [] },
    verify:    { feedback: [] },
  })
  assertDeep(captured, [
    '',
    'Agent feedback on workflow instructions:',
    '  [implement] improve the prompt',
  ], 'should log feedback from the single phase that has it')
})

test('3. All three phases with feedback', () => {
  logFeedback({
    implement: { feedback: ['impl note'] },
    tests:     { feedback: ['test note'] },
    verify:    { feedback: ['verify note'] },
  })
  assertDeep(captured, [
    '',
    'Agent feedback on workflow instructions:',
    '  [implement] impl note',
    '  [tests] test note',
    '  [verify] verify note',
  ], 'should log feedback from all phases in insertion order')
})

test('4. Null phase result', () => {
  logFeedback({
    implement: { feedback: ['something'] },
    tests:     null,
    verify:    { feedback: ['other'] },
  })
  assertDeep(captured, [
    '',
    'Agent feedback on workflow instructions:',
    '  [implement] something',
    '  [verify] other',
  ], 'null result should be safely skipped via (result && result.feedback) || []')
})

test('5. Missing feedback field ({})', () => {
  logFeedback({
    implement: {},
    tests:     {},
    verify:    {},
  })
  assertDeep(captured, [], 'empty objects with no feedback field should produce no output')
})

test('6. Undefined feedback', () => {
  logFeedback({
    implement: { feedback: undefined },
    tests:     { feedback: undefined },
    verify:    { feedback: undefined },
  })
  assertDeep(captured, [], 'undefined feedback should be treated as empty via || []')
})

test('7. Multiple items from one phase', () => {
  logFeedback({
    implement: { feedback: ['first note', 'second note', 'third note'] },
  })
  assertDeep(captured, [
    '',
    'Agent feedback on workflow instructions:',
    '  [implement] first note',
    '  [implement] second note',
    '  [implement] third note',
  ], 'multiple feedback items from one phase should each get their own line')
})

test('8. Early exit (2 phases only)', () => {
  // Matches the actual early-exit call at line 592: logFeedback({ implement: impl, tests })
  logFeedback({
    implement: { feedback: ['early feedback'] },
    tests:     { feedback: ['test feedback'] },
  })
  assertDeep(captured, [
    '',
    'Agent feedback on workflow instructions:',
    '  [implement] early feedback',
    '  [tests] test feedback',
  ], 'should work with fewer than 3 phases (mirrors the early exit path)')
})

test('9. Object.entries ordering preserved', () => {
  // Object.entries preserves insertion order for string keys in modern JS engines.
  // Use the actual phase keys in a deliberate order to confirm.
  const phases = {}
  phases.verify    = { feedback: ['v'] }
  phases.implement = { feedback: ['i'] }
  phases.tests     = { feedback: ['t'] }

  logFeedback(phases)

  // Order must follow insertion: verify, implement, tests
  assert(captured[2] === '  [verify] v',    `expected verify first, got: ${captured[2]}`)
  assert(captured[3] === '  [implement] i', `expected implement second, got: ${captured[3]}`)
  assert(captured[4] === '  [tests] t',     `expected tests third, got: ${captured[4]}`)
})

// --- Summary ---

console.log('')
console.log(`Total: ${passed + failed} | Passed: ${passed} | Failed: ${failed}`)
process.exit(failed > 0 ? 1 : 0)
