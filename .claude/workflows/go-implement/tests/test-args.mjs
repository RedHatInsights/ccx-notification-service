// Test script for the args parsing logic from go-implement.js lines 260-282.
//
// Extracts the guard logic into a parseArgs() function, mocks log(),
// and tests 12 input scenarios.

// ---------------------------------------------------------------------------
// Extracted parseArgs function
// ---------------------------------------------------------------------------

/**
 * Extracted args-parsing logic from go-implement.js lines 266-282.
 * Variable names match the workflow exactly: issueKey, designDocPath,
 * bddPathsRaw, bddPaths.
 *
 * @param {*} args  - the raw value the workflow runtime would pass
 * @param {Function} log - mock for the workflow's log() function
 * @returns {{ result: object|null, returned: object|null }}
 *   result   - the parsed output ({ issueKey, designDocPath, bddPathsRaw, bddPaths }) or null on early return
 *   returned - the early-return value ({ error }) if the function bailed out, else null
 */
function parseArgs(args, log) {
  // --- lines 266-272 ---
  let parsed
  try {
    parsed = typeof args === 'string' ? JSON.parse(args) : args
  } catch (e) {
    log('Error: args is not valid JSON: ' + e.message)
    return { result: null, returned: { error: 'Invalid JSON args: ' + e.message } }
  }

  // --- lines 274-277 ---
  if (!parsed || !parsed.issue) {
    log('Error: args.issue is required. Pass a Jira issue key (e.g., { "issue": "CCXDEV-12345" }).')
    return { result: null, returned: { error: 'args.issue is required - pass a Jira issue key like "CCXDEV-12345"' } }
  }

  // --- lines 279-282 ---
  const issueKey = parsed.issue
  const designDocPath = parsed.designDoc || ''
  const bddPathsRaw = parsed.bddPaths || []
  const bddPaths = Array.isArray(bddPathsRaw) ? bddPathsRaw : [bddPathsRaw]

  return { result: { issueKey, designDocPath, bddPathsRaw, bddPaths }, returned: null }
}

// ---------------------------------------------------------------------------
// Test harness
// ---------------------------------------------------------------------------

let passCount = 0
let failCount = 0

function createLog() {
  const messages = []
  const fn = (msg) => messages.push(msg)
  fn.messages = messages
  return fn
}

function runTest(testNum, description, testFn) {
  const log = createLog()
  try {
    const { pass, detail } = testFn(log)
    if (pass) {
      passCount++
      console.log(`[PASS] Test ${testNum}: ${description}`)
    } else {
      failCount++
      console.log(`[FAIL] Test ${testNum}: ${description}`)
      console.log(`       ${detail}`)
      if (log.messages.length) {
        console.log(`       Logged: ${log.messages.join(' | ')}`)
      }
    }
  } catch (err) {
    failCount++
    console.log(`[FAIL] Test ${testNum}: ${description}`)
    console.log(`       Exception: ${err.message}`)
  }
}

// ---------------------------------------------------------------------------
// Test cases
// ---------------------------------------------------------------------------

console.log('=== Testing go-implement.js args parsing (lines 260-282) ===')
console.log('')

// 1. args=undefined
runTest(1, 'args=undefined returns issue-required error', (log) => {
  const { result, returned } = parseArgs(undefined, log)
  const ok = result === null &&
    returned !== null &&
    returned.error.includes('args.issue is required') &&
    log.messages[0].includes('args.issue is required')
  return {
    pass: ok,
    detail: `result=${JSON.stringify(result)}, returned=${JSON.stringify(returned)}, logs=${JSON.stringify(log.messages)}`,
  }
})

// 2. args=null
runTest(2, 'args=null returns issue-required error', (log) => {
  const { result, returned } = parseArgs(null, log)
  const ok = result === null &&
    returned !== null &&
    returned.error.includes('args.issue is required') &&
    log.messages[0].includes('args.issue is required')
  return {
    pass: ok,
    detail: `result=${JSON.stringify(result)}, returned=${JSON.stringify(returned)}, logs=${JSON.stringify(log.messages)}`,
  }
})

// 3. args="" (empty string)
runTest(3, 'args="" (empty string) returns invalid JSON error', (log) => {
  const { result, returned } = parseArgs('', log)
  const ok = result === null &&
    returned !== null &&
    returned.error.includes('Invalid JSON args') &&
    log.messages[0].includes('not valid JSON')
  return {
    pass: ok,
    detail: `result=${JSON.stringify(result)}, returned=${JSON.stringify(returned)}, logs=${JSON.stringify(log.messages)}`,
  }
})

// 4. args='{"issue":"CCXDEV-123"}' (JSON string)
runTest(4, 'args=JSON string with issue parses correctly', (log) => {
  const { result, returned } = parseArgs('{"issue":"CCXDEV-123"}', log)
  const ok = result !== null &&
    result.issueKey === 'CCXDEV-123' &&
    result.designDocPath === '' &&
    Array.isArray(result.bddPathsRaw) && result.bddPathsRaw.length === 0 &&
    Array.isArray(result.bddPaths) && result.bddPaths.length === 0 &&
    returned === null &&
    log.messages.length === 0
  return {
    pass: ok,
    detail: `result=${JSON.stringify(result)}, returned=${JSON.stringify(returned)}, logs=${JSON.stringify(log.messages)}`,
  }
})

