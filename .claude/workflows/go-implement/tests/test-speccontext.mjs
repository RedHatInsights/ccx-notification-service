// Test script for specContext construction from go-implement.js lines 321-344.
//
// The production code uses a specSections array with conditional .push() calls
// and .join('\n\n') to assemble the specification context injected into every
// agent prompt.

/**
 * Extracted from go-implement.js lines 321-344.
 * Builds the spec context markdown string that gets injected into every agent prompt.
 *
 * @param {string} issueKey - Jira issue key, e.g. "CCXDEV-12345"
 * @param {string} issue - The issue body text (markdown)
 * @param {string|null} designDocPath - Path to a design document, or null/empty
 * @param {string[]} bddPaths - Array of BDD feature-file paths
 * @returns {string} The assembled specContext string
 */
function buildSpecContext(issueKey, issue, designDocPath, bddPaths) {
  // The Jira issue body is always present.
  const specSections = [
    `## Issue Specification (${issueKey})\n\n${issue}`,
  ]

  // Design doc is optional.
  if (designDocPath) {
    specSections.push(`## Design Document\n\nRead the design document at \`${designDocPath}\` for architecture and implementation\nguidance. Use it as the primary guide for approach and structure. Reference it liberally.`)
  }

  // BDD feature files are optional.
  if (bddPaths.length) {
    specSections.push(`## BDD Scenarios\n\nThese feature files describe expected user-facing behavior:\n${bddPaths.map(p => '- `' + p + '`').join('\n')}`)
  }

  return specSections.join('\n\n')
}

// ---------------------------------------------------------------------------
// Test harness
// ---------------------------------------------------------------------------
let passed = 0
let failed = 0

function assert(condition, testName, detail) {
  if (condition) {
    console.log(`[PASS] ${testName}`)
    passed++
  } else {
    console.log(`[FAIL] ${testName} -- ${detail}`)
    failed++
  }
}

// ---------------------------------------------------------------------------
// Test 1: Issue only (no design doc, no BDD)
// ---------------------------------------------------------------------------
{
  const result = buildSpecContext('CCXDEV-100', 'Fix the widget', null, [])

  assert(
    result.includes('## Issue Specification (CCXDEV-100)'),
    'T1: contains issue heading',
    `got: ${result.substring(0, 80)}`
  )
  assert(
    result.includes('Fix the widget'),
    'T1: contains issue body',
    `got: ${result}`
  )
  assert(
    !result.includes('## Design Document'),
    'T1: no design doc section',
    'Design Document section found but should be absent'
  )
  assert(
    !result.includes('## BDD Scenarios'),
    'T1: no BDD section',
    'BDD Scenarios section found but should be absent'
  )
  // With only one section, there should be no double-newline separator
  // (the only \n\n is within the issue section itself, between heading and body)
  const separatorCount = (result.match(/\n\n/g) || []).length
  assert(
    separatorCount === 1,
    'T1: exactly one double-newline (inside issue section)',
    `found ${separatorCount} double-newline sequences`
  )
}

// ---------------------------------------------------------------------------
// Test 2: Issue + design doc path
// ---------------------------------------------------------------------------
{
  const result = buildSpecContext(
    'CCXDEV-200',
    'Add aggregator connection',
    '/home/user/docs/design.md',
    []
  )

  assert(
    result.includes('## Issue Specification (CCXDEV-200)'),
    'T2: contains issue heading',
    `got: ${result.substring(0, 80)}`
  )
  assert(
    result.includes('## Design Document'),
    'T2: contains design doc section',
    'Design Document section missing'
  )
  assert(
    result.includes('`/home/user/docs/design.md`'),
    'T2: contains design doc path in backticks',
    `path not found in: ${result}`
  )
  assert(
    result.includes('primary guide for approach and structure'),
    'T2: contains design doc guidance text',
    'guidance text missing'
  )
  assert(
    !result.includes('## BDD Scenarios'),
    'T2: no BDD section',
    'BDD Scenarios section found but should be absent'
  )
  // Two sections joined by \n\n: the separator is between them
  const issueEnd = result.indexOf('Add aggregator connection') + 'Add aggregator connection'.length
  const designStart = result.indexOf('## Design Document')
  const between = result.substring(issueEnd, designStart)
  assert(
    between === '\n\n',
    'T2: sections separated by exactly one double-newline',
    `separator between sections: ${JSON.stringify(between)}`
  )
}

// ---------------------------------------------------------------------------
// Test 3: Issue + BDD paths (2 paths)
// ---------------------------------------------------------------------------
{
  const bdd = [
    '/home/user/bdd/notification.feature',
    '/home/user/bdd/cooldown.feature',
  ]
  const result = buildSpecContext('CCXDEV-300', 'Cooldown logic', null, bdd)

  assert(
    result.includes('## Issue Specification (CCXDEV-300)'),
    'T3: contains issue heading',
    `got: ${result.substring(0, 80)}`
  )
  assert(
    !result.includes('## Design Document'),
    'T3: no design doc section',
    'Design Document section found but should be absent'
  )
  assert(
    result.includes('## BDD Scenarios'),
    'T3: contains BDD section',
    'BDD Scenarios section missing'
  )
  assert(
    result.includes('- `/home/user/bdd/notification.feature`'),
    'T3: contains first BDD path',
    `first path not found in: ${result}`
  )
  assert(
    result.includes('- `/home/user/bdd/cooldown.feature`'),
    'T3: contains second BDD path',
    `second path not found in: ${result}`
  )
  assert(
    result.includes('expected user-facing behavior'),
    'T3: contains BDD guidance text',
    'guidance text missing'
  )
  // Verify the \n\n separator between issue and BDD (no design doc in between)
  const issueEnd = result.indexOf('Cooldown logic') + 'Cooldown logic'.length
  const bddStart = result.indexOf('## BDD Scenarios')
  const between = result.substring(issueEnd, bddStart)
  assert(
    between === '\n\n',
    'T3: issue and BDD separated by exactly one double-newline',
    `separator: ${JSON.stringify(between)}`
  )
}

