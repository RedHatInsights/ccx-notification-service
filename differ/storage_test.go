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
	"strconv"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"

	"github.com/RedHatInsights/ccx-notification-service/conf"
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

// TestNewStorageError checks whether constructor for new storage returns error for improper storage configuration
func TestNewStorageError(t *testing.T) {
	_, err := differ.NewStorage(&conf.StorageConfiguration{
		Driver: "non existing driver",
	})
	assert.EqualError(t, err, "driver non existing driver is not supported")
}

// TestNewStorageNoType checks whether constructor for new storage returns error for improper storage configuration
func TestNewStorageNoType(t *testing.T) {
	_, err := differ.NewStorage(&conf.StorageConfiguration{
		Driver: "",
	})
	assert.EqualError(t, err, "driver  is not supported")
}

// TestNewStorageWithLogging tests creating new storage with logs
func TestNewStorageWithLogging(t *testing.T) {
	_, err := differ.NewStorage(&conf.StorageConfiguration{
		Driver:        "postgres",
		PGPort:        1234,
		PGUsername:    "user",
		LogSQLQueries: true,
	})

	assert.NoError(t, err, "error retrieving new storage")
}

// TestNewStorageReturnedImplementation check what implementation of storage is returnd
func TestNewStorageReturnedImplementation(t *testing.T) {
	s, _ := differ.NewStorage(&conf.StorageConfiguration{
		Driver:        "postgres",
		PGPort:        1234,
		PGUsername:    "user",
		LogSQLQueries: true,
	})
	assert.IsType(t, &differ.DBStorage{}, s)
}

