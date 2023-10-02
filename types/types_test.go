// Copyright 2022 Red Hat, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package types_test

import (
	"testing"

	"github.com/RedHatInsights/ccx-notification-service/types"
	"github.com/stretchr/testify/assert"
)

func TestMakeSetOfTags(t *testing.T) {
	var testScenarios = []struct {
		input    []string
		expected types.TagsSet
	}{
		{
			input:    []string{},
			expected: map[string]struct{}{},
		},
		{
			input: []string{"tag1"},
			expected: map[string]struct{}{
				"tag1": {},
			},
		},
		{
			input: []string{"tag1", "tag2"},
			expected: map[string]struct{}{
				"tag1": {},
				"tag2": {},
			},
		},
		{
			input: []string{"tag1", "tag1"},
			expected: map[string]struct{}{
				"tag1": {},
			},
		},
	}

	for _, scenario := range testScenarios {
		value := types.MakeSetOfTags(scenario.input)
		assert.Equal(t, value, scenario.expected)
	}
}

/* TestToString function checks the behaviour of EventType.ToString() method. */
func TestToString(t *testing.T) {
	const expected = "new-recommendation"

	asString := types.InstantNotif.ToString()
	assert.Equal(t, expected, asString, "Unexpected string returned for InstantNotif.ToString")
}

/* TestError function checks the behaviour of MissingMandatoryFile.Error() method. */
func TestError(t *testing.T) {
	/* create error structure first */
	err := types.MissingMandatoryFile{
		FileName: "foo",
	}

	const expected = "Missing required file: foo"
	asString := err.Error()
	assert.Equal(t, expected, asString, "Unexpected error string")
}

/* TestErrorEmptyFilename function checks the behaviour of MissingMandatoryFile.Error() method. */
func TestErrorEmptyFilename(t *testing.T) {
	/* create error structure first */
	err := types.MissingMandatoryFile{
		FileName: "",
	}

	const expected = "Missing required file: "
	asString := err.Error()
	assert.Equal(t, expected, asString, "Unexpected error string")
}