// ---------------------------------------------------------------------------
// Test 4: All three (issue + design doc + BDD paths)
// ---------------------------------------------------------------------------
{
  const bdd = ['/bdd/a.feature', '/bdd/b.feature']
  const result = buildSpecContext(
    'CCXDEV-400',
    'Full integration',
    '/docs/arch.md',
    bdd
  )

  assert(
    result.includes('## Issue Specification (CCXDEV-400)'),
    'T4: contains issue heading',
    `got: ${result.substring(0, 80)}`
  )
  assert(
    result.includes('Full integration'),
    'T4: contains issue body',
    'issue body missing'
  )
  assert(
    result.includes('## Design Document'),
    'T4: contains design doc section',
    'Design Document section missing'
  )
  assert(
    result.includes('`/docs/arch.md`'),
    'T4: contains design doc path',
    'design doc path missing'
  )
  assert(
    result.includes('## BDD Scenarios'),
    'T4: contains BDD section',
    'BDD Scenarios section missing'
  )
  assert(
    result.includes('- `/bdd/a.feature`'),
    'T4: contains first BDD path',
    'first BDD path missing'
  )
  assert(
    result.includes('- `/bdd/b.feature`'),
    'T4: contains second BDD path',
    'second BDD path missing'
  )

  // Verify ordering: Issue before Design Doc before BDD
  const issueIdx = result.indexOf('## Issue Specification')
  const designIdx = result.indexOf('## Design Document')
  const bddIdx = result.indexOf('## BDD Scenarios')
  assert(
    issueIdx < designIdx && designIdx < bddIdx,
    'T4: sections in correct order (issue < design < BDD)',
    `indices: issue=${issueIdx}, design=${designIdx}, bdd=${bddIdx}`
  )

  // Verify double-newline separation between all three sections
  const issueSection = result.substring(issueIdx, designIdx)
  assert(
    issueSection.endsWith('\n\n'),
    'T4: issue section ends with double-newline before design doc',
    `issue section tail: ${JSON.stringify(issueSection.slice(-10))}`
  )
  const designSection = result.substring(designIdx, bddIdx)
  assert(
    designSection.endsWith('\n\n'),
    'T4: design doc section ends with double-newline before BDD',
    `design section tail: ${JSON.stringify(designSection.slice(-10))}`
  )
}

// ---------------------------------------------------------------------------
// Test 5: Single BDD path
// ---------------------------------------------------------------------------
{
  const result = buildSpecContext(
    'CCXDEV-500',
    'Single feature test',
    null,
    ['/features/only.feature']
  )

  assert(
    result.includes('## BDD Scenarios'),
    'T5: contains BDD section',
    'BDD Scenarios section missing'
  )
  assert(
    result.includes('- `/features/only.feature`'),
    'T5: contains the single BDD path',
    `path not found in: ${result}`
  )
  // Make sure there is exactly one list item
  const listItems = result.match(/^- `/gm)
  assert(
    listItems && listItems.length === 1,
    'T5: exactly one BDD list item',
    `found ${listItems ? listItems.length : 0} list items`
  )
  // Verify markdown heading format: ## followed by space
  assert(
    /^## BDD Scenarios$/m.test(result),
    'T5: BDD heading uses ## markdown format on its own line',
    `heading not in expected format`
  )
}

// ---------------------------------------------------------------------------
// Test 6: Empty issue body
// ---------------------------------------------------------------------------
{
  const result = buildSpecContext('CCXDEV-600', '', null, [])

  assert(
    result.includes('## Issue Specification (CCXDEV-600)'),
    'T6: contains issue heading even with empty body',
    `got: ${result.substring(0, 80)}`
  )
  // The heading is always present; after it there should be two newlines then empty body
  assert(
    result.includes('(CCXDEV-600)\n\n'),
    'T6: heading followed by double newline (before empty body)',
    `unexpected format: ${JSON.stringify(result)}`
  )
  // The result should be just the heading + \n\n + empty body (no trailing content)
  assert(
    result === '## Issue Specification (CCXDEV-600)\n\n',
    'T6: entire output is just the issue heading with trailing newlines',
    `got: ${JSON.stringify(result)}`
  )
  assert(
    !result.includes('## Design Document'),
    'T6: no design doc section',
    'Design Document found but should be absent'
  )
  assert(
    !result.includes('## BDD Scenarios'),
    'T6: no BDD section',
    'BDD Scenarios found but should be absent'
  )
  // Verify markdown heading format for issue section
  assert(
    /^## Issue Specification \(CCXDEV-600\)$/m.test(result),
    'T6: issue heading uses ## markdown format with key in parens',
    `heading not in expected format`
  )
}

// ---------------------------------------------------------------------------
// Summary
// ---------------------------------------------------------------------------
console.log('\n' + '='.repeat(50))
console.log(`Total: ${passed + failed} | Passed: ${passed} | Failed: ${failed}`)
if (failed > 0) {
  process.exit(1)
} else {
  console.log('All tests passed.')
}
