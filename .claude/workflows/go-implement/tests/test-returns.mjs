// Test: return statement consistency in go-implement.js
//
// Reads the workflow file, finds all `return { ... }` statements, categorizes
// each by parsing context (not hardcoded line numbers), and checks consistency
// rules across all returns.

import { readFileSync } from 'fs'

const filePath = new URL('../go-implement.js', import.meta.url).pathname
const source = readFileSync(filePath, 'utf8')
const lines = source.split('\n')

// ---------------------------------------------------------------------------
// 1. Find all return statements that return objects
// ---------------------------------------------------------------------------

const returns = []

for (let i = 0; i < lines.length; i++) {
  if (/^\s*return\s*\{/.test(lines[i])) {
    const startLine = i + 1 // 1-based

    // Collect full return statement, tracking brace depth
    let braceDepth = 0
    let fullText = ''
    let j = i
    for (; j < lines.length; j++) {
      fullText += lines[j] + '\n'
      for (const ch of lines[j]) {
        if (ch === '{') braceDepth++
        if (ch === '}') braceDepth--
      }
      if (braceDepth === 0) break
    }
    const endLine = j + 1 // 1-based

    returns.push({ startLine, endLine, text: fullText.trim() })
  }
}

// ---------------------------------------------------------------------------
// 2. Locate the first phase() call - the boundary between setup and phases
// ---------------------------------------------------------------------------

let firstPhaseCallLine = -1
for (let i = 0; i < lines.length; i++) {
  // Match standalone phase() calls like: phase('Implement')
  // Exclude comments and string references
  const trimmed = lines[i].trimStart()
  if (/^phase\s*\(/.test(trimmed)) {
    firstPhaseCallLine = i + 1 // 1-based
    break
  }
}

if (firstPhaseCallLine === -1) {
  console.error('ERROR: Could not find any phase() call in the workflow file')
  process.exit(1)
}

// ---------------------------------------------------------------------------
// 3. Categorize each return
// ---------------------------------------------------------------------------
// Categories are determined by parsing context, not line numbers:
//   - "pre-phase error": return appears before the first phase() call
//   - "phase failure": return has an `error` field and a null phase result
//   - "gate exit": return is after a phase but has no `error` field and either
//     has `aborted` or has a null phase that indicates early termination
//   - "success": return after phases with no `error` or `aborted` field

function categorize(ret) {
  // Returns before the first phase() call are pre-phase errors
  if (ret.startLine < firstPhaseCallLine) {
    return 'pre-phase error'
  }

  const hasError = /\berror\s*:/.test(ret.text)
  const hasAborted = /\baborted\s*:/.test(ret.text)

  // Phase failure: has an error field (a phase agent returned null)
  if (hasError) {
    return 'phase failure'
  }

  // Gate exit: has aborted field (tests failed) or has null phases indicating
  // early termination without an error (e.g., no files modified)
  if (hasAborted) {
    return 'gate exit'
  }

  // Fallback: returns with null phases but no error or aborted field.
  if (/:\s*null/.test(ret.text) && !hasError && !hasAborted) {
    return 'gate exit'
  }

  // Success: after phases, no error, no aborted
  return 'success'
}

// ---------------------------------------------------------------------------
// 4. Extract top-level field names from a return object
// ---------------------------------------------------------------------------
// Handles comments inside object literals by stripping them first.

function stripComments(text) {
  // Remove single-line comments (// ...) that are not inside strings
  let result = ''
  let inString = false
  let stringChar = ''
  let escaped = false
  let i = 0
  while (i < text.length) {
    const ch = text[i]

    if (escaped) {
      result += ch
      escaped = false
      i++
      continue
    }
    if (ch === '\\' && inString) {
      result += ch
      escaped = true
      i++
      continue
    }
    if (inString) {
      if (ch === stringChar) inString = false
      result += ch
      i++
      continue
    }
    if (ch === "'" || ch === '"' || ch === '`') {
      inString = true
      stringChar = ch
      result += ch
      i++
      continue
    }
    // Check for // comment
    if (ch === '/' && i + 1 < text.length && text[i + 1] === '/') {
      // Skip to end of line
      while (i < text.length && text[i] !== '\n') i++
      continue
    }
    // Check for /* */ comment
    if (ch === '/' && i + 1 < text.length && text[i + 1] === '*') {
      i += 2
      while (i < text.length && !(text[i] === '*' && i + 1 < text.length && text[i + 1] === '/')) i++
      i += 2 // skip */
      continue
    }
    result += ch
    i++
  }
  return result
}

function extractFields(text) {
  // Strip the "return" keyword and outer braces
  const cleaned = stripComments(text)
  const inner = cleaned.replace(/^return\s*\{/, '').replace(/\}\s*$/, '')

  const fields = []
  let depth = 0
  let current = ''
  let inString = false
  let stringChar = ''
  let escaped = false

  for (let i = 0; i < inner.length; i++) {
    const ch = inner[i]

    if (escaped) {
      escaped = false
      continue
    }
    if (ch === '\\') {
      escaped = true
      continue
    }
    if (inString) {
      if (ch === stringChar) inString = false
      continue
    }
    if (ch === "'" || ch === '"' || ch === '`') {
      inString = true
      stringChar = ch
      continue
    }

    if (ch === '{' || ch === '[' || ch === '(') depth++
    if (ch === '}' || ch === ']' || ch === ')') depth--

    if (depth === 0 && ch === ':') {
      const fieldName = current.trim().replace(/,\s*$/, '').trim()
      if (fieldName) fields.push(fieldName)
      current = ''
      continue
    }

    if (depth === 0 && ch === ',') {
      current = ''
      continue
    }

    current += ch
  }

  return fields
}

// Check if a return uses spread with costUsd for a given phase
function usesSpreadWithCost(text, phaseName) {
  const varMap = { implement: 'impl', tests: 'tests', verify: 'verify' }
  const varName = varMap[phaseName]
  if (!varName) return false
  const pattern = new RegExp(
    phaseName + ':\\s*\\{\\s*\\.\\.\\.\\s*' + varName + '\\s*,\\s*costUsd\\s*:'
  )
  return pattern.test(text)
}

// ---------------------------------------------------------------------------
// 5. Analyze each return
// ---------------------------------------------------------------------------

const analyzed = returns.map(ret => {
  const category = categorize(ret)
  const fields = extractFields(ret.text)
  return { ...ret, category, fields }
})

// ---------------------------------------------------------------------------
// 6. Print table
// ---------------------------------------------------------------------------

console.log('='.repeat(120))
console.log('RETURN STATEMENT ANALYSIS')
console.log('='.repeat(120))
console.log('')
console.log('First phase() call at line ' + firstPhaseCallLine)
console.log('')

console.log(
  'Line'.padEnd(12) +
  'Category'.padEnd(22) +
  'Fields'
)
console.log('-'.repeat(120))

for (const ret of analyzed) {
  const lineRange = ret.startLine === ret.endLine
    ? `L${ret.startLine}`
    : `L${ret.startLine}-${ret.endLine}`

  console.log(
    lineRange.padEnd(12) +
    ret.category.padEnd(22) +
    ret.fields.join(', ')
  )
}

console.log('')
console.log(`Total return statements: ${analyzed.length}`)
console.log('')

// ---------------------------------------------------------------------------
// 7. Consistency checks
// ---------------------------------------------------------------------------

console.log('='.repeat(120))
console.log('CONSISTENCY CHECKS')
console.log('='.repeat(120))
console.log('')

const phaseFailures = analyzed.filter(r => r.category === 'phase failure')
const gateExits = analyzed.filter(r => r.category === 'gate exit')
const successReturns = analyzed.filter(r => r.category === 'success')
const nonPrePhase = [...phaseFailures, ...gateExits, ...successReturns]

let totalChecks = 0
let passedChecks = 0

function check(name, items, condition) {
  totalChecks++
  const failures = []
  for (const item of items) {
    if (!condition(item)) {
      const lineRange = item.startLine === item.endLine
        ? `L${item.startLine}`
        : `L${item.startLine}-${item.endLine}`
      failures.push(lineRange)
    }
  }
  const passed = failures.length === 0
  if (passed) passedChecks++
  const status = passed ? '[PASS]' : '[FAIL]'
  console.log(`${status} ${name}`)
  if (!passed) {
    console.log(`       Failed at: ${failures.join(', ')}`)
  }
  return passed
}

// Check 1: All phase-failure and gate-exit returns have setupCostUsd and totalCostUsd
check(
  'All phase-failure/gate-exit returns have setupCostUsd',
  [...phaseFailures, ...gateExits],
  r => r.fields.includes('setupCostUsd')
)

check(
  'All phase-failure/gate-exit returns have totalCostUsd',
  [...phaseFailures, ...gateExits],
  r => r.fields.includes('totalCostUsd')
)

// Check 2: All returns with phase data (implement/tests/verify) use spread with costUsd
check(
  'All returns with implement data use { ...impl, costUsd }',
  analyzed.filter(r => /implement\s*:\s*\{/.test(r.text)),
  r => usesSpreadWithCost(r.text, 'implement')
)

check(
  'All returns with tests data use { ...tests, costUsd }',
  analyzed.filter(r => /tests\s*:\s*\{/.test(r.text)),
  r => usesSpreadWithCost(r.text, 'tests')
)

check(
  'All returns with verify data use { ...verify, costUsd }',
  analyzed.filter(r => /verify\s*:\s*\{/.test(r.text)),
  r => usesSpreadWithCost(r.text, 'verify')
)

// Check 3: All non-pre-phase returns include implement, tests, verify fields
check(
  'All non-pre-phase returns include "implement" field',
  nonPrePhase,
  r => r.fields.includes('implement')
)

check(
  'All non-pre-phase returns include "tests" field',
  nonPrePhase,
  r => r.fields.includes('tests')
)

check(
  'All non-pre-phase returns include "verify" field',
  nonPrePhase,
  r => r.fields.includes('verify')
)

console.log('')
console.log('-'.repeat(120))
console.log(`Total: ${passedChecks}/${totalChecks} checks passed`)
console.log('')

// Summary by category
console.log('Returns by category:')
const categories = {}
for (const r of analyzed) {
  categories[r.category] = (categories[r.category] || 0) + 1
}
for (const [cat, count] of Object.entries(categories)) {
  console.log(`  ${cat}: ${count}`)
}

if (passedChecks < totalChecks) {
  process.exit(1)
}
