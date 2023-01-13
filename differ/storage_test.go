/*
Copyright © 2022, 2023 Red Hat, Inc.

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
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"

	"github.com/RedHatInsights/ccx-notification-service/differ"
	"github.com/RedHatInsights/ccx-notification-service/types"
)

// wrongDatabaseDriver is any integer value different from DBDriverSQLite3 and
// DBDriverPostgres
const wrongDatabaseDriver = 10

// mustCreateMockConnection function tries to create a new mock connection and
// checks if the operation was finished without problems.
func mustCreateMockConnection(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	// try to initialize new mock connection
	connection, mock, err := sqlmock.New()

	// check the status
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	return connection, mock
}

// checkConnectionClose function perform mocked DB closing operation and checks
// if the connection is properly closed from unit tests.
func checkConnectionClose(t *testing.T, connection *sql.DB) {
	// connection to mocked DB needs to be closed properly
	err := connection.Close()

	// check the error status
	if err != nil {
		t.Fatalf("error during closing connection: %v", err)
	}
}

// checkAllExpectations function checks if all database-related operations have
// been really met.
func checkAllExpectations(t *testing.T, mock sqlmock.Sqlmock) {
	// check if all expectations were met
	err := mock.ExpectationsWereMet()

	// check the error status
	if err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestReadLastNotifiedRecordForClusterList(t *testing.T) {
	var (
		now            = time.Now()
		clusters       = "'first cluster','second cluster'"
		orgs           = "'1','2'"
		clusterEntries = []types.ClusterEntry{
			{
				OrgID:         1,
				AccountNumber: 1,
				ClusterName:   "first cluster",
				KafkaOffset:   1,
				UpdatedAt:     types.Timestamp(now),
			},
			{
				OrgID:         2,
				AccountNumber: 2,
				ClusterName:   "second cluster",
				KafkaOffset:   1,
				UpdatedAt:     types.Timestamp(now),
			},
		}
		timeOffset           = "1 day"
		timeOffsetNotSet     = ""
		timeOffsetEmptySpace = "   "
		timeOffsetSetToZero  = "0"
		timeOffsetSetToZeroX = "0 hours"
	)

	db, mock := newMock(t)
	defer func() { _ = db.Close() }()

	sut := differ.NewFromConnection(db, types.DBDriverPostgres)

	expectedQuery := fmt.Sprintf(`
	SELECT org_id, cluster, report, notified_at
	FROM (
		SELECT DISTINCT ON (cluster) *
		FROM reported
		WHERE event_type_id = %v AND state = 1 AND org_id IN (%v) AND cluster IN (%v)
		ORDER BY cluster, notified_at DESC) t
	WHERE notified_at > NOW() - $1::INTERVAL ;
	`, types.NotificationBackendTarget, orgs, clusters)

	rows := sqlmock.NewRows(
		[]string{"org_id", "cluster", "report", "notified_at"}).
		AddRow(1, "first cluster", "test", now).
		AddRow(1, "first cluster", "test", now)

	mock.ExpectQuery(regexp.QuoteMeta(expectedQuery)).
		WithArgs(timeOffset).
		WillReturnRows(rows)

	records, err := sut.ReadLastNotifiedRecordForClusterList(
		clusterEntries, timeOffset, types.NotificationBackendTarget)
	assert.NoError(t, err, "error running ReadLastNotifiedRecordForClusterList")
	fmt.Println(records)

	// If timeOffset is 0 or empty string, the WHERE clause is not included
	expectedQuery = fmt.Sprintf(`
	SELECT org_id, cluster, report, notified_at
	FROM (
		SELECT DISTINCT ON (cluster) *
		FROM reported
		WHERE event_type_id = %v AND state = 1 AND org_id IN (%v) AND cluster IN (%v)
		ORDER BY cluster, notified_at DESC) t ;
	`, types.NotificationBackendTarget, orgs, clusters)

	mock.ExpectQuery(regexp.QuoteMeta(expectedQuery)).WillReturnRows(rows)
	_, err = sut.ReadLastNotifiedRecordForClusterList(
		clusterEntries, timeOffsetNotSet, types.NotificationBackendTarget)
	assert.NoError(t, err, "unexpected query")

	mock.ExpectQuery(regexp.QuoteMeta(expectedQuery)).WillReturnRows(rows)
	_, err = sut.ReadLastNotifiedRecordForClusterList(
		clusterEntries, timeOffsetSetToZero, types.NotificationBackendTarget)
	assert.NoError(t, err, "unexpected query")

	mock.ExpectQuery(regexp.QuoteMeta(expectedQuery)).WillReturnRows(rows)
	_, err = sut.ReadLastNotifiedRecordForClusterList(
		clusterEntries, timeOffsetSetToZeroX, types.NotificationBackendTarget)
	assert.NoError(t, err, "unexpected query")

	mock.ExpectQuery(regexp.QuoteMeta(expectedQuery)).WillReturnRows(rows)
	_, err = sut.ReadLastNotifiedRecordForClusterList(
		clusterEntries, timeOffsetEmptySpace, types.NotificationBackendTarget)
	assert.NoError(t, err, "unexpected query")
}

func newMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	return db, mock
}

// Test the checkArgs function when flag for --show-version is set
func TestInClauseFromSlice(t *testing.T) {
	stringSlice := make([]string, 0)
	assert.Equal(t, "", differ.InClauseFromStringSlice(stringSlice))

	stringSlice = []string{"first item", "second item"}
	assert.Equal(t, "'first item','second item'", differ.InClauseFromStringSlice(stringSlice))
}

// TestReadErrorExistPositiveResult checks if Storage.ReadErrorExists returns
// expected results (positive test).
func TestReadErrorExistPositiveResult(t *testing.T) {
	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// prepare mocked result for SQL query
	rows := sqlmock.NewRows([]string{"exists"})
	rows.AddRow(true)

	// expected query performed by tested function
	expectedQuery := "SELECT exists\\(SELECT 1 FROM read_errors WHERE org_id=\\$1 and cluster=\\$2 and updated_at=\\$3\\);"

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	exists, err := storage.ReadErrorExists(1, "123", time.Now())
	if err != nil {
		t.Error("error was not expected while querying read_errors table", err)
	}

	assert.True(t, exists, "True return value is expected")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestReadErrorExistNegativeResult checks if Storage.ReadErrorExists returns
// expected results (positive test).
func TestReadErrorExistNegativeResult(t *testing.T) {
	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// prepare mocked result for SQL query
	rows := sqlmock.NewRows([]string{"exists"})
	rows.AddRow(false)

	// expected query performed by tested function
	expectedQuery := "SELECT exists\\(SELECT 1 FROM read_errors WHERE org_id=\\$1 and cluster=\\$2 and updated_at=\\$3\\);"

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	exists, err := storage.ReadErrorExists(1, "123", time.Now())
	if err != nil {
		t.Error("error was not expected while querying read_errors table", err)
	}

	assert.False(t, exists, "False return value is expected")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestWriteReadError function checks the method
// Storage.WriteReadError.
func TestWriteReadError(t *testing.T) {
	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// expected query performed by tested function
	expectedStatement := "INSERT INTO read_errors\\(org_id, cluster, updated_at, created_at, error_text\\) VALUES \\(\\$1, \\$2, \\$3, \\$4, \\$5\\);"

	mock.ExpectExec(expectedStatement).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	err := storage.WriteReadError(1, "foo", time.Now(), errors.New("my error"))
	if err != nil {
		t.Errorf("error was not expected while writing report for cluster: %s", err)
	}

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestWriteReadErrorOnError function checks the method
// Storage.WriteReadError.
func TestWriteReadErrorOnError(t *testing.T) {
	// error to be thrown
	mockedError := errors.New("mocked error")

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// expected query performed by tested function
	expectedStatement := "INSERT INTO read_errors\\(org_id, cluster, updated_at, created_at, error_text\\) VALUES \\(\\$1, \\$2, \\$3, \\$4, \\$5\\);"

	mock.ExpectExec(expectedStatement).WillReturnError(mockedError)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	err := storage.WriteReadError(1, "foo", time.Now(), errors.New("my error"))
	if err == nil {
		t.Errorf("error was expected while writing error report: %s", err)
	}

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestWriteReadErrorWrongDriver function checks the method
// Storage.WriteReadError.
func TestWriteReadErrorWrongDriver(t *testing.T) {
	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// expected database operations
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, wrongDatabaseDriver)

	// call the tested method
	err := storage.WriteReadError(1, "foo", time.Now(), errors.New("my error"))
	if err == nil {
		t.Errorf("error was expected while writing error report: %s", err)
	}

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}
