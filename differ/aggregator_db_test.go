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
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"

	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/differ"
	"github.com/RedHatInsights/ccx-notification-service/types"
)

// TestAggregatorStorageErrorMessage checks the error message of
// AggregatorStorageError.
func TestAggregatorStorageErrorMessage(t *testing.T) {
	err := &differ.AggregatorStorageError{}
	assert.Equal(t, "AggregatorStorageError", err.Error())
}

// TestConnectAndCloseAggregatorDBSuccess tests the happy path where the
// aggregator DB connection is established and closed successfully. This
// covers the acceptance criteria: "Aggregator DB connection is established
// before any other startup work" and "Connection is closed after disabled
// rules are fetched."
func TestConnectAndCloseAggregatorDBSuccess(t *testing.T) {
	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.InfoLevel)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	defer zerolog.SetGlobalLevel(zerolog.WarnLevel)

	config := conf.ConfigStruct{
		AggregatorStorage: conf.StorageConfiguration{
			Driver: "sqlite3",
		},
	}

	d := differ.Differ{}
	err := differ.ConnectAndCloseAggregatorDB(&d, &config)
	assert.Nil(t, err)

	// After successful connect-and-close, AggregatorStorage should be nil
	// (connection was cleaned up).
	assert.Nil(t, d.AggregatorStorage)

	executionLog := buf.String()
	assert.Contains(t, executionLog, differ.AggregatorDBConnectionMessage)
	assert.Contains(t, executionLog, differ.AggregatorDBClosedMessage)
}

// TestConnectAndCloseAggregatorDBConnectionFailure tests that when the
// aggregator DB connection fails, the method returns an AggregatorStorageError.
// This covers: "If the aggregator DB is unreachable, the service exits with
// an appropriate error."
func TestConnectAndCloseAggregatorDBConnectionFailure(t *testing.T) {
	config := conf.ConfigStruct{
		AggregatorStorage: conf.StorageConfiguration{
			Driver: "unsupported-driver",
		},
	}

	d := differ.Differ{}
	err := differ.ConnectAndCloseAggregatorDB(&d, &config)
	assert.ErrorIs(t, err, &differ.AggregatorStorageError{})
	assert.Nil(t, d.AggregatorStorage)
}

// TestConnectAndCloseAggregatorDBLogsConnectionMessage verifies that the
// connection attempt log message is produced.
func TestConnectAndCloseAggregatorDBLogsConnectionMessage(t *testing.T) {
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
	_ = differ.ConnectAndCloseAggregatorDB(&d, &config)

	executionLog := buf.String()
	assert.Contains(t, executionLog, differ.AggregatorDBConnectionMessage)
}

// TestConnectAndCloseAggregatorDBLogsOnConnectionFailure verifies the error
// log message when the aggregator DB connection fails.
func TestConnectAndCloseAggregatorDBLogsOnConnectionFailure(t *testing.T) {
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
	_ = differ.ConnectAndCloseAggregatorDB(&d, &config)

	executionLog := buf.String()
	assert.Contains(t, executionLog, "Cannot connect to the aggregator database")
}

// TestConnectAndCloseAggregatorDBFailureDoesNotLogClose verifies that when
// connection fails, the close log message is NOT produced. This demonstrates
// the fail-fast behavior: the method returns immediately on connection error.
func TestConnectAndCloseAggregatorDBFailureDoesNotLogClose(t *testing.T) {
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
	_ = differ.ConnectAndCloseAggregatorDB(&d, &config)

	executionLog := buf.String()
	assert.NotContains(t, executionLog, differ.AggregatorDBClosedMessage)
}

// TestRunIgnoreDisabledRulesSkipsAggregatorDB verifies that when
// --ignore-disabled-rules is set, no aggregator DB connection attempt is made.
// This is the acceptance criteria: "If --ignore-disabled-rules is set, no
// connection attempt is made."
// The aggregator storage is configured with an invalid driver so that any
// connection attempt would fail. If the skip logic works, the service proceeds
// past the aggregator DB step and fails later (content fetch), proving no
// connection was attempted.
// Uses ServiceLog path to avoid mock Kafka broker overhead.
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
			Driver: "invalid-driver-should-never-be-used",
		},
	}
	cliFlags := types.CliFlags{
		InstantReports:      true,
		IgnoreDisabledRules: true,
	}
	retval := differ.Run(config, cliFlags)
	// The service should skip the aggregator DB connection entirely and
	// proceed to the next step (fetching content), which will fail because
	// no content service is running. FetchContentError means we got past
	// the aggregator DB step successfully.
	assert.Equal(t, differ.ExitStatusFetchContentError, retval)

	executionLog := buf.String()
	assert.Contains(t, executionLog, differ.AggregatorDBSkippedMessage)
	assert.NotContains(t, executionLog, differ.AggregatorDBConnectionMessage)
}

// TestRunAggregatorDBFailureExitCode verifies that Run returns
// ExitStatusAggregatorStorageError when the aggregator DB is unreachable.
// This tests the full integration path through Run -> start ->
// connectAndCloseAggregatorDB -> selectError.
// Uses ServiceLog path to avoid mock Kafka broker overhead.
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

// TestExitStatusAggregatorStorageErrorValue checks that
// ExitStatusAggregatorStorageError has the expected value (one past
// ExitStatusServiceLogError).
func TestExitStatusAggregatorStorageErrorValue(t *testing.T) {
	assert.Equal(t, differ.ExitStatusServiceLogError+1, differ.ExitStatusAggregatorStorageError)
}

// TestExportedAggregatorDBConstants checks that the exported message constants
// have the expected values from the spec.
func TestExportedAggregatorDBConstants(t *testing.T) {
	assert.Equal(t, "Connecting to aggregator database to fetch disabled rules", differ.AggregatorDBConnectionMessage)
	assert.Equal(t, "Aggregator database connection closed", differ.AggregatorDBClosedMessage)
	assert.Equal(t, "Skipping aggregator DB connection (--ignore-disabled-rules is set)", differ.AggregatorDBSkippedMessage)
}
