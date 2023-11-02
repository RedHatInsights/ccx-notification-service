/*
Copyright © 2023 Red Hat, Inc.

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

// Documentation in literate-programming-style is available at:
// https://redhatinsights.github.io/ccx-notification-writer/packages/differ/storage_test.html

import (
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"

	"github.com/RedHatInsights/ccx-notification-service/differ"
	"github.com/RedHatInsights/ccx-notification-service/types"
)

// TestPrintNewReportsForCleanupFunction checks if function printNewReportsForCleanup performs
// the right query when non empty set is returned.
func TestPrintNewReportsForCleanupFunction(t *testing.T) {
	cliFlags := types.CliFlags{
		MaxAge: "1 day",
	}

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// prepare mocked result for SQL query
	rows := sqlmock.NewRows([]string{"org_id", "account_number", "cluster_name", "updated_at", "kafka_offset"})

	// these three rows should be returned
	rows.AddRow(0, 1000, "ID=0", time.Now(), 0)
	rows.AddRow(1, 1001, "ID=1", time.Now(), 0)
	rows.AddRow(2, 1002, "ID=2", time.Now(), 0)

	// expected query performed by tested function
	expectedQuery := "SELECT org_id, account_number, cluster, updated_at, kafka_offset\n\t\t  FROM new_reports\n\t\t WHERE updated_at < NOW\\(\\) - \\$1::INTERVAL\n\t\t ORDER BY updated_at\n"

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested function
	err := differ.PrintNewReportsForCleanup(storage, cliFlags)

	// tested method should NOT return an error
	assert.NoError(t, err, "error was not expected while querying database")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestPrintNewReportsForCleanupFunctionOnError checks if function printNewReportsForCleanup performs
// the right query when non empty set is returned.
func TestPrintNewReportsForCleanupFunctionOnError(t *testing.T) {
	cliFlags := types.CliFlags{
		MaxAge: "1 day",
	}

	// error to be thrown
	mockedError := errors.New("mocked error")

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// expected query performed by tested function
	expectedQuery := "SELECT org_id, account_number, cluster, updated_at, kafka_offset\n\t\t  FROM new_reports\n\t\t WHERE updated_at < NOW\\(\\) - \\$1::INTERVAL\n\t\t ORDER BY updated_at\n"

	// let's raise an error!
	mock.ExpectQuery(expectedQuery).WillReturnError(mockedError)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested function
	err := differ.PrintNewReportsForCleanup(storage, cliFlags)

	// tested method should NOT return an error
	assert.Error(t, err, "error was expected while querying database")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}
