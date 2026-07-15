// Test script for Phase 3 verify agent prompt construction
// Extracted from go-implement.js 

// ---------------------------------------------------------------------------
// Extract the FEEDBACK_PROMPT (verbatim from go-implement.js )
// ---------------------------------------------------------------------------
const FEEDBACK_PROMPT = `## Workflow feedback

Your structured output includes a \`feedback\` array of strings. Add one entry per issue you encountered with these instructions - not about the code. For example: an instruction that was unclear or ambiguous, a step that was missing, a prescribed approach that did not work, the Jira spec contradicting the design document or BDD scenarios (if provided), or anything else that would help improve these instructions. Leave it empty if everything worked as described.`

// ---------------------------------------------------------------------------
// Extract the template literal into a standalone function
// (verbatim from go-implement.js )
// ---------------------------------------------------------------------------
function buildVerifyPrompt(specContext, tests, baseSha) {
  return `You are an independent reviewer. Your job is to find problems, not to confirm everything is fine. Be skeptical. Default to flagging concerns rather than assuming correctness. The project conventions from AGENTS.md are already in your context.

${specContext}

## Context

A previous agent reported: tests passed=${tests.testsPassed}, testsWritten=${tests.testsWritten}. Verify this independently, do not assume it is accurate. All changes are in the working tree, not committed.

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
7. If \`make before_commit\` fails, check whether the failing test or file appears in the diff (\`git diff ${baseSha}\`). If it does not, the failure is likely pre-existing. Also check whether the failure could be caused by a changed dependency (e.g., a modified interface that an existing test relies on). Note the distinction between new and pre-existing failures in your report.

## Constraints

- Do not fix any code. This is a read-only review.
- Do not silently skip a failing check.
- Do not create any git commits.

## Before finishing, verify

1. You checked every acceptance criteria against the Jira issue, the implementation and the respective tests, as well as the design doc (if provided) and BDD specs (if provided).
2. You ran \`make before_commit\` and reported the full output.
3. Every finding has a severity (critical, major, minor, or note).
4. Your verdict is one of: pass, fail, or pass_with_notes.

${FEEDBACK_PROMPT}`
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------
let passed = 0
let failed = 0

function check(condition, detail) {
  if (condition) {
    console.log(`  [PASS] ${detail}`)
    passed++
  } else {
    console.log(`  [FAIL] ${detail}`)
    failed++
  }
}

// ---------------------------------------------------------------------------
// Test 1: No failures (failures=[]) - no "already identified" text
// ---------------------------------------------------------------------------
console.log('\n--- Test 1: No failures (failures=[]) ---')
{
  const prompt = buildVerifyPrompt(
    '## Spec\nDo the thing.',
    { testsPassed: true, testsWritten: 3, failures: [] },
    'abc1234'
  )

  check(
    !prompt.includes('already identified'),
    'Empty failures array does not produce "already identified" text'
  )
  check(
    !prompt.includes('The following test failures'),
    'Empty failures array does not produce the known-failures block'
  )
  check(
    !prompt.includes('undefined'),
    'No "undefined" leaks into text'
  )
  check(
    !prompt.includes('null'),
    'No "null" leaks into text'
  )
  // The empty conditional leaves just a blank line between context and severity
  check(
    prompt.includes('not committed.\n\n\n\n## Severity'),
    'Blank line placeholder between context and severity sections'
  )
}

// ---------------------------------------------------------------------------
// Test 2: Failures undefined - no crash, no "undefined" in output
// ---------------------------------------------------------------------------
console.log('\n--- Test 2: Failures undefined ---')
{
  const prompt = buildVerifyPrompt(
    '## Spec\nDo the thing.',
    { testsPassed: false, testsWritten: 0 },
    'def5678'
  )

  check(
    !prompt.includes('The following test failures'),
    'Undefined failures does not produce the known-failures block'
  )
  check(
    !prompt.includes('undefined'),
    'No "undefined" leaks when failures is missing'
  )
  check(
    prompt.includes('tests passed=false'),
    'testsPassed=false is interpolated correctly'
  )
  check(
    prompt.includes('testsWritten=0'),
    'testsWritten=0 is interpolated correctly'
  )
}

// ---------------------------------------------------------------------------
// Test 3: Two known failures - both as markdown bullets with real newlines
// ---------------------------------------------------------------------------
console.log('\n--- Test 3: Two known failures ---')
{
  const prompt = buildVerifyPrompt(
    '## Spec\nMultiple failures.',
    { testsPassed: false, testsWritten: 5, failures: ['TestFoo failed: exit 1', 'TestBar: nil pointer'] },
    'aaa1111'
  )

  check(
    prompt.includes('The following test failures were already identified'),
    'Known-failures header present'
  )
  check(
    prompt.includes('- TestFoo failed: exit 1'),
    'First failure bullet rendered'
  )
  check(
    prompt.includes('- TestBar: nil pointer'),
    'Second failure bullet rendered'
  )
  // Verify actual newline between bullets, not literal backslash-n
  const idx1 = prompt.indexOf('- TestFoo failed: exit 1')
  const idx2 = prompt.indexOf('- TestBar: nil pointer')
  check(
    idx2 === idx1 + '- TestFoo failed: exit 1'.length + 1,
    'Bullets separated by a real newline (\\n), not literal \\n'
  )
  check(
    !prompt.includes('\\n'),
    'No literal backslash-n in output'
  )
}

// ---------------------------------------------------------------------------
// Test 4: Single failure
// ---------------------------------------------------------------------------
console.log('\n--- Test 4: Single failure ---')
{
  const prompt = buildVerifyPrompt(
    '## Spec\nSingle fail.',
    { testsPassed: false, testsWritten: 2, failures: ['TestAlpha panicked'] },
    'bbb2222'
  )

  check(
    prompt.includes('The following test failures were already identified'),
    'Known-failures header present for single failure'
  )
  check(
    prompt.includes('- TestAlpha panicked'),
    'Single failure bullet rendered'
  )
  // Only one bullet, no extra join newlines
  const lines = prompt.split('\n')
  const bulletLines = lines.filter(l => l.startsWith('- TestAlpha'))
  check(
    bulletLines.length === 1,
    'Exactly one bullet line for single failure'
  )
}

// ---------------------------------------------------------------------------
// Test 5: baseSha interpolation - SHA appears in git diff commands
// ---------------------------------------------------------------------------
console.log('\n--- Test 5: baseSha interpolation ---')
{
  const sha = 'deadbeef1234567890abcdef'
  const prompt = buildVerifyPrompt(
    '## Spec\nSha test.',
    { testsPassed: true, testsWritten: 1, failures: [] },
    sha
  )

  // baseSha appears in two places: instruction 1 and instruction 7
  const gitDiffOccurrences = prompt.split(`git diff ${sha}`).length - 1
  check(
    gitDiffOccurrences === 2,
    `baseSha appears exactly 2 times in git diff commands (found ${gitDiffOccurrences})`
  )

  // Verify it's inside backticks (inline code)
  check(
    prompt.includes(`\`git diff ${sha}\``),
    'git diff command is wrapped in backticks (inline code)'
  )

  // Verify backticks are real backtick characters, not escaped
  check(
    !prompt.includes('\\`'),
    'No escaped backticks in output'
  )
}

// ---------------------------------------------------------------------------
// Test 6: Full prompt with all specContext sections
//         Check headings, no uninterpolated ${}, beforeCommitPassed mentioned
// ---------------------------------------------------------------------------
console.log('\n--- Test 6: Full prompt with all specContext sections ---')
{
  const specContext = `## Issue Specification (CCXDEV-99999)

Summary: Add new database table for tracking widget states

### Acceptance Criteria

1. Create migration adding \`widget_states\` table
2. Add Go struct with JSON tags
3. Write CRUD operations in storage.go

## Design Document

Read the design document at \`/path/to/design.md\` for architecture and implementation
guidance.

## BDD Scenarios

These feature files describe expected user-facing behavior:
- \`/path/to/widgets.feature\``

  const tests = {
    testsPassed: true,
    testsWritten: 7,
    failures: ['TestWidgetCleanup: context deadline exceeded'],
  }
  const sha = 'f00dcafe'

  const prompt = buildVerifyPrompt(specContext, tests, sha)

  // -- Headings present --
  check(
    prompt.includes('## Issue Specification (CCXDEV-99999)'),
    'Issue specification heading present'
  )
  check(
    prompt.includes('## Design Document'),
    'Design document heading present'
  )
  check(
    prompt.includes('## BDD Scenarios'),
    'BDD scenarios heading present'
  )
  check(
    prompt.includes('## Context'),
    'Context heading present'
  )
  check(
    prompt.includes('## Severity definitions'),
    'Severity definitions heading present'
  )
  check(
    prompt.includes('## Instructions'),
    'Instructions heading present'
  )
  check(
    prompt.includes('## Constraints'),
    'Constraints heading present'
  )
  check(
    prompt.includes('## Before finishing, verify'),
    'Before finishing heading present'
  )
  check(
    prompt.includes('## Workflow feedback'),
    'Workflow feedback heading present'
  )

  // -- No uninterpolated ${} --
  // eslint-disable-next-line no-template-curly-in-string
  const unresolvedPattern = /\$\{[^}]+\}/
  check(
    !unresolvedPattern.test(prompt),
    'No uninterpolated ${...} placeholders remain in the output'
  )

  // -- beforeCommitPassed is mentioned, not the old codeStylePassed --
  check(
    prompt.includes('beforeCommitPassed'),
    'beforeCommitPassed is mentioned in the prompt'
  )
  check(
    !prompt.includes('codeStylePassed'),
    'Old codeStylePassed is NOT mentioned anywhere in the prompt'
  )

  // -- Content interpolation checks --
  check(
    prompt.includes('tests passed=true, testsWritten=7'),
    'Test stats interpolated correctly'
  )
  check(
    prompt.includes('- TestWidgetCleanup: context deadline exceeded'),
    'Known failure bullet present'
  )
  check(
    prompt.includes(`\`git diff ${sha}\``),
    'baseSha interpolated in instructions'
  )
  check(
    prompt.includes('`make before_commit`'),
    'make before_commit reference present'
  )

  // -- Structural checks --
  const headings = prompt.split('\n').filter(l => /^## /.test(l))
  check(
    headings.length >= 7,
    `At least 7 ## headings found (got ${headings.length})`
  )

  // -- No leaks --
  check(
    !prompt.includes('[object Object]'),
    'No [object Object] in output'
  )

  // -- All four severity levels --
  check(
    prompt.includes('- **critical**:') &&
      prompt.includes('- **major**:') &&
      prompt.includes('- **minor**:') &&
      prompt.includes('- **note**:'),
    'All four severity levels present with bold markdown'
  )

  // -- Prompt ends cleanly --
  check(
    prompt.trimEnd().endsWith('Leave it empty if everything worked as described.'),
    'Prompt ends cleanly with final feedback instruction'
  )

  // -- Starts correctly --
  check(
    prompt.startsWith('You are an independent reviewer.'),
    'Prompt starts with reviewer preamble'
  )
}

// ---------------------------------------------------------------------------
// Summary
// ---------------------------------------------------------------------------
console.log('\n' + '='.repeat(50))
console.log(`Total: ${passed + failed} tests | ${passed} passed | ${failed} failed`)
if (failed > 0) {
  console.log('SOME TESTS FAILED')
  process.exit(1)
} else {
  console.log('ALL TESTS PASSED')
}