// 5. args={issue:"CCXDEV-123"} (object)
runTest(5, 'args=object with issue parses correctly', (log) => {
  const { result, returned } = parseArgs({ issue: 'CCXDEV-123' }, log)
  const ok = result !== null &&
    result.issueKey === 'CCXDEV-123' &&
    result.designDocPath === '' &&
    Array.isArray(result.bddPathsRaw) && result.bddPathsRaw.length === 0 &&
    Array.isArray(result.bddPaths) && result.bddPaths.length === 0 &&
    returned === null &&
    log.messages.length === 0
  return {
    pass: ok,
    detail: `result=${JSON.stringify(result)}, returned=${JSON.stringify(returned)}, logs=${JSON.stringify(log.messages)}`,
  }
})

// 6. args={issue:""} (empty issue)
runTest(6, 'args={issue:""} returns issue-required error', (log) => {
  const { result, returned } = parseArgs({ issue: '' }, log)
  const ok = result === null &&
    returned !== null &&
    returned.error.includes('args.issue is required') &&
    log.messages[0].includes('args.issue is required')
  return {
    pass: ok,
    detail: `result=${JSON.stringify(result)}, returned=${JSON.stringify(returned)}, logs=${JSON.stringify(log.messages)}`,
  }
})

// 7. args={issue:" "} (whitespace-only issue)
// The original code does `!parsed.issue` which is falsy only for empty string.
// " " (space) is truthy, so it passes validation and becomes the issueKey.
runTest(7, 'args={issue:" "} (whitespace issue is accepted by the code)', (log) => {
  const { result, returned } = parseArgs({ issue: ' ' }, log)
  const ok = result !== null &&
    result.issueKey === ' ' &&
    returned === null
  return {
    pass: ok,
    detail: `result=${JSON.stringify(result)}, returned=${JSON.stringify(returned)}, logs=${JSON.stringify(log.messages)}`,
  }
})

// 8. args={issue:"X",bddPaths:"/single.feature"} (string bddPaths)
runTest(8, 'bddPaths as string is coerced to single-element array', (log) => {
  const { result, returned } = parseArgs({ issue: 'X', bddPaths: '/single.feature' }, log)
  const ok = result !== null &&
    result.issueKey === 'X' &&
    result.bddPathsRaw === '/single.feature' &&
    Array.isArray(result.bddPaths) &&
    result.bddPaths.length === 1 &&
    result.bddPaths[0] === '/single.feature' &&
    returned === null
  return {
    pass: ok,
    detail: `result=${JSON.stringify(result)}, returned=${JSON.stringify(returned)}`,
  }
})

// 9. args={issue:"X",bddPaths:["/a.feature","/b.feature"]}
runTest(9, 'bddPaths as array is preserved', (log) => {
  const { result, returned } = parseArgs({ issue: 'X', bddPaths: ['/a.feature', '/b.feature'] }, log)
  const ok = result !== null &&
    Array.isArray(result.bddPathsRaw) &&
    result.bddPathsRaw.length === 2 &&
    Array.isArray(result.bddPaths) &&
    result.bddPaths.length === 2 &&
    result.bddPaths[0] === '/a.feature' &&
    result.bddPaths[1] === '/b.feature' &&
    returned === null
  return {
    pass: ok,
    detail: `result=${JSON.stringify(result)}, returned=${JSON.stringify(returned)}`,
  }
})

// 10. args={issue:"X",designDoc:"/doc.md"}
runTest(10, 'designDoc is captured in designDocPath', (log) => {
  const { result, returned } = parseArgs({ issue: 'X', designDoc: '/doc.md' }, log)
  const ok = result !== null &&
    result.issueKey === 'X' &&
    result.designDocPath === '/doc.md' &&
    returned === null
  return {
    pass: ok,
    detail: `result=${JSON.stringify(result)}, returned=${JSON.stringify(returned)}`,
  }
})

// 11. args={issue:"X"} (no optionals)
runTest(11, 'no optionals - defaults applied', (log) => {
  const { result, returned } = parseArgs({ issue: 'X' }, log)
  const ok = result !== null &&
    result.issueKey === 'X' &&
    result.designDocPath === '' &&
    Array.isArray(result.bddPathsRaw) && result.bddPathsRaw.length === 0 &&
    Array.isArray(result.bddPaths) && result.bddPaths.length === 0 &&
    returned === null &&
    log.messages.length === 0
  return {
    pass: ok,
    detail: `result=${JSON.stringify(result)}, returned=${JSON.stringify(returned)}, logs=${JSON.stringify(log.messages)}`,
  }
})

// 12. args="not json"
runTest(12, 'args="not json" returns invalid JSON error', (log) => {
  const { result, returned } = parseArgs('not json', log)
  const ok = result === null &&
    returned !== null &&
    returned.error.includes('Invalid JSON args') &&
    log.messages[0].includes('not valid JSON')
  return {
    pass: ok,
    detail: `result=${JSON.stringify(result)}, returned=${JSON.stringify(returned)}, logs=${JSON.stringify(log.messages)}`,
  }
})

// ---------------------------------------------------------------------------
// Summary
// ---------------------------------------------------------------------------

console.log('')
console.log(`--- Summary: ${passCount} passed, ${failCount} failed out of ${passCount + failCount} tests ---`)
process.exit(failCount > 0 ? 1 : 0)