// TestReadLastNotifiedRecordForClusterListEmptyClusterEntries test checks how
// empty sequence of cluster entries is handled by metohd
// ReadLastNotifiedRecordForClusterList
func TestReadLastNotifiedRecordForClusterListEmptyClusterEntries(t *testing.T) {
	// empty sequence of cluster entries
	clusterEntries := []types.ClusterEntry{}

	// second parameter passed to tested method
	timeOffset := "1 day"

	// prepare database mock
	db, _ := newMock(t)
	defer func() { _ = db.Close() }()

	// establish connection to mocked database
	sut := differ.NewFromConnection(db, types.DBDriverPostgres)

	// call tested method
	records, err := sut.ReadLastNotifiedRecordForClusterList(
		clusterEntries, timeOffset, types.NotificationBackendTarget)

	// test returned values
	assert.NoError(t, err, "error running ReadLastNotifiedRecordForClusterList")
	assert.Len(t, records, 0, "empty output is expected")

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
	assert.NoError(t, err, "error was not expected while querying read_errors table")

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
	assert.NoError(t, err, "error was not expected while querying read_errors table")

	assert.False(t, exists, "False return value is expected")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestReadErrorExistNothingFound checks if Storage.ReadErrorExists returns
// expected results (nothing has been found in table).
func TestReadErrorExistNothingFound(t *testing.T) {
	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// prepare mocked result for SQL query
	rows := sqlmock.NewRows([]string{"exists"})

	// expected query performed by tested function
	expectedQuery := "SELECT exists\\(SELECT 1 FROM read_errors WHERE org_id=\\$1 and cluster=\\$2 and updated_at=\\$3\\);"

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	exists, err := storage.ReadErrorExists(1, "123", time.Now())

	// error is expected to be returned from called method
	assert.Error(t, err, "error was expected while querying read_errors table")

	assert.False(t, exists, "False return value is expected")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestReadErrorExistOnScanError checks if Storage.ReadErrorExists returns
// expected results on scan error
func TestReadErrorOnScanError(t *testing.T) {
	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// prepare mocked result for SQL query
	rows := sqlmock.NewRows([]string{"exists"})
	rows.AddRow("this is not a boolean value")

	// expected query performed by tested function
	expectedQuery := "SELECT exists\\(SELECT 1 FROM read_errors WHERE org_id=\\$1 and cluster=\\$2 and updated_at=\\$3\\);"

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	_, err := storage.ReadErrorExists(1, "123", time.Now())

	// error is expected to be returned from called method
	assert.Error(t, err, "an error is expected while scanning read_errors table")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestReadErrorExistOnError checks if Storage.ReadErrorExists returns
// expected results on query error
func TestReadErrorOnError(t *testing.T) {
	// error to be thrown
	mockedError := errors.New("mocked error")

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// expected query performed by tested function
	expectedQuery := "SELECT exists\\(SELECT 1 FROM read_errors WHERE org_id=\\$1 and cluster=\\$2 and updated_at=\\$3\\);"

	// let's raise an error!
	mock.ExpectQuery(expectedQuery).WillReturnError(mockedError)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	_, err := storage.ReadErrorExists(1, "123", time.Now())

	// error is expected to be returned from called method
	assert.Error(t, err, "an error is expected while querying read_errors table")

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
	assert.NoError(t, err, "error was not expected while writing report for cluster")

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

	// error is expected to be returned from called method
	assert.Error(t, err, "error was expected while writing error report")

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

	// error is expected to be returned from called method
	assert.Error(t, err, "error was expected while writing error report")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestReadStatesEmptyRecordSet checks if method Storage.ReadStates returns
// empty record set.
func TestReadStatesEmptyRecordSet(t *testing.T) {
	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// prepare mocked result for SQL query
	rows := sqlmock.NewRows([]string{"id", "value", "comment"})

	// expected query performed by tested function
	expectedQuery := "SELECT id, value, comment FROM states ORDER BY id"

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	states, err := storage.ReadStates()

	// tested method should NOT return an error
	assert.NoError(t, err, "error was not expected while querying states table")

	// no states should be returned
	assert.Empty(t, states, "Set of states should be empty")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestReadStatesNonEmptyRecordSet checks if method Storage.ReadStates returns
// non empty record set.
func TestReadStatesNonEmptyRecordSet(t *testing.T) {
	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// prepare mocked result for SQL query
	rows := sqlmock.NewRows([]string{"id", "value", "comment"})

	// these three rows should be returned
	rows.AddRow(0, 1000, "ID=0")
	rows.AddRow(1, 2000, "ID=1")
	rows.AddRow(2, 3000, "ID=2")

	// expected query performed by tested function
	expectedQuery := "SELECT id, value, comment FROM states ORDER BY id"

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	states, err := storage.ReadStates()

	// tested method should NOT return an error
	assert.NoError(t, err, "error was not expected while querying states table")

	// exactly three states should be returned
	assert.Len(t, states, 3, "Exactly 3 states should be returned")

	// check returned result set values
	for i := 0; i < 3; i++ {
		assert.Equal(t, states[i].ID, types.StateID(i))
		assert.Equal(t, states[i].Value, strconv.Itoa((i+1)*1000))
	}

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestReadStatesOnScanError checks if method Storage.ReadStates returns
// expected results on scan error.
func TestReadStatesOnScanError(t *testing.T) {
	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// prepare mocked result for SQL query
	rows := sqlmock.NewRows([]string{"id", "value", "comment"})

	// these three rows should be returned
	rows.AddRow("this is not integer!", 1000, "ID=0")
	rows.AddRow(1, 2000, "ID=1")
	rows.AddRow(2, 3000, "ID=2")

	// expected query performed by tested function
	expectedQuery := "SELECT id, value, comment FROM states ORDER BY id"

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	states, err := storage.ReadStates()

	// tested method SHOULD return an error
	assert.Error(t, err, "an error is expected while scanning states table")

	// no states should be returned
	assert.Empty(t, states, "Set of states should be empty")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestReadStatesOnError checks if method Storage.ReadStates returns
// expected results on query error.
func TestReadStatesOnError(t *testing.T) {
	// error to be thrown
	mockedError := errors.New("mocked error")

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// expected query performed by tested function
	expectedQuery := "SELECT id, value, comment FROM states ORDER BY id"

	// let's raise an error!
	mock.ExpectQuery(expectedQuery).WillReturnError(mockedError)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	states, err := storage.ReadStates()

	// tested method SHOULD return an error
	assert.Error(t, err, "an error is expected while quering states table")

	// no states should be returned
	assert.Empty(t, states, "Set of states should be empty")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestReadClusterListEmptyRecordSet checks if method Storage.ReadClusterList
// returns empty record set.
func TestReadClusterListEmptyRecordSet(t *testing.T) {
	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// prepare mocked result for SQL query
	rows := sqlmock.NewRows([]string{
		"org_id",
		"account_number",
		"cluster",
		"kafka_offset",
		"updated_at"})

	// expected query performed by tested function
	expectedQuery := `
		SELECT DISTINCT ON \(cluster\)
		org_id, account_number, cluster, kafka_offset, updated_at
		FROM new_reports
		ORDER BY cluster, updated_at DESC`

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	clusterList, err := storage.ReadClusterList()

	// tested method should NOT return an error
	assert.NoError(t, err, "error was not expected while querying new_reports table")

	// no clusters tates should be returned
	assert.Empty(t, clusterList, "List of clusters should be empty")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestReadClusterListNonEmptyRecordSet checks if method Storage.ReadClusterList returns
// non empty record set.
func TestReadClusterListNonEmptyRecordSet(t *testing.T) {
	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// prepare mocked result for SQL query
	rows := sqlmock.NewRows([]string{
		"org_id",
		"account_number",
		"cluster",
		"kafka_offset",
		"updated_at"})

	// these three rows should be returned
	rows.AddRow(0, 1000, "cluster1", 10000, time.Now())
	rows.AddRow(1, 2000, "cluster2", 10001, time.Now())
	rows.AddRow(2, 3000, "cluster3", 10002, time.Now())

	// expected query performed by tested function
	expectedQuery := `
		SELECT DISTINCT ON \(cluster\)
		org_id, account_number, cluster, kafka_offset, updated_at
		FROM new_reports
		ORDER BY cluster, updated_at DESC`

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	clusterList, err := storage.ReadClusterList()

	// tested method should NOT return an error
	assert.NoError(t, err, "error was not expected while querying new_reports table")

	// exactly three clusters should be returned
	assert.Len(t, clusterList, 3, "Exactly 3 clusters should be returned")

	// check returned result set values
	for i := 0; i < 3; i++ {
		cluster := clusterList[i]
		assert.Equal(t, cluster.OrgID, types.OrgID(i))
		assert.Equal(t, cluster.AccountNumber, types.AccountNumber((i+1)*1000))
		assert.Equal(t, cluster.KafkaOffset, types.KafkaOffset(i+10000))
	}

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestReadClusterListOnScanError checks if method Storage.ReadClusterList returns
// expected results on scan error.
func TestReadClusterListOnScanError(t *testing.T) {
	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// prepare mocked result for SQL query
	// prepare mocked result for SQL query
	rows := sqlmock.NewRows([]string{
		"org_id",
		"account_number",
		"cluster",
		"kafka_offset",
		"updated_at"})

	// these three rows should be returned
	rows.AddRow("this is not integer!", 1000, "cluster1", 10000, time.Now())
	rows.AddRow(1, 2000, "cluster2", 10001, time.Now())
	rows.AddRow(2, 3000, "cluster3", 10002, time.Now())

	// expected query performed by tested function
	expectedQuery := `
		SELECT DISTINCT ON \(cluster\)
		org_id, account_number, cluster, kafka_offset, updated_at
		FROM new_reports
		ORDER BY cluster, updated_at DESC`

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	clusterList, err := storage.ReadClusterList()

	// tested method SHOULD return an error
	assert.Error(t, err, "an error is expected while querying new_reports table")

	// no clusters tates should be returned
	assert.Empty(t, clusterList, "List of clusters should be empty")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestReadClusterListOnError checks if method Storage.ReadClusterList returns
// expected results on query error.
func TestReadClusterListOnError(t *testing.T) {
	// error to be thrown
	mockedError := errors.New("mocked error")

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// expected query performed by tested function
	expectedQuery := `
		SELECT DISTINCT ON \(cluster\)
		org_id, account_number, cluster, kafka_offset, updated_at
		FROM new_reports
		ORDER BY cluster, updated_at DESC`

	// let's raise an error!
	mock.ExpectQuery(expectedQuery).WillReturnError(mockedError)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	states, err := storage.ReadClusterList()

	// tested method SHOULD return an error
	assert.Error(t, err, "an error is expected while quering new_reports table")

	// no clusters should be returned
	assert.Empty(t, states, "List of clusters should be empty")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestReadReportForCluster checks if method
// Storage.ReadReportForCluster returns correct output.
func TestReadReportForCluster(t *testing.T) {
	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// prepare mocked result for SQL query
	rows := sqlmock.NewRows([]string{
		"report",
		"updated_at"})

	// timestamp
	expectedTimestamp := time.Now()

	// report to be returned
	expectedReport := "this is mocked report"

	// only one result must be returned
	rows.AddRow(expectedReport, expectedTimestamp)

	// expected query performed by tested function
	expectedQuery := `
		SELECT report, updated_at
		  FROM new_reports
		 WHERE org_id = \$1 AND cluster = \$2
		 ORDER BY updated_at DESC
		 LIMIT 1
                `

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// parameters for tested method
	orgID := types.OrgID(42)
	clusterName := types.ClusterName("foo")

	// call the tested method
	returnedReport, returnedTimestamp, err := storage.ReadReportForCluster(orgID, clusterName)

	// tested method should NOT return an error
	assert.NoError(t, err, "error was not expected while querying new_reports table for given cluster")

	// check returned report and timestamp
	assert.Equal(t, returnedReport, types.ClusterReport(expectedReport))
	assert.Equal(t, returnedTimestamp, types.Timestamp(expectedTimestamp))

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestReadReportForClusterOnScanError checks if method
// Storage.ReadReportForCluster returns expected results on
// scan error.
func TestReadReportForClusterOnScanError(t *testing.T) {
	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// prepare mocked result for SQL query
	rows := sqlmock.NewRows([]string{
		"report",
		"updated_at"})

	// report to be returned
	expectedReport := "this is mocked report"

	// only one result must be returned
	rows.AddRow(expectedReport, "this is not a timestamp value")

	// expected query performed by tested function
	expectedQuery := `
		SELECT report, updated_at
		  FROM new_reports
		 WHERE org_id = \$1 AND cluster = \$2
		 ORDER BY updated_at DESC
		 LIMIT 1
                `

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// parameters for tested method
	orgID := types.OrgID(42)
	clusterName := types.ClusterName("foo")

	// call the tested method
	returnedReport, returnedTimestamp, err := storage.ReadReportForCluster(orgID, clusterName)

	// tested method SHOULD return an error
	assert.Error(t, err, "error SHOULD be thrown while querying new_reports table for given cluster")

	// check returned report and timestamp
	assert.Equal(t, returnedReport, types.ClusterReport(expectedReport))
	assert.True(t, time.Time(returnedTimestamp).IsZero())

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestReadReportForClusterOnError checks if method
// Storage.ReadReportForCluster returns expected results on
// query error.
func TestReadReportForClusterOnError(t *testing.T) {
	// error to be thrown
	mockedError := errors.New("mocked error")

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// expected query performed by tested function
	expectedQuery := `
		SELECT report, updated_at
		  FROM new_reports
		 WHERE org_id = \$1 AND cluster = \$2
		 ORDER BY updated_at DESC
		 LIMIT 1
                `

	// let's raise an error!
	mock.ExpectQuery(expectedQuery).WillReturnError(mockedError)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// parameters for tested method
	orgID := types.OrgID(42)
	clusterName := types.ClusterName("foo")

	// call the tested method
	returnedReport, returnedTimestamp, err := storage.ReadReportForCluster(orgID, clusterName)

	// tested method SHOULD return an error
	assert.Error(t, err, "error SHOULD be thrown while querying new_reports table for given cluster")

	// check returned report and timestamp
	assert.Empty(t, returnedReport)
	assert.True(t, time.Time(returnedTimestamp).IsZero())

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestReadReportForClusterAtOffset checks if method
// Storage.ReadReportForClusterAtOffset returns correct output.
func TestReadReportForClusterAtOffset(t *testing.T) {
	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// prepare mocked result for SQL query
	rows := sqlmock.NewRows([]string{
		"report"})

	// report to be returned
	expectedReport := "this is mocked report"

	// only one result must be returned
	rows.AddRow(expectedReport)

	// expected query performed by tested function
	expectedQuery := `
		SELECT report
		  FROM new_reports
		 WHERE org_id = \$1 AND cluster = \$2 AND kafka_offset = \$3;
                `

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// parameters for tested method
	orgID := types.OrgID(42)
	clusterName := types.ClusterName("foo")
	kafkaOffset := types.KafkaOffset(0)

	// call the tested method
	returnedReport, err := storage.ReadReportForClusterAtOffset(orgID, clusterName, kafkaOffset)

	// tested method should NOT return an error
	assert.NoError(t, err, "error was not expected while querying new_reports table for given cluster and offset")

	// check returned report
	assert.Equal(t, returnedReport, types.ClusterReport(expectedReport))

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestReadReportForClusterAtOffsetOnScanError checks if method
// Storage.ReadReportForClusterAtOffset returns expected results on
// scan error.
func TestReadReportForClusterAtOffsetOnScanError(t *testing.T) {
	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// prepare mocked result for SQL query
	rows := sqlmock.NewRows([]string{
		"report"})

	// report to be returned
	expectedReport := 42 // not a string

	// only one result must be returned
	rows.AddRow(expectedReport)

	// expected query performed by tested function
	expectedQuery := `
		SELECT report
		  FROM new_reports
		 WHERE org_id = \$1 AND cluster = \$2 AND kafka_offset = \$3;
                `

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// parameters for tested method
	orgID := types.OrgID(42)
	clusterName := types.ClusterName("foo")
	kafkaOffset := types.KafkaOffset(0)

	// call the tested method
	returnedReport, err := storage.ReadReportForClusterAtOffset(orgID, clusterName, kafkaOffset)

	// tested method SHOULD return an error
	assert.Error(t, err, "error SHOULD be thrown while querying new_reports table for given cluster and offset")

	// check returned report
	assert.Empty(t, returnedReport)

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestReadReportForClusterAtOffsetOnError checks if method
// Storage.ReadReportForClusterAtOffset returns expected results on
// query error.
func TestReadReportForClusterAtOffsetOnError(t *testing.T) {
	// error to be thrown
	mockedError := errors.New("mocked error")

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// expected query performed by tested function
	expectedQuery := `
		SELECT report
		  FROM new_reports
		 WHERE org_id = \$1 AND cluster = \$2 AND kafka_offset = \$3;
                `

	// let's raise an error!
	mock.ExpectQuery(expectedQuery).WillReturnError(mockedError)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// parameters for tested method
	orgID := types.OrgID(42)
	clusterName := types.ClusterName("foo")
	kafkaOffset := types.KafkaOffset(0)

	// call the tested method
	returnedReport, err := storage.ReadReportForClusterAtOffset(orgID, clusterName, kafkaOffset)

	// tested method SHOULD return an error
	assert.Error(t, err, "error SHOULD be thrown while querying new_reports table for given cluster and offset")

	// check returned report
	assert.Empty(t, returnedReport)

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestReadReportForClusterAtTime checks if method
// Storage.ReadReportForClusterAtTime returns correct output.
func TestReadReportForClusterAtTime(t *testing.T) {
	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// prepare mocked result for SQL query
	rows := sqlmock.NewRows([]string{
		"report"})

	// report to be returned
	expectedReport := "this is mocked report"

	// only one result must be returned
	rows.AddRow(expectedReport)

	// expected query performed by tested function
	expectedQuery := `
		SELECT report
		  FROM new_reports
		 WHERE org_id = \$1 AND cluster = \$2 AND updated_at = \$3;
                `

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// parameters for tested method
	orgID := types.OrgID(42)
	clusterName := types.ClusterName("foo")
	updatedAt := types.Timestamp(time.Now())

	// call the tested method
	returnedReport, err := storage.ReadReportForClusterAtTime(orgID, clusterName, updatedAt)

	// tested method should NOT return an error
	assert.NoError(t, err, "error was not expected while querying new_reports table for given cluster and timestamp")

	// check returned report
	assert.Equal(t, returnedReport, types.ClusterReport(expectedReport))

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestReadReportForClusterAtTimeOnScanError checks if method
// Storage.ReadReportForClusterAtTime returns expected results on
// scan error.
func TestReadReportForClusterAtTimeOnScanError(t *testing.T) {
	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// prepare mocked result for SQL query
	rows := sqlmock.NewRows([]string{
		"report"})

	// report to be returned
	expectedReport := 42 // not a string

	// only one result must be returned
	rows.AddRow(expectedReport)

	// expected query performed by tested function
	expectedQuery := `
		SELECT report
		  FROM new_reports
		 WHERE org_id = \$1 AND cluster = \$2 AND updated_at = \$3;
                `

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// parameters for tested method
	orgID := types.OrgID(42)
	clusterName := types.ClusterName("foo")
	updatedAt := types.Timestamp(time.Now())

	// call the tested method
	returnedReport, err := storage.ReadReportForClusterAtTime(orgID, clusterName, updatedAt)

	// tested method SHOULD return an error
	assert.Error(t, err, "error SHOULD be thrown while querying new_reports table for given cluster and timestamp")

	// check returned report
	assert.Empty(t, returnedReport)

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestReadReportForClusterAtTimeOnError checks if method
// Storage.ReadReportForClusterAtTime returns expected results on
// query error.
func TestReadReportForClusterAtTimeOnError(t *testing.T) {
	// error to be thrown
	mockedError := errors.New("mocked error")

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// expected query performed by tested function
	expectedQuery := `
		SELECT report
		  FROM new_reports
		 WHERE org_id = \$1 AND cluster = \$2 AND updated_at = \$3;
                `

	// let's raise an error!
	mock.ExpectQuery(expectedQuery).WillReturnError(mockedError)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// parameters for tested method
	orgID := types.OrgID(42)
	clusterName := types.ClusterName("foo")
	updatedAt := types.Timestamp(time.Now())

	// call the tested method
	returnedReport, err := storage.ReadReportForClusterAtTime(orgID, clusterName, updatedAt)

	// tested method SHOULD return an error
	assert.Error(t, err, "error SHOULD be thrown while querying new_reports table for given cluster and timestamp")

	// check returned report
	assert.Empty(t, returnedReport)

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestCleanup function checks the method Storage.Cleanup.
func TestCleanup(t *testing.T) {
	const cleanupStatement = "DELETE FROM foo"
	const maxAge = "1 day"

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	mock.ExpectExec(cleanupStatement).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	affected, err := storage.Cleanup(maxAge, cleanupStatement)
	assert.Equal(t, affected, 1)
	assert.NoError(t, err, "error was not expected while cleaning operation")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestCleanupOnError function checks the method Storage.Cleanup when error is
// detected during cleanup operation.
func TestCleanupOnError(t *testing.T) {
	const cleanupStatement = "DELETE FROM foo"
	const maxAge = "1 day"

	// error to be thrown
	mockedError := errors.New("mocked error")

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	mock.ExpectExec(cleanupStatement).WillReturnError(mockedError)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	_, err := storage.Cleanup(maxAge, cleanupStatement)
	assert.Error(t, err, "error was expected while writing error report")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestCleanupNewReports function checks the method
// Storage.CleanupNewReports.
func TestCleanupNewReports(t *testing.T) {
	const cleanupStatement = "DELETE FROM new_reports WHERE updated_at < NOW\\(\\) - \\$1::INTERVAL"
	const maxAge = "1 day"

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	mock.ExpectExec(cleanupStatement).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	affected, err := storage.CleanupNewReports(maxAge)
	assert.Equal(t, affected, 1)
	assert.NoError(t, err, "error was not expected while cleaning operation")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestCleanupNewReportsOnError function checks the method Storage.CleanupNewReports when error is
// detected during cleanup operation.
func TestCleanupNewReportsOnError(t *testing.T) {
	const cleanupStatement = "DELETE FROM new_reports WHERE updated_at < NOW\\(\\) - \\$1::INTERVAL"
	const maxAge = "1 day"

	// error to be thrown
	mockedError := errors.New("mocked error")

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	mock.ExpectExec(cleanupStatement).WillReturnError(mockedError)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	_, err := storage.CleanupNewReports(maxAge)
	assert.Error(t, err, "error was expected while writing error report")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestCleanupOldReports function checks the method
// Storage.CleanupOldReports.
func TestCleanupOldReports(t *testing.T) {
	const cleanupStatement = "DELETE FROM reported WHERE updated_at < NOW\\(\\) - \\$1::INTERVAL"
	const maxAge = "1 day"

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	mock.ExpectExec(cleanupStatement).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	affected, err := storage.CleanupOldReports(maxAge)
	assert.Equal(t, affected, 1)
	assert.NoError(t, err, "error was not expected while cleaning operation")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestCleanupOldReportsOnError function checks the method Storage.CleanupOldReports when error is
// detected during cleanup operation.
func TestCleanupOldReportsOnError(t *testing.T) {
	const cleanupStatement = "DELETE FROM reported WHERE updated_at < NOW\\(\\) - \\$1::INTERVAL"
	const maxAge = "1 day"

	// error to be thrown
	mockedError := errors.New("mocked error")

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	mock.ExpectExec(cleanupStatement).WillReturnError(mockedError)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	_, err := storage.CleanupOldReports(maxAge)
	assert.Error(t, err, "error was expected while writing error report")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestDeleteRowFromNewReports function checks the method
// Storage.DeleteRowFromNewReports.
func TestDeleteRowFromNewReports(t *testing.T) {
	const cleanupStatement = "DELETE FROM new_reports WHERE org_id = \\$1 AND cluster = \\$2 AND updated_at = \\$3"
	const orgID = 2
	const clusterName = "00000000-0000-0000-0000-000000000000"

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	mock.ExpectExec(cleanupStatement).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	affected, err := storage.DeleteRowFromNewReports(types.OrgID(orgID), types.ClusterName(clusterName), types.Timestamp(time.Now()))
	assert.Equal(t, affected, 1)
	assert.NoError(t, err, "error was not expected while deleting one row")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestDeleteRowFromNewReportsOnError function checks the method
// Storage.DeleteRowFromNewReports.
func TestDeleteRowFromNewReportsOnError(t *testing.T) {
	const cleanupStatement = "DELETE FROM new_reports WHERE org_id = \\$1 AND cluster = \\$2 AND updated_at = \\$3"
	const orgID = 2
	const clusterName = "00000000-0000-0000-0000-000000000000"

	// error to be thrown
	mockedError := errors.New("mocked error")

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	mock.ExpectExec(cleanupStatement).WillReturnError(mockedError)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	_, err := storage.DeleteRowFromNewReports(types.OrgID(orgID), types.ClusterName(clusterName), types.Timestamp(time.Now()))
	assert.Error(t, err, "error was expected while deleting one row")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestDeleteRowFromReported function checks the method
// Storage.DeleteRowFromReported.
func TestDeleteRowFromReported(t *testing.T) {
	const cleanupStatement = "DELETE   FROM reported  WHERE org_id = \\$1    AND cluster = \\$2    AND notified_at = \\$3"
	const orgID = 2
	const clusterName = "00000000-0000-0000-0000-000000000000"

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	mock.ExpectExec(cleanupStatement).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	affected, err := storage.DeleteRowFromReported(types.OrgID(orgID), types.ClusterName(clusterName), types.Timestamp(time.Now()))
	assert.Equal(t, affected, 1)
	assert.NoError(t, err, "error was not expected while deleting one row")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestDeleteRowFromReportedOnError function checks the method
// Storage.DeleteRowFromReported.
func TestDeleteRowFromReportedOnError(t *testing.T) {
	const cleanupStatement = "DELETE   FROM reported  WHERE org_id = \\$1    AND cluster = \\$2    AND notified_at = \\$3"
	const orgID = 2
	const clusterName = "00000000-0000-0000-0000-000000000000"

	// error to be thrown
	mockedError := errors.New("mocked error")

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	mock.ExpectExec(cleanupStatement).WillReturnError(mockedError)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	_, err := storage.DeleteRowFromReported(types.OrgID(orgID), types.ClusterName(clusterName), types.Timestamp(time.Now()))
	assert.Error(t, err, "error was expected while deleting one row")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestReadNotificationTypesEmptyRecordSet checks if method Storage.ReadNotificationTypes returns
// empty record set.
func TestReadNotificationTypesEmptyRecordSet(t *testing.T) {
	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// prepare mocked result for SQL query
	rows := sqlmock.NewRows([]string{"id", "value", "frequency", "comment"})

	// expected query performed by tested function
	expectedQuery := "SELECT id, value, frequency, comment FROM notification_types ORDER BY id"

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	notificationTypes, err := storage.ReadNotificationTypes()

	// tested method should NOT return an error
	assert.NoError(t, err, "error was not expected while querying notification types")

	// no notification types should be returned
	assert.Empty(t, notificationTypes, "Set of states should be empty")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestReadNotificationTypesNonEmptyRecordSet checks if method Storage.ReadNotificationTypes returns
// non empty record set.
func TestReadNotificationTypesNonEmptyRecordSet(t *testing.T) {
	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// prepare mocked result for SQL query
	rows := sqlmock.NewRows([]string{"id", "value", "frequency", "comment"})

	// these three rows should be returned
	rows.AddRow(0, 1000, 3, "ID=0")
	rows.AddRow(1, 2000, 3, "ID=1")
	rows.AddRow(2, 3000, 3, "ID=2")

	// expected query performed by tested function
	expectedQuery := "SELECT id, value, frequency, comment FROM notification_types ORDER BY id"

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	notificationTypes, err := storage.ReadNotificationTypes()

	// tested method should NOT return an error
	assert.NoError(t, err, "error was not expected while querying notification types")

	// exactly three notification types should be returned
	assert.Len(t, notificationTypes, 3, "Exactly 3 notification types should be returned")

	// check returned result set values
	for i := 0; i < 3; i++ {
		assert.Equal(t, int(notificationTypes[i].ID), i)
		assert.Equal(t, notificationTypes[i].Value, strconv.Itoa((i+1)*1000))
	}

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestReadNotificationTypesOnScanError checks if method Storage.ReadNotificationTypes returns
// expected results on scan error.
func TestReadNotificationTypesOnScanError(t *testing.T) {
	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// prepare mocked result for SQL query
	rows := sqlmock.NewRows([]string{"id", "value", "frequency", "comment"})

	// these three rows should be returned
	rows.AddRow("this is not integer!", 1000, 3, "ID=0")
	rows.AddRow(1, 2000, 3, "ID=1")
	rows.AddRow(2, 3000, 3, "ID=2")

	// expected query performed by tested function
	expectedQuery := "SELECT id, value, frequency, comment FROM notification_types ORDER BY id"

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	notificationTypes, err := storage.ReadNotificationTypes()

	// tested method SHOULD return an error
	assert.Error(t, err, "an error is expected while scanning states table")

	// no states should be returned
	assert.Empty(t, notificationTypes, "Set of notification types should be empty")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestReadNotificationTypesOnError checks if method Storage.ReadNotificationTypes returns
// expected results on query error.
func TestReadNotificationTypesOnError(t *testing.T) {
	// error to be thrown
	mockedError := errors.New("mocked error")

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// expected query performed by tested function
	expectedQuery := "SELECT id, value, frequency, comment FROM notification_types ORDER BY id"

	// let's raise an error!
	mock.ExpectQuery(expectedQuery).WillReturnError(mockedError)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	notificationTypes, err := storage.ReadNotificationTypes()

	// tested method SHOULD return an error
	assert.Error(t, err, "an error is expected while quering notification types table")

	// no states should be returned
	assert.Empty(t, notificationTypes, "Set of notification types should be empty")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestPrintNewReportsEmptyRecordSet checks if method Storage.PrintNewReports performs
// the right query when empty set is returned.
func TestPrintNewReportsEmptyRecordSet(t *testing.T) {
	const maxAge = "1 day"
	const tableName = "foo"

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// prepare mocked result for SQL query
	rows := sqlmock.NewRows([]string{"org_id", "account_number", "cluster_name", "updated_at", "kafka_offset"})

	// expected query performed by tested function
	expectedQuery := "SELECT id, value, FROM foo ORDER BY id"

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	err := storage.PrintNewReports(maxAge, expectedQuery, tableName)

	// tested method should NOT return an error
	assert.NoError(t, err, "error was not expected while querying database")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestPrintNewReportsNonEmptyRecordSet checks if method Storage.PrintNewReports performs
// the right query when non empty set is returned.
func TestPrintNewReportsNonEmptyRecordSet(t *testing.T) {
	const maxAge = "1 day"
	const tableName = "foo"

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// prepare mocked result for SQL query
	rows := sqlmock.NewRows([]string{"org_id", "account_number", "cluster_name", "updated_at", "kafka_offset"})

	// these three rows should be returned
	rows.AddRow(0, 1000, "ID=0", time.Now(), 0)
	rows.AddRow(1, 1001, "ID=1", time.Now(), 0)
	rows.AddRow(2, 1002, "ID=2", time.Now(), 0)

	// expected query performed by tested function
	expectedQuery := "SELECT id, value, FROM foo ORDER BY id"

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	err := storage.PrintNewReports(maxAge, expectedQuery, tableName)

	// tested method should NOT return an error
	assert.NoError(t, err, "error was not expected while querying database")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestPrintNewReportsOnScanError checks if method Storage.PrintNewReports performs
// the test for query errors.
func TestPrintNewReportsOnScanError(t *testing.T) {
	const maxAge = "1 day"
	const tableName = "foo"

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// prepare mocked result for SQL query
	rows := sqlmock.NewRows([]string{"org_id", "account_number", "cluster_name", "updated_at", "kafka_offset"})

	// these three rows should be returned
	rows.AddRow(0, "not a number!", "ID=0", time.Now(), 0)
	rows.AddRow(1, "not a number!", "ID=1", time.Now(), 0)
	rows.AddRow(2, "not a number!", "ID=2", time.Now(), 0)

	// expected query performed by tested function
	expectedQuery := "SELECT id, value, FROM foo ORDER BY id"

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	err := storage.PrintNewReports(maxAge, expectedQuery, tableName)

	// tested method should return an error
	assert.Error(t, err, "error was expected while querying database")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestPrintNewReportsOnError checks if method Storage.ReadNotificationTypes returns
// expected results on error.
func TestPrintNewReportsOnError(t *testing.T) {
	const maxAge = "1 day"
	const tableName = "foo"

	// error to be thrown
	mockedError := errors.New("mocked error")

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// expected query performed by tested function
	expectedQuery := "SELECT id, value, frequency, comment FROM notification_types ORDER BY id"

	// let's raise an error!
	mock.ExpectQuery(expectedQuery).WillReturnError(mockedError)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	err := storage.PrintNewReports(maxAge, expectedQuery, tableName)

	// tested method SHOULD return an error
	assert.Error(t, err, "an error is expected while quering notification types table")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestStorageClose tests method Storage.Close
func TestStorageClose(t *testing.T) {
	storage, err := differ.NewStorage(&conf.StorageConfiguration{
		Driver:        "postgres",
		PGPort:        1234,
		PGUsername:    "user",
		LogSQLQueries: true,
	})

	assert.NoError(t, err, "error retrieving new storage")

	err = storage.Close()
	assert.NoError(t, err, "error closing storage")
}

// TestPrintNewReportsForCleanupEmptyRecordSet checks if method Storage.PrintNewReportsForCleanup performs
// the right query when empty set is returned.
func TestPrintNewReportsForCleanupEmptyRecordSet(t *testing.T) {
	const maxAge = "1 day"

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// prepare mocked result for SQL query
	rows := sqlmock.NewRows([]string{"org_id", "account_number", "cluster_name", "updated_at", "kafka_offset"})

	// expected query performed by tested function
	expectedQuery := "SELECT org_id, account_number, cluster, updated_at, kafka_offset\n\t\t  FROM new_reports\n\t\t WHERE updated_at < NOW\\(\\) - \\$1::INTERVAL\n\t\t ORDER BY updated_at\n"

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	err := storage.PrintNewReportsForCleanup(maxAge)

	// tested method should NOT return an error
	assert.NoError(t, err, "error was not expected while querying database")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestPrintNewReportsForCleanupNonEmptyRecordSet checks if method Storage.PrintNewReportsForCleanup performs
// the right query when non empty set is returned.
func TestPrintNewReportsForCleanupNonEmptyRecordSet(t *testing.T) {
	const maxAge = "1 day"

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

	// call the tested method
	err := storage.PrintNewReportsForCleanup(maxAge)

	// tested method should NOT return an error
	assert.NoError(t, err, "error was not expected while querying database")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestPrintNewReportsForCleanupOnScanError checks if method Storage.PrintNewReportsForCleanup performs
// the test for query errors.
func TestPrintNewReportsForCleanupOnScanError(t *testing.T) {
	const maxAge = "1 day"

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// prepare mocked result for SQL query
	rows := sqlmock.NewRows([]string{"org_id", "account_number", "cluster_name", "updated_at", "kafka_offset"})

	// these three rows should be returned
	rows.AddRow(0, "not a number!", "ID=0", time.Now(), 0)
	rows.AddRow(1, "not a number!", "ID=1", time.Now(), 0)
	rows.AddRow(2, "not a number!", "ID=2", time.Now(), 0)

	// expected query performed by tested function
	expectedQuery := "SELECT org_id, account_number, cluster, updated_at, kafka_offset\n\t\t  FROM new_reports\n\t\t WHERE updated_at < NOW\\(\\) - \\$1::INTERVAL\n\t\t ORDER BY updated_at\n"

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	err := storage.PrintNewReportsForCleanup(maxAge)

	// tested method should return an error
	assert.Error(t, err, "error was expected while querying database")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestPrintNewReportsForCleanupOnError checks if method Storage.PrintNewReportsForCleanup returns
// expected results on error.
func TestPrintNewReportsForCleanupOnError(t *testing.T) {
	const maxAge = "1 day"

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

	// call the tested method
	err := storage.PrintNewReportsForCleanup(maxAge)

	// tested method SHOULD return an error
	assert.Error(t, err, "an error is expected while quering notification types table")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestPrintOldReportsForCleanupEmptyRecordSet checks if method Storage.PrintOldReportsForCleanup performs
// the right query when empty set is returned.
func TestPrintOldReportsForCleanupEmptyRecordSet(t *testing.T) {
	const maxAge = "1 day"

	// prepare Old mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// prepare mocked result for SQL query
	rows := sqlmock.NewRows([]string{"org_id", "account_number", "cluster_name", "updated_at", "kafka_offset"})

	// expected query performed by tested function
	expectedQuery := "SELECT org_id, account_number, cluster, updated_at, 0\n\t\t  FROM reported\n\t\t WHERE updated_at < NOW\\(\\) - \\$1::INTERVAL\n\t\t ORDER BY updated_at\n"

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	err := storage.PrintOldReportsForCleanup(maxAge)

	// tested method should NOT return an error
	assert.NoError(t, err, "error was not expected while querying database")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestPrintOldReportsForCleanupNonEmptyRecordSet checks if method Storage.PrintOldReportsForCleanup performs
// the right query when non empty set is returned.
func TestPrintOldReportsForCleanupNonEmptyRecordSet(t *testing.T) {
	const maxAge = "1 day"

	// prepare Old mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// prepare mocked result for SQL query
	rows := sqlmock.NewRows([]string{"org_id", "account_number", "cluster_name", "updated_at", "kafka_offset"})

	// these three rows should be returned
	rows.AddRow(0, 1000, "ID=0", time.Now(), 0)
	rows.AddRow(1, 1001, "ID=1", time.Now(), 0)
	rows.AddRow(2, 1002, "ID=2", time.Now(), 0)

	// expected query performed by tested function
	expectedQuery := "SELECT org_id, account_number, cluster, updated_at, 0\n\t\t  FROM reported\n\t\t WHERE updated_at < NOW\\(\\) - \\$1::INTERVAL\n\t\t ORDER BY updated_at\n"

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	err := storage.PrintOldReportsForCleanup(maxAge)

	// tested method should NOT return an error
	assert.NoError(t, err, "error was not expected while querying database")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestPrintOldReportsForCleanupOnScanError checks if method Storage.PrintOldReportsForCleanup performs
// the test for query errors.
func TestPrintOldReportsForCleanupOnScanError(t *testing.T) {
	const maxAge = "1 day"

	// prepare Old mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// prepare mocked result for SQL query
	rows := sqlmock.NewRows([]string{"org_id", "account_number", "cluster_name", "updated_at", "kafka_offset"})

	// these three rows should be returned
	rows.AddRow(0, "not a number!", "ID=0", time.Now(), 0)
	rows.AddRow(1, "not a number!", "ID=1", time.Now(), 0)
	rows.AddRow(2, "not a number!", "ID=2", time.Now(), 0)

	// expected query performed by tested function
	expectedQuery := "SELECT org_id, account_number, cluster, updated_at, 0\n\t\t  FROM reported\n\t\t WHERE updated_at < NOW\\(\\) - \\$1::INTERVAL\n\t\t ORDER BY updated_at\n"

	mock.ExpectQuery(expectedQuery).WillReturnRows(rows)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	err := storage.PrintOldReportsForCleanup(maxAge)

	// tested method should return an error
	assert.Error(t, err, "error was expected while querying database")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestPrintOldReportsForCleanupOnError checks if method Storage.PrintOldReportsForCleanup returns
// expected results on error.
func TestPrintOldReportsForCleanupOnError(t *testing.T) {
	const maxAge = "1 day"

	// error to be thrown
	mockedError := errors.New("mocked error")

	// prepare Old mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// expected query performed by tested function
	expectedQuery := "SELECT org_id, account_number, cluster, updated_at, 0\n\t\t  FROM reported\n\t\t WHERE updated_at < NOW\\(\\) - \\$1::INTERVAL\n\t\t ORDER BY updated_at\n"

	// let's raise an error!
	mock.ExpectQuery(expectedQuery).WillReturnError(mockedError)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	err := storage.PrintOldReportsForCleanup(maxAge)

	// tested method SHOULD return an error
	assert.Error(t, err, "an error is expected while quering notification types table")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

func tryToWriteNotificationRecordImpl(storage *differ.DBStorage) error {
	// insert parameters
	orgID := types.OrgID(1)
	accountNumber := types.AccountNumber(2)
	clusterName := types.ClusterName("foo")
	notificationTypeID := types.NotificationTypeID(0)
	stateID := types.StateID(0)
	report := types.ClusterReport("")
	updatedAt := types.Timestamp(time.Now())
	notifiedAt := types.Timestamp(time.Now())
	errorLog := ""
	eventTarget := types.EventTarget(1)

	return storage.WriteNotificationRecordImpl(
		orgID, accountNumber, clusterName, notificationTypeID,
		stateID, report, updatedAt, notifiedAt, errorLog, eventTarget)
}

// expected query performed by tested function
const expectedStatementWriteNotificationReportImpl = "INSERT INTO reported \\(org_id, account_number, cluster, notification_type, state, report, updated_at, notified_at, error_log, event_type_id\\) VALUES \\(\\$1, \\$2, \\$3, \\$4, \\$5\\, \\$6\\, \\$7\\, \\$8\\, \\$9\\, \\$10\\)"

// TestWriteNotificationRecordImpl function checks the method
// Storage.WriteNotificationRecordImpl.
func TestWriteNotificationRecordImpl(t *testing.T) {

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	mock.ExpectExec(expectedStatementWriteNotificationReportImpl).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	err := tryToWriteNotificationRecordImpl(storage)
	assert.NoError(t, err, "error was not expected while writing report for cluster")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestWriteNotificationRecordImplOnError function checks the method
// Storage.WriteNotificationRecordImpl.
func TestWriteNotificationRecordImplOnError(t *testing.T) {
	// error to be thrown
	mockedError := errors.New("mocked error")

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	mock.ExpectExec(expectedStatementWriteNotificationReportImpl).WillReturnError(mockedError)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	err := tryToWriteNotificationRecordImpl(storage)

	// error is expected to be returned from called method
	assert.Error(t, err, "error was expected while writing error report")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestWriteNotificationRecordImplWrongDriver function checks the method
// Storage.WriteNotificationRecordImpl.
func TestWriteNotificationRecordImplWrongDriver(t *testing.T) {
	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// expected database operations
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, wrongDatabaseDriver)

	// call the tested method
	err := tryToWriteNotificationRecordImpl(storage)

	// error is expected to be returned from called method
	assert.Error(t, err, "error was expected while writing error report")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// expected query performed by tested function
const expectedStatementWriteNotificationReport = "INSERT INTO reported \\(org_id, account_number, cluster, notification_type, state, report, updated_at, notified_at, error_log, event_type_id\\) VALUES \\(\\$1, \\$2, \\$3, \\$4, \\$5\\, \\$6\\, \\$7\\, \\$8\\, \\$9\\, \\$10\\)"

// TestWriteNotificationRecord function checks the method
// Storage.WriteNotificationRecord.
func TestWriteNotificationRecord(t *testing.T) {
	// empty record to be stored in database
	notificationRecord := types.NotificationRecord{}

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	mock.ExpectExec(expectedStatementWriteNotificationReport).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	err := storage.WriteNotificationRecord(&notificationRecord)
	assert.NoError(t, err, "error was not expected while writing report for cluster")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestWriteNotificationRecordOnError function checks the method
// Storage.WriteNotificationRecord.
func TestWriteNotificationRecordOnError(t *testing.T) {
	// empty record to be stored in database
	notificationRecord := types.NotificationRecord{}

	// error to be thrown
	mockedError := errors.New("mocked error")

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	mock.ExpectExec(expectedStatementWriteNotificationReport).WillReturnError(mockedError)
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	err := storage.WriteNotificationRecord(&notificationRecord)

	// error is expected to be returned from called method
	assert.Error(t, err, "error was expected while writing error report")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

// TestWriteNotificationRecordWrongDriver function checks the method
// Storage.WriteNotificationRecord.
func TestWriteNotificationRecordWrongDriver(t *testing.T) {
	// empty record to be stored in database
	notificationRecord := types.NotificationRecord{}

	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	// expected database operations
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, wrongDatabaseDriver)

	// call the tested method
	err := storage.WriteNotificationRecord(&notificationRecord)

	// error is expected to be returned from called method
	assert.Error(t, err, "error was expected while writing error report")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}

func tryToWriteNotificationRecordForCluster(storage *differ.DBStorage) error {
	// insert parameters
	notificationTypeID := types.NotificationTypeID(0)
	stateID := types.StateID(0)
	report := types.ClusterReport("")
	notifiedAt := types.Timestamp(time.Now())
	errorLog := ""
	eventTarget := types.EventTarget(1)

	clusterEntry := types.ClusterEntry{
		OrgID:         types.OrgID(1),
		AccountNumber: types.AccountNumber(2),
		ClusterName:   types.ClusterName("foo"),
	}

	// call the tested function
	return storage.WriteNotificationRecordForCluster(clusterEntry,
		notificationTypeID, stateID, report, notifiedAt,
		errorLog, eventTarget)
}

// expected query performed by tested function
const expectedStatementWriteNotificationReportForCluster = "INSERT INTO reported \\(org_id, account_number, cluster, notification_type, state, report, updated_at, notified_at, error_log, event_type_id\\) VALUES \\(\\$1, \\$2, \\$3, \\$4, \\$5\\, \\$6\\, \\$7\\, \\$8\\, \\$9\\, \\$10\\)"

// TestWriteNotificationRecordForCluster function checks the method
// Storage.WriteNotificationRecordForCluster.
func TestWriteNotificationRecordForCluster(t *testing.T) {
	// prepare new mocked connection to database
	connection, mock := mustCreateMockConnection(t)

	mock.ExpectExec(expectedStatementWriteNotificationReportForCluster).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectClose()

	// prepare connection to mocked database
	storage := differ.NewFromConnection(connection, 1)

	// call the tested method
	err := tryToWriteNotificationRecordForCluster(storage)
	assert.NoError(t, err, "error was not expected while writing report for cluster")

	// connection to mocked DB needs to be closed properly
	checkConnectionClose(t, connection)

	// check if all expectations were met
	checkAllExpectations(t, mock)
}
