/*
Copyright © 2022 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package differ

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEvaluatorDefaultExpression function checks the filter expression
// evaluator for default expression used by Notification Service
func TestEvaluatorDefaultExpression(t *testing.T) {
	const expression = "totalRisk >= totalRiskThreshold"
	const totalRiskThreshold = 3

	// try all combinations of totalRisk
	for totalRisk := 0; totalRisk <= 5; totalRisk++ {
		// try to evaluate the expression
		result, err := evaluateFilterExpression(expression,
			0, 0, 0, totalRiskThreshold,
			0, 0, totalRisk)

		// expression should be evaluated w/o errors
		assert.NoError(t, err, "Error is not expected there")

		// repeat the expression, but now in Go
		if totalRisk >= totalRiskThreshold {
			assert.Equal(t, 1, result, "Result should be 1")
		} else {
			assert.Equal(t, 0, result, "Result should be 0")
		}
	}
}

// TestEvaluatorComplicatedExpression function checks the filter expression
// evaluator for custom expression
func TestEvaluatorComplicatedExpression(t *testing.T) {
	const expression = "totalRisk >= totalRiskThreshold && likelihood+2 > likelihoodThreshold"
	const totalRiskThreshold = 3
	const likelihoodThreshold = 2

	// try all combinations of likelihood and totalRisk
	for likelihood := 0; likelihood <= 5; likelihood++ {
		for totalRisk := 0; totalRisk <= 5; totalRisk++ {
			// try to evaluate the expression
			result, err := evaluateFilterExpression(expression,
				likelihoodThreshold, 0, 0, totalRiskThreshold,
				likelihood, 0, totalRisk)

			// expression should be evaluated w/o errors
			assert.NoError(t, err, "Error is not expected there")

			// repeat the expression, but now in Go
			if totalRisk >= totalRiskThreshold && likelihood+2 > likelihoodThreshold {
				assert.Equal(t, 1, result, "Result should be 1")
			} else {
				assert.Equal(t, 0, result, "Result should be 0")
			}
		}
	}
}

// TestEvaluatorEmptyExpression function checks the filter expression
// evaluator for expression that is empty
func TestEvaluatorEmptyExpression(t *testing.T) {
	const expression = ""
	const totalRiskThreshold = 3
	const likelihoodThreshold = 2

	// try all combinations of likelihood and totalRisk
	for likelihood := 0; likelihood <= 5; likelihood++ {
		for totalRisk := 0; totalRisk <= 5; totalRisk++ {
			// try to evaluate the expression
			_, err := evaluateFilterExpression(expression,
				likelihoodThreshold, 0, 0, totalRiskThreshold,
				likelihood, 0, totalRisk)

			// error should be reported for incorrect expression
			assert.Error(t, err, "Error is expected there")
		}
	}
}

// TestEvaluatorIncorrectExpression function checks the filter expression
// evaluator for expression that is not correct
func TestEvaluatorIncorrectExpression(t *testing.T) {
	// this is wrong expression
	const expression = "totalRisk >="
	const totalRiskThreshold = 3
	const likelihoodThreshold = 2

	// try all combinations of likelihood and totalRisk
	for likelihood := 0; likelihood <= 5; likelihood++ {
		for totalRisk := 0; totalRisk <= 5; totalRisk++ {
			// try to evaluate the expression
			_, err := evaluateFilterExpression(expression,
				likelihoodThreshold, 0, 0, totalRiskThreshold,
				likelihood, 0, totalRisk)

			// error should be reported for incorrect expression
			assert.Error(t, err, "Error is expected there")
		}
	}
}

type TestCase struct {
	name          string
	expression    string
	expectedValue int
	expectedError bool
}

// TestEvaluatorRelational checks the evaluator.Evaluate function for simple
// relational expression
func TestEvaluatorRelational(t *testing.T) {
	testCases := []TestCase{
		{
			name:          "less than",
			expression:    "totalRisk < totalRiskThreshold",
			expectedValue: 1,
		},
		{
			name:          "less then or equal",
			expression:    "totalRisk <= totalRiskThreshold",
			expectedValue: 1,
		},
		{
			name:          "equal",
			expression:    "totalRisk == totalRiskThreshold",
			expectedValue: 0,
		},
		{
			name:          "greater or equal",
			expression:    "totalRisk >= totalRiskThreshold",
			expectedValue: 0,
		},
		{
			name:          "greater than",
			expression:    "totalRisk > totalRiskThreshold",
			expectedValue: 0,
		},
		{
			name:          "not equal",
			expression:    "totalRisk != totalRiskThreshold",
			expectedValue: 1,
		},
		{
			name:          "arithmetic + less than",
			expression:    "totalRisk + 1 < totalRiskThreshold",
			expectedValue: 0,
		},
		{
			name:          "arithmetic + less then or equal",
			expression:    "totalRisk + 1 <= totalRiskThreshold",
			expectedValue: 1,
		},
		{
			name:          "arithmetic + equal",
			expression:    "totalRisk + 1 == totalRiskThreshold",
			expectedValue: 1,
		},
		{
			name:          "arithmetic + greater or equal",
			expression:    "totalRisk + 1 >= totalRiskThreshold",
			expectedValue: 1,
		},
		{
			name:          "arithmetic + greater than",
			expression:    "totalRisk + 1 > totalRiskThreshold",
			expectedValue: 0,
		},
		{
			name:          "arithmetic + not equal",
			expression:    "totalRisk + 1 != totalRiskThreshold",
			expectedValue: 0,
		},
	}

	const totalRiskThreshold = 3
	const totalRisk = 2
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := evaluateFilterExpression(tc.expression,
				0, 0, 0, totalRiskThreshold,
				0, 0, totalRisk)
			assert.NoError(t, err, "unexpected error")
			assert.Equal(t, tc.expectedValue, result)
		})
	}
}
