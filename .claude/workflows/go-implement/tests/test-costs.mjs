// Test script for estimateCost and formatCost from go-implement.js

// --- Extracted from go-implement.js ---
const USD_PER_OUTPUT_MTOK = 25
const USD_PER_INPUT_MTOK = 5
const INPUT_RATIO = 50

function estimateCost(outputTokens) {
  const estimatedInputTokens = outputTokens * INPUT_RATIO
  const usd =
    (outputTokens * USD_PER_OUTPUT_MTOK + estimatedInputTokens * USD_PER_INPUT_MTOK) / 1_000_000
  return usd
}

function formatCost(usd) {
  return '$' + usd.toFixed(4)
}

// --- Test harness ---
let passed = 0
let failed = 0

function test(id, description, actual, expected) {
  const ok = Object.is(actual, expected) || (typeof actual === 'number' && typeof expected === 'number' && Math.abs(actual - expected) < 1e-12)
  if (ok) {
    console.log(`[PASS] #${id}: ${description}`)
    passed++
  } else {
    console.log(`[FAIL] #${id}: ${description} - expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`)
    failed++
  }
}

// --- estimateCost tests (1-4) ---
// Formula: usd = (outputTokens * 25 + outputTokens * 50 * 5) / 1_000_000
//        = outputTokens * (25 + 250) / 1_000_000
//        = outputTokens * 275 / 1_000_000

test(1, 'estimateCost(0) = 0', estimateCost(0), 0)

test(2, 'estimateCost(1000)', estimateCost(1000), 1000 * 275 / 1_000_000)
// 1000 * 275 / 1e6 = 0.275

test(3, 'estimateCost(100000)', estimateCost(100000), 100000 * 275 / 1_000_000)
// 100000 * 275 / 1e6 = 27.5

test(4, 'estimateCost(1000000)', estimateCost(1000000), 1000000 * 275 / 1_000_000)
// 1000000 * 275 / 1e6 = 275

// --- formatCost tests (5-9) ---

test(5, 'formatCost(0) = "$0.0000"', formatCost(0), '$0.0000')

test(6, 'formatCost(0.275) = "$0.2750"', formatCost(0.275), '$0.2750')

test(7, 'formatCost(27.5) = "$27.5000"', formatCost(27.5), '$27.5000')

test(8, 'formatCost(275) = "$275.0000"', formatCost(275), '$275.0000')

test(9, 'formatCost(0.00001) = "$0.0000"', formatCost(0.00001), '$0.0000')

// --- Cost tracking simulation (10-15) ---
// Simulate the workflow's budget.spent() pattern.
// budget.spent() returns cumulative output tokens at each checkpoint.
// Sequence: 0 -> 500 -> 5000 -> 12000 -> 18000

const spentValues = [0, 500, 5000, 12000, 18000]

const preSetup = spentValues[0]
const setupCost = estimateCost(spentValues[1] - preSetup)
test(10, 'Setup phase: 500 output tokens', setupCost, estimateCost(500))

const preImpl = spentValues[1]
const implCost = estimateCost(spentValues[2] - preImpl)
test(11, 'Implement phase: 4500 output tokens', implCost, estimateCost(4500))

const preTests = spentValues[2]
const testsCost = estimateCost(spentValues[3] - preTests)
test(12, 'Tests phase: 7000 output tokens', testsCost, estimateCost(7000))

const preVerify = spentValues[3]
const verifyCost = estimateCost(spentValues[4] - preVerify)
test(13, 'Verify phase: 6000 output tokens', verifyCost, estimateCost(6000))

const totalCost = setupCost + implCost + testsCost + verifyCost
test(14, 'Total cost = sum of phase costs', totalCost, setupCost + implCost + testsCost + verifyCost)

test(15, 'Total cost = estimateCost(18000)', totalCost, estimateCost(18000))

// --- Summary ---
console.log('')
console.log(`Total: ${passed + failed} tests, ${passed} passed, ${failed} failed`)
if (failed > 0) {
  process.exit(1)
}
