/*
Copyright © 2024, 2025 Red Hat, Inc.

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

package main

import (
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetupCliFlagsIgnoreDisabledRulesRegistered checks that the
// --ignore-disabled-rules flag is registered and its default value is
// false.
func TestSetupCliFlagsIgnoreDisabledRulesRegistered(t *testing.T) {
	// Reset global flag.CommandLine so that setupCliFlags can register
	// flags cleanly without conflicting with previously registered flags
	// or real os.Args.
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	cliFlags := setupCliFlags()
	assert.False(t, cliFlags.IgnoreDisabledRules,
		"IgnoreDisabledRules should default to false")
}

// TestSetupCliFlagsIgnoreDisabledRulesSetTrue checks that passing
// --ignore-disabled-rules on the command line sets the flag to true.
func TestSetupCliFlagsIgnoreDisabledRulesSetTrue(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	// Temporarily override os.Args to include the flag
	origArgs := os.Args
	defer func() { os.Args = origArgs }()
	os.Args = []string{"cmd", "--ignore-disabled-rules"}

	cliFlags := setupCliFlags()
	assert.True(t, cliFlags.IgnoreDisabledRules,
		"IgnoreDisabledRules should be true when --ignore-disabled-rules is passed")
}

// TestSetupCliFlagsIgnoreDisabledRulesDescription checks that the
// --ignore-disabled-rules flag has the expected usage description.
func TestSetupCliFlagsIgnoreDisabledRulesDescription(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	origArgs := os.Args
	defer func() { os.Args = origArgs }()
	os.Args = []string{"cmd"}

	setupCliFlags()

	f := flag.CommandLine.Lookup("ignore-disabled-rules")
	require.NotNil(t, f, "Flag --ignore-disabled-rules should be registered")
	assert.Equal(t,
		"skip disabled rules check, process all rules as if none are disabled",
		f.Usage,
		"Flag description should match the spec")
}

// TestSetupCliFlagsIgnoreDisabledRulesVisibleInHelp checks that the
// --ignore-disabled-rules flag is visible when iterating over all
// registered flags (as --help does).
func TestSetupCliFlagsIgnoreDisabledRulesVisibleInHelp(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	origArgs := os.Args
	defer func() { os.Args = origArgs }()
	os.Args = []string{"cmd"}

	setupCliFlags()

	found := false
	flag.CommandLine.VisitAll(func(f *flag.Flag) {
		if f.Name == "ignore-disabled-rules" {
			found = true
		}
	})
	assert.True(t, found,
		"--ignore-disabled-rules should be visible in --help output (registered flags)")
}

// TestSetupCliFlagsAllExistingFlagsStillRegistered checks that adding
// the new flag did not remove any previously existing flags.
func TestSetupCliFlagsAllExistingFlagsStillRegistered(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	origArgs := os.Args
	defer func() { os.Args = origArgs }()
	os.Args = []string{"cmd"}

	setupCliFlags()

	expectedFlags := []string{
		"instant-reports",
		"show-version",
		"show-authors",
		"show-configuration",
		"print-new-reports-for-cleanup",
		"new-reports-cleanup",
		"print-old-reports-for-cleanup",
		"old-reports-cleanup",
		"cleanup-on-startup",
		"verbose",
		"ignore-disabled-rules",
		"max-age",
	}

	for _, name := range expectedFlags {
		f := flag.CommandLine.Lookup(name)
		assert.NotNilf(t, f, "Flag --%s should be registered", name)
	}
}

// TestSetupCliFlagsIgnoreDisabledRulesDefaultFalseExplicit checks that
// explicitly passing --ignore-disabled-rules=false keeps the value false.
func TestSetupCliFlagsIgnoreDisabledRulesDefaultFalseExplicit(t *testing.T) {
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	origArgs := os.Args
	defer func() { os.Args = origArgs }()
	os.Args = []string{"cmd", "--ignore-disabled-rules=false"}

	cliFlags := setupCliFlags()
	assert.False(t, cliFlags.IgnoreDisabledRules,
		"IgnoreDisabledRules should be false when --ignore-disabled-rules=false is passed")
}
