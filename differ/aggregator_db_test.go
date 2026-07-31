/*
Copyright © 2025, 2026 Red Hat, Inc.

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

package differ_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"

	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/differ"
	"github.com/RedHatInsights/ccx-notification-service/tests/mocks"
	"github.com/RedHatInsights/ccx-notification-service/types"
)

// TestAggregatorStorageErrorMessage checks the error message of
// AggregatorStorageError.
func TestAggregatorStorageErrorMessage(t *testing.T) {
	err := &differ.AggregatorStorageError{}
	assert.Equal(t, "AggregatorStorageError", err.Error())
}

// --- loadDisabledRules tests (unit, with mock storage) ---

// TestLoadDisabledRulesPopulatesMap verifies the happy path: when
// ReadClusterRuleToggles returns data, loadDisabledRules populates
// ClusterDisabledRules on the Differ struct.
func TestLoadDisabledRulesPopulatesMap(t *testing.T) {
	expected := types.ClusterDisabledRules{
		{ClusterID: "cluster-1", RuleID: "rule.module.1", ErrorKey: "ek1"}: {},
		{ClusterID: "cluster-2", RuleID: "rule.module.2", ErrorKey: "ek2"}: {},
	}

	storage := mocks.Storage{}
	storage.On("ReadClusterRuleToggles").Return(expected, nil)

	d := differ.Differ{
		AggregatorStorage:    &storage,
		ClusterDisabledRules: make(types.ClusterDisabledRules),
	}
	err := differ.LoadDisabledRules(&d)

	assert.NoError(t, err)
	assert.Len(t, d.ClusterDisabledRules, 2)
	for key := range expected {
		_, ok := d.ClusterDisabledRules[key]
		assert.True(t, ok, "expected key %v in ClusterDisabledRules", key)
	}
	storage.AssertExpectations(t)
}

// TestLoadDisabledRulesEmptyTable verifies that when the aggregator
// table has no disabled rules, the map stays empty.
func TestLoadDisabledRulesEmptyTable(t *testing.T) {
	storage := mocks.Storage{}
	storage.On("ReadClusterRuleToggles").Return(make(types.ClusterDisabledRules), nil)

	d := differ.Differ{
		AggregatorStorage:    &storage,
		ClusterDisabledRules: make(types.ClusterDisabledRules),
	}
	err := differ.LoadDisabledRules(&d)

	assert.NoError(t, err)
	assert.NotNil(t, d.ClusterDisabledRules)
	assert.Empty(t, d.ClusterDisabledRules)
	storage.AssertExpectations(t)
}

// TestLoadDisabledRulesQueryError verifies that when ReadClusterRuleToggles
// fails, loadDisabledRules returns an AggregatorStorageError.
func TestLoadDisabledRulesQueryError(t *testing.T) {
	storage := mocks.Storage{}
	storage.On("ReadClusterRuleToggles").Return(types.ClusterDisabledRules(nil), fmt.Errorf("query failed"))

	d := differ.Differ{
		AggregatorStorage:    &storage,
		ClusterDisabledRules: make(types.ClusterDisabledRules),
	}
	err := differ.LoadDisabledRules(&d)

	assert.ErrorIs(t, err, &differ.AggregatorStorageError{})
	assert.Empty(t, d.ClusterDisabledRules, "map should not be modified on error")
	storage.AssertExpectations(t)
}

// TestLoadDisabledRulesLogsCount verifies that the info log with the
// disabled rules count is emitted on success.
func TestLoadDisabledRulesLogsCount(t *testing.T) {
	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.InfoLevel)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	defer zerolog.SetGlobalLevel(zerolog.WarnLevel)

	expected := types.ClusterDisabledRules{
		{ClusterID: "c1", RuleID: "r1", ErrorKey: "ek1"}: {},
	}

	storage := mocks.Storage{}
	storage.On("ReadClusterRuleToggles").Return(expected, nil)

	d := differ.Differ{
		AggregatorStorage:    &storage,
		ClusterDisabledRules: make(types.ClusterDisabledRules),
	}
	_ = differ.LoadDisabledRules(&d)

	executionLog := buf.String()
	assert.Contains(t, executionLog, "Loaded per-cluster disabled rules from cluster_rule_toggle")
}

// --- fetchDisabledRulesFromAggregatorDB tests (real DB, same pattern as Run) ---

// TestFetchDisabledRulesFromAggregatorDBConnectionFailure tests that when the
// aggregator DB connection fails, the method returns an AggregatorStorageError.
func TestFetchDisabledRulesFromAggregatorDBConnectionFailure(t *testing.T) {
	config := conf.ConfigStruct{
		AggregatorStorage: conf.StorageConfiguration{
			Driver: "unsupported-driver",
		},
	}

	d := differ.Differ{}
	err := differ.FetchDisabledRulesFromAggregatorDB(&d, &config)
	assert.ErrorIs(t, err, &differ.AggregatorStorageError{})
}

// TestFetchDisabledRulesFromAggregatorDBLogsConnectionMessage verifies that the
// connection attempt log message is produced.
func TestFetchDisabledRulesFromAggregatorDBLogsConnectionMessage(t *testing.T) {
	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.InfoLevel)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	defer zerolog.SetGlobalLevel(zerolog.WarnLevel)

	config := conf.ConfigStruct{
		AggregatorStorage: conf.StorageConfiguration{
			Driver: "unsupported-driver",
		},
	}

	d := differ.Differ{}
	_ = differ.FetchDisabledRulesFromAggregatorDB(&d, &config)

	executionLog := buf.String()
	assert.Contains(t, executionLog, differ.AggregatorDBConnectionMessage)
}

// TestFetchDisabledRulesFromAggregatorDBLogsOnConnectionFailure verifies the error
// log message when the aggregator DB connection fails.
func TestFetchDisabledRulesFromAggregatorDBLogsOnConnectionFailure(t *testing.T) {
	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.ErrorLevel)
	zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	defer zerolog.SetGlobalLevel(zerolog.WarnLevel)

	config := conf.ConfigStruct{
		AggregatorStorage: conf.StorageConfiguration{
			Driver: "unsupported-driver",
		},
	}

	d := differ.Differ{}
	_ = differ.FetchDisabledRulesFromAggregatorDB(&d, &config)

	executionLog := buf.String()
	assert.Contains(t, executionLog, "Cannot connect to the aggregator database")
}

// TestFetchDisabledRulesFromAggregatorDBFailureDoesNotLogClose verifies that when
// connection fails, the close log message is NOT produced.
func TestFetchDisabledRulesFromAggregatorDBFailureDoesNotLogClose(t *testing.T) {
	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.InfoLevel)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	defer zerolog.SetGlobalLevel(zerolog.WarnLevel)

	config := conf.ConfigStruct{
		AggregatorStorage: conf.StorageConfiguration{
			Driver: "unsupported-driver",
		},
	}

	d := differ.Differ{}
	_ = differ.FetchDisabledRulesFromAggregatorDB(&d, &config)

	executionLog := buf.String()
	assert.NotContains(t, executionLog, differ.AggregatorDBClosedMessage)
}

// TestFetchDisabledRulesFromAggregatorDBReadTogglesFailure tests that when the
// aggregator DB connection succeeds but cluster_rule_toggle does not exist,
// the method returns an AggregatorStorageError.
func TestFetchDisabledRulesFromAggregatorDBReadTogglesFailure(t *testing.T) {
	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.ErrorLevel)
	zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	defer zerolog.SetGlobalLevel(zerolog.WarnLevel)

	config := conf.ConfigStruct{
		AggregatorStorage: conf.StorageConfiguration{
			Driver: "sqlite3",
		},
	}

	d := differ.Differ{}
	err := differ.FetchDisabledRulesFromAggregatorDB(&d, &config)

	assert.ErrorIs(t, err, &differ.AggregatorStorageError{})

	executionLog := buf.String()
	assert.Contains(t, executionLog, "Cannot read cluster rule toggles from the aggregator database")
}

// --- Run-level integration tests ---

// TestRunIgnoreDisabledRulesSkipsAggregatorDB verifies that when
// --ignore-disabled-rules is set, no aggregator DB connection attempt is made.
func TestRunIgnoreDisabledRulesSkipsAggregatorDB(t *testing.T) {
	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.InfoLevel)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	defer zerolog.SetGlobalLevel(zerolog.WarnLevel)

	config := conf.ConfigStruct{
		Storage: conf.StorageConfiguration{
			Driver: "sqlite3",
		},
		ServiceLog: conf.ServiceLogConfiguration{
			Enabled: true,
		},
		AggregatorStorage: conf.StorageConfiguration{
			Driver: "sqlite3",
		},
	}
	cliFlags := types.CliFlags{
		InstantReports:      true,
		IgnoreDisabledRules: true,
	}
	retval := differ.Run(config, cliFlags)
	assert.Equal(t, differ.ExitStatusFetchContentError, retval)

	executionLog := buf.String()
	assert.Contains(t, executionLog, differ.AggregatorDBSkippedMessage)
	assert.NotContains(t, executionLog, differ.AggregatorDBConnectionMessage)
}

// TestRunAggregatorDBFailureExitCode verifies that Run returns
// ExitStatusAggregatorStorageError when the aggregator DB is unreachable.
func TestRunAggregatorDBFailureExitCode(t *testing.T) {
	config := conf.ConfigStruct{
		Storage: conf.StorageConfiguration{
			Driver: "sqlite3",
		},
		ServiceLog: conf.ServiceLogConfiguration{
			Enabled: true,
		},
		AggregatorStorage: conf.StorageConfiguration{
			Driver: "unsupported-driver",
		},
	}
	cliFlags := types.CliFlags{
		InstantReports: true,
	}
	retval := differ.Run(config, cliFlags)
	assert.Equal(t, differ.ExitStatusAggregatorStorageError, retval)
}

// --- Constants and error type tests ---

// TestExitStatusAggregatorStorageErrorValue checks that
// ExitStatusAggregatorStorageError has the expected value.
func TestExitStatusAggregatorStorageErrorValue(t *testing.T) {
	assert.Equal(t, differ.ExitStatusServiceLogError+1, differ.ExitStatusAggregatorStorageError)
}

// TestExportedAggregatorDBConstants checks that the exported message constants
// have the expected values.
func TestExportedAggregatorDBConstants(t *testing.T) {
	assert.Equal(t, "Connecting to aggregator database to fetch disabled rules", differ.AggregatorDBConnectionMessage)
	assert.Equal(t, "Aggregator database connection closed", differ.AggregatorDBClosedMessage)
	assert.Equal(t, "Skipping aggregator DB connection (--ignore-disabled-rules is set)", differ.AggregatorDBSkippedMessage)
}

// TestNewInitializesClusterDisabledRulesMap checks that New() initializes
// ClusterDisabledRules as an empty (not nil) map.
func TestNewInitializesClusterDisabledRulesMap(t *testing.T) {
	config := conf.ConfigStruct{
		ServiceLog: conf.ServiceLogConfiguration{
			Enabled: true,
		},
	}
	d, err := differ.New(&config, nil)
	assert.NoError(t, err)

	assert.NotNil(t, d.ClusterDisabledRules, "ClusterDisabledRules should be initialized by New()")
	assert.Empty(t, d.ClusterDisabledRules, "ClusterDisabledRules should be empty after construction")
}

// TestIgnoreDisabledRulesClusterDisabledRulesStaysEmpty verifies that when
// --ignore-disabled-rules is set, ClusterDisabledRules remains empty because
// the aggregator DB connection is skipped entirely.
func TestIgnoreDisabledRulesClusterDisabledRulesStaysEmpty(t *testing.T) {
	config := conf.ConfigStruct{
		Storage: conf.StorageConfiguration{
			Driver: "sqlite3",
		},
		ServiceLog: conf.ServiceLogConfiguration{
			Enabled: true,
		},
		AggregatorStorage: conf.StorageConfiguration{
			Driver: "sqlite3",
		},
	}
	cliFlags := types.CliFlags{
		InstantReports:      true,
		IgnoreDisabledRules: true,
	}
	retval := differ.Run(config, cliFlags)
	assert.Equal(t, differ.ExitStatusFetchContentError, retval)
}
