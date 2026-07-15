// Test suite for collectFeedback() from go-implement.js
//
// collectFeedback both logs feedback to the workflow progress output
// and returns a markdown section string for displaySummary.

const logs = []
function log(msg) { logs.push(msg) }

// Extracted from go-implement.js
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

let passed = 0
let failed = 0

function test(id, description, fn) {
  logs.length = 0
  try {
    const ok = fn()
    if (ok) {
      console.log(`[PASS] ${id}. ${description}`)
      passed++
    } else {
      console.log(`[FAIL] ${id}. ${description}`)
      failed++
    }
  } catch (e) {
    console.log(`[FAIL] ${id}. ${description} (threw: ${e.message})`)
    failed++
  }
}

// 1. No feedback from any phase
test(1, 'No feedback returns empty string and no logs', () => {
  const result = collectFeedback({
    implement: { summary: 'did stuff', feedback: [] },
    tests: { testsWritten: 3, feedback: [] },
    verify: { verdict: 'pass', feedback: [] },
  })
  return result === '' && logs.length === 0
})

// 2. Feedback from one phase
test(2, 'Single phase feedback returns markdown and logs', () => {
  const result = collectFeedback({
    implement: { feedback: ['instruction 2 was unclear'] },
  })
  return result.includes('### Agent feedback') &&
    result.includes('- **[implement]** instruction 2 was unclear') &&
    logs.some(l => l.includes('[implement] instruction 2 was unclear'))
})

// 3. Feedback from all three phases
test(3, 'All phases produce prefixed entries', () => {
  const result = collectFeedback({
    implement: { feedback: ['a'] },
    tests: { feedback: ['b'] },
    verify: { feedback: ['c'] },
  })
  return result.includes('**[implement]** a') &&
    result.includes('**[tests]** b') &&
    result.includes('**[verify]** c') &&
    logs.filter(l => l.startsWith('  [')).length === 3
})

// 4. Null phase result
test(4, 'Null phase result does not crash', () => {
  const result = collectFeedback({
    implement: null,
    tests: { feedback: ['something'] },
  })
  return result.includes('**[tests]** something') &&
    !result.includes('[implement]')
})

// 5. Missing feedback field
test(5, 'Phase with no feedback field produces nothing', () => {
  const result = collectFeedback({
    implement: { summary: 'did stuff' },
  })
  return result === '' && logs.length === 0
})

// 6. Undefined feedback
test(6, 'Undefined feedback treated as empty', () => {
  const result = collectFeedback({
    implement: { feedback: undefined },
  })
  return result === '' && logs.length === 0
})

// 7. Multiple items from one phase
test(7, 'Multiple items from one phase each get a line', () => {
  const result = collectFeedback({
    implement: { feedback: ['first', 'second', 'third'] },
  })
  const bullets = result.split('\n').filter(l => l.startsWith('- **[implement]**'))
  return bullets.length === 3 &&
    logs.filter(l => l.startsWith('  [implement]')).length === 3
})

// 8. Early exit with two phases (verify missing)
test(8, 'Works with only implement and tests', () => {
  const result = collectFeedback({
    implement: { feedback: ['a'] },
    tests: { feedback: ['b'] },
  })
  return result.includes('**[implement]** a') &&
    result.includes('**[tests]** b') &&
    !result.includes('[verify]')
})

// 9. Object.entries ordering preserved
test(9, 'Phase order matches insertion order', () => {
  const result = collectFeedback({
    implement: { feedback: ['first'] },
    tests: { feedback: ['second'] },
    verify: { feedback: ['third'] },
  })
  const implIdx = result.indexOf('[implement]')
  const testsIdx = result.indexOf('[tests]')
  const verifyIdx = result.indexOf('[verify]')
  return implIdx < testsIdx && testsIdx < verifyIdx
})

// 10. Return value is valid markdown
test(10, 'Markdown starts with newlines and heading', () => {
  const result = collectFeedback({
    implement: { feedback: ['x'] },
  })
  return result.startsWith('\n\n### Agent feedback on workflow instructions\n\n')
})

// 11. Log output matches expected format
test(11, 'Log lines use [phaseName] prefix format', () => {
  collectFeedback({
    implement: { feedback: ['check this'] },
  })
  return logs[0] === '' &&
    logs[1] === 'Agent feedback on workflow instructions:' &&
    logs[2] === '  [implement] check this'
})

console.log('')
console.log(`Total: ${passed + failed} | Passed: ${passed} | Failed: ${failed}`)
if (failed > 0) process.exit(1)
