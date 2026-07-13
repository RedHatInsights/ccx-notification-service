// Copyright 2024, 2025 Red Hat, Inc
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

/*
TestCliFlagsIgnoreDisabledRulesDefaultValue checks that the default value

	of IgnoreDisabledRules in a zero-value CliFlags is false, matching the
	spec requirement that the flag defaults to false.
*/
func TestCliFlagsIgnoreDisabledRulesDefaultValue(t *testing.T) {
	var cliFlags types.CliFlags
	assert.False(t, cliFlags.IgnoreDisabledRules,
		"IgnoreDisabledRules should default to false when CliFlags is zero-initialized")
}

/*
TestCliFlagsIgnoreDisabledRulesCanBeEnabled checks that IgnoreDisabledRules

	can be set to true, as required when the flag is passed on the command
	line.
*/
func TestCliFlagsIgnoreDisabledRulesCanBeEnabled(t *testing.T) {
	cliFlags := types.CliFlags{
		IgnoreDisabledRules: true,
	}
	assert.True(t, cliFlags.IgnoreDisabledRules,
		"IgnoreDisabledRules should be true when explicitly set")
}

/*
TestCliFlagsIgnoreDisabledRulesDoesNotAffectOtherFlags checks that

	setting IgnoreDisabledRules does not interfere with other CLI flags.
*/
func TestCliFlagsIgnoreDisabledRulesDoesNotAffectOtherFlags(t *testing.T) {
	cliFlags := types.CliFlags{
		InstantReports:      true,
		Verbose:             true,
		IgnoreDisabledRules: true,
	}
	assert.True(t, cliFlags.InstantReports,
		"InstantReports should remain true")
	assert.True(t, cliFlags.Verbose,
		"Verbose should remain true")
	assert.True(t, cliFlags.IgnoreDisabledRules,
		"IgnoreDisabledRules should be true")
	assert.False(t, cliFlags.ShowVersion,
		"ShowVersion should remain false (default)")
	assert.False(t, cliFlags.CleanupOnStartup,
		"CleanupOnStartup should remain false (default)")
	assert.Equal(t, "", cliFlags.MaxAge,
		"MaxAge should remain empty (default)")
}
