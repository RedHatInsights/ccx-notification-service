/*
Copyright © 2023 Pavel Tisnovsky

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

//
// Benchmarks to check what's the best PostgreSQL data type to store cluster
// name. From user perspective cluster names are represented as UUIDs strings
// that have the following display format:
//
// 123e4567-e89b-12d3-a456-426614174000
//
// There are four basic data types that can be used to store such names in PostgreSQL:
//     1) CHAR(36)
//     2) VARCHAR
//     3) BYTEA
//     4) UUID
//
// This benchmark checks which data type is best from performance perspective.
//
// For each data type, three benchmarks are run:
//     1) INSERTion into REPORTED table
//     2) DELETion from REPORTED table when reports are identified by cluster name
//     3) SELECTion from REPORTED table when reports are identified by cluster name
//

package main

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq" // PostgreSQL database driver

	"testing"
)

// SQL statements to create and drop tables used in benchmarks
const (
	// CreateTableReportedBenchmarkCharClusterID is SQL statement used to
	// create reported table where cluster name is represented as char(36)
	CreateTableReportedBenchmarkCharClusterID = `
		CREATE TABLE IF NOT EXISTS reported_benchmark_1 (
		    org_id            integer not null,
		    account_number    integer not null,
		    cluster           character(36) not null,
		    notification_type integer not null,
		    state             integer not null,
		    report            varchar not null,
		    updated_at        timestamp not null,
		    notified_at       timestamp not null,
		    error_log         varchar,

		    PRIMARY KEY (org_id, cluster, notified_at)
		);
		`

	// SQL statement used to drop (delete) first table used by benchmarks
	DropTableReportedBenchmarkCharClusterID = `
	        DROP TABLE IF EXISTS reported_benchmark_1;
        `

	// Index for the reported table used in benchmarks for
	// notified_at column
	CreateIndexReportedNotifiedAtDescV1 = `
                CREATE INDEX IF NOT EXISTS notified_at_desc_idx
		    ON reported_benchmark_1
		 USING btree (notified_at DESC);
        `

	// Index for the reported table used in benchmarks for
	// cluster column
	CreateIndexReportedClusterV1 = `
                CREATE INDEX IF NOT EXISTS reported_benchmark_1_cluster_idx
		    ON reported_benchmark_1
		 USING btree (cluster);
        `

	// Select cluster names from reported table
	SelectClusterNamesFromReportedV1Statement = `
            SELECT cluster FROM reported_benchmark_1
	`

	// Select report from reported table identified by cluster name
	SelectReportFromReportedV1Statement = `
            SELECT org_id, account_number, notification_type, state, report, updated_at, notified_at, error_log
	      FROM reported_benchmark_1
	     WHERE cluster=$1
	`

	// Insert one record into reported table
	InsertIntoReportedV1Statement = `
            INSERT INTO reported_benchmark_1
            (org_id, account_number, cluster, notification_type, state, report, updated_at, notified_at, error_log)
            VALUES
            ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	// Delete one record from reported table
	DeleteFromReportedV1Statement = `
            DELETE FROM reported_benchmark_1 WHERE cluster=$1
	`

	// CreateTableReportedBenchmarkVarcharClusterID is SQL statement used
	// to create reported table where cluster name is represented as
	// varchar(36)
	CreateTableReportedBenchmarkVarcharClusterID = `
		CREATE TABLE IF NOT EXISTS reported_benchmark_2 (
		    org_id            integer not null,
		    account_number    integer not null,
		    cluster           varchar(36) not null,
		    notification_type integer not null,
		    state             integer not null,
		    report            varchar not null,
		    updated_at        timestamp not null,
		    notified_at       timestamp not null,
		    error_log         varchar,

		    PRIMARY KEY (org_id, cluster, notified_at)
		);
		`

	// SQL statement used to drop (delete) second table used by benchmarks
	DropTableReportedBenchmarkVarcharClusterID = `
	        DROP TABLE IF EXISTS reported_benchmark_2;
        `
	// Index for the reported table used in benchmarks for
	// notified_at column
	CreateIndexReportedNotifiedAtDescV2 = `
                CREATE INDEX IF NOT EXISTS notified_at_desc_idx
		    ON reported_benchmark_2
		 USING btree (notified_at DESC);
        `

	// Index for the reported table used in benchmarks for
	// cluster column
	CreateIndexReportedClusterV2 = `
                CREATE INDEX IF NOT EXISTS reported_benchmark_2_cluster_idx
		    ON reported_benchmark_2
		 USING btree (cluster);
        `

	// Select cluster names from reported table
	SelectClusterNamesFromReportedV2Statement = `
            SELECT cluster FROM reported_benchmark_2
	`

	// Select report from reported table identified by cluster name
	SelectReportFromReportedV2Statement = `
            SELECT org_id, account_number, notification_type, state, report, updated_at, notified_at, error_log
	      FROM reported_benchmark_2
	     WHERE cluster=$1
	`

	// Insert one record into reported table
	InsertIntoReportedV2Statement = `
            INSERT INTO reported_benchmark_2
            (org_id, account_number, cluster, notification_type, state, report, updated_at, notified_at, error_log)
            VALUES
            ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	// Delete one record from reported table
	DeleteFromReportedV2Statement = `
            DELETE FROM reported_benchmark_2 WHERE cluster=$1
	`

	// CreateTableReportedBenchmarkByteArrayClusterID is SQL statement used
	// to create reported table where cluster name is represented as
	// BYTEA (byte array)
	CreateTableReportedBenchmarkByteArrayClusterID = `
		CREATE TABLE IF NOT EXISTS reported_benchmark_3 (
		    org_id            integer not null,
		    account_number    integer not null,
		    cluster           bytea not null,
		    notification_type integer not null,
		    state             integer not null,
		    report            varchar not null,
		    updated_at        timestamp not null,
		    notified_at       timestamp not null,
		    error_log         varchar,

		    PRIMARY KEY (org_id, cluster, notified_at)
		);
		`

	// SQL statement used to drop (delete) third table used by benchmarks
	DropTableReportedBenchmarkByteArrayClusterID = `
	        DROP TABLE IF EXISTS reported_benchmark_3;
        `
	// Index for the reported table used in benchmarks for
	// notified_at column
	CreateIndexReportedNotifiedAtDescV3 = `
                CREATE INDEX IF NOT EXISTS notified_at_desc_idx
		    ON reported_benchmark_3
		 USING btree (notified_at DESC);
        `

	// Index for the reported table used in benchmarks for
	// cluster column
	CreateIndexReportedClusterV3 = `
                CREATE INDEX IF NOT EXISTS reported_benchmark_3_cluster_idx
		    ON reported_benchmark_3
		 USING btree (cluster);
        `

	// Select cluster names from reported table
	SelectClusterNamesFromReportedV3Statement = `
            SELECT cluster FROM reported_benchmark_3
	`

	// Select report from reported table identified by cluster name
	SelectReportFromReportedV3Statement = `
            SELECT org_id, account_number, notification_type, state, report, updated_at, notified_at, error_log
	      FROM reported_benchmark_3
	     WHERE cluster=$1
	`

	// Insert one record into reported table
	InsertIntoReportedV3Statement = `
            INSERT INTO reported_benchmark_3
            (org_id, account_number, cluster, notification_type, state, report, updated_at, notified_at, error_log)
            VALUES
            ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	// Delete one record from reported table
	DeleteFromReportedV3Statement = `
            DELETE FROM reported_benchmark_3 WHERE cluster=$1
	`

	// CreateTableReportedBenchmarkUUIDClusterID is SQL statement used to
	// create reported table where cluster name is represented as UUID data
	// type (16 bytes)
	CreateTableReportedBenchmarkUUIDClusterID = `
		CREATE TABLE IF NOT EXISTS reported_benchmark_4 (
		    org_id            integer not null,
		    account_number    integer not null,
		    cluster           uuid not null,
		    notification_type integer not null,
		    state             integer not null,
		    report            varchar not null,
		    updated_at        timestamp not null,
		    notified_at       timestamp not null,
		    error_log         varchar,

		    PRIMARY KEY (org_id, cluster, notified_at)
		);
		`

	// SQL statement used to drop (delete) fourth table used by benchmarks
	DropTableReportedBenchmarkUUIDClusterID = `
	        DROP TABLE IF EXISTS reported_benchmark_4;
        `
	// Index for the reported table used in benchmarks for
	// notified_at column
	CreateIndexReportedNotifiedAtDescV4 = `
                CREATE INDEX IF NOT EXISTS notified_at_desc_idx
		    ON reported_benchmark_4
		 USING btree (notified_at DESC);
        `

	// Index for the reported table used in benchmarks for
	// cluster column
	CreateIndexReportedClusterV4 = `
                CREATE INDEX IF NOT EXISTS reported_benchmark_4_cluster_idx
		    ON reported_benchmark_4
		 USING btree (cluster);
        `

	// Select cluster names from reported table
	SelectClusterNamesFromReportedV4Statement = `
            SELECT cluster FROM reported_benchmark_4
	`

	// Select report from reported table identified by cluster name
	SelectReportFromReportedV4Statement = `
            SELECT org_id, account_number, notification_type, state, report, updated_at, notified_at, error_log
	      FROM reported_benchmark_4
	     WHERE cluster=$1
	`

	// Insert one record into reported table
	InsertIntoReportedV4Statement = `
            INSERT INTO reported_benchmark_4
            (org_id, account_number, cluster, notification_type, state, report, updated_at, notified_at, error_log)
            VALUES
            ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	// Delete one record from reported table
	DeleteFromReportedV4Statement = `
            DELETE FROM reported_benchmark_4 WHERE cluster=$1
	`
)

// Tests configuration
const (
	// Should tables used by benchmarks be dropped at the end?
	DropTables = true

	// Report to be used in benchmarks
	Report = ""

	// Time to breath between tests specified in minutes
	TimeToBreath = 2
)

// ConnectionInfo structure stores all values needed to connect to PSQL
type ConnectionInfo struct {
	username string
	password string
	host     string
	port     int
	dBName   string
	params   string
}

// InsertFunction represents callback functions called from benchmarks in order
// to insert new record into selected table
type InsertFunction func(b *testing.B, connection *sql.DB, insertStatement *string, i int, report *string)

// SelectFunction represents callback functions called from benchmarks in order
// to read report or reports identified by cluster name
type SelectFunction func(b *testing.B, connection *sql.DB, selectStatement *string, clusterName string) int

// readEnvVariable function tries to read content of specified environment
// variable with check if the variable exists
func readEnvVariable(b *testing.B, variableName string) string {
	// try to read content of given environment variable
	value := os.Getenv(variableName)

	// check if environment variable has been set
	if value == "" {
		b.Fatal(variableName, "environment variable not provided")
	}
	return value
}

// readConnectionInfoFromEnvVars function tries to read and parse environment
// variables used to connect to PSQL with all required error checks
func readConnectionInfoFromEnvVars(b *testing.B) ConnectionInfo {
	var connectionInfo ConnectionInfo

	// read string values
	connectionInfo.username = readEnvVariable(b, "DB_USER_NAME")
	connectionInfo.password = readEnvVariable(b, "DB_PASSWORD")
	connectionInfo.host = readEnvVariable(b, "DB_HOST")
	connectionInfo.dBName = readEnvVariable(b, "DB_NAME")
	connectionInfo.params = readEnvVariable(b, "DB_PARAMS")

	// parse port number
	port := readEnvVariable(b, "DB_PORT")
	portValue, err := strconv.Atoi(port)
	if err != nil {
		b.Fatal(err)
	}
	connectionInfo.port = portValue

	return connectionInfo
}

// connectToDatabase perform connection to PSQL with all required error checks
func connectToDatabase(b *testing.B, connectionInfo *ConnectionInfo) *sql.DB {
	// connection string
	dataSource := fmt.Sprintf(
		"postgresql://%v:%v@%v:%v/%v?%v",
		connectionInfo.username,
		connectionInfo.password,
		connectionInfo.host,
		connectionInfo.port,
		connectionInfo.dBName,
		connectionInfo.params,
	)

	// perform connection
	connection, err := sql.Open("postgres", dataSource)
	if err != nil {
		b.Error("Can not connect to data storage", err)
		return nil
	}

	// connection seems to be established
	return connection
}

// mustExecuteStatement function executes given SQL statement with check if the
// operation was finished without problems
func mustExecuteStatement(b *testing.B, connection *sql.DB, statement string) {
	// execute given SQL statement
	_, err := connection.Exec(statement)

	// check for any error thrown during execution
	if err != nil {
		b.Fatal("SQL statement '"+statement+"' error", err)
	}
}

// insertIntoReportedV1 function inserts one new record into reported table v1,
// v2, or v4 where cluster is represented as character(36) or as UUID. In case
// of any error detected, benchmarks fail immediately.
func insertIntoReportedV1(b *testing.B, connection *sql.DB, insertStatement *string, i int, report *string) {
	// following columns needs to be updated with data:
	// 1 | org_id            | integer                     | not null  |
	// 2 | account_number    | integer                     | not null  |
	// 3 | cluster           | character(36)               | not null  |
	// 4 | notification_type | integer                     | not null  |
	// 5 | state             | integer                     | not null  |
	// 6 | report            | character varying           | not null  |
	// 7 | updated_at        | timestamp without time zone | not null  |
	// 8 | notified_at       | timestamp without time zone | not null  |
	// 9 | error_log         | character varying           |           |

	// or

	// following columns needs to be updated with data:
	// 1 | org_id            | integer                     | not null  |
	// 2 | account_number    | integer                     | not null  |
	// 3 | cluster           | varchar(36)                 | not null  |
	// 4 | notification_type | integer                     | not null  |
	// 5 | state             | integer                     | not null  |
	// 6 | report            | character varying           | not null  |
	// 7 | updated_at        | timestamp without time zone | not null  |
	// 8 | notified_at       | timestamp without time zone | not null  |
	// 9 | error_log         | character varying           |           |

	// or

	// 1 | org_id            | integer                     | not null  |
	// 2 | account_number    | integer                     | not null  |
	// 3 | cluster           | UUID                        | not null  |
	// 4 | notification_type | integer                     | not null  |
	// 5 | state             | integer                     | not null  |
	// 6 | report            | character varying           | not null  |
	// 7 | updated_at        | timestamp without time zone | not null  |
	// 8 | notified_at       | timestamp without time zone | not null  |
	// 9 | error_log         | character varying           |           |

	// fill-in data to be inserted into the table
	orgID := i % 1000                  // limited number of org IDs
	accountNumber := orgID + 1         // can be different than org ID
	clusterName := uuid.New().String() // unique
	notificationTypeID := 1            // instant report
	stateID := 1 + i%4                 // just four states can be used
	updatedAt := time.Now()            // don't have to be unique
	notifiedAt := time.Now()           // don't have to be unique
	errorLog := ""                     // usually empty

	// perform insert
	_, err := connection.Exec(*insertStatement, orgID,
		accountNumber, clusterName, notificationTypeID, stateID,
		report, updatedAt, notifiedAt, errorLog)

	// check for any error, possible to exit immediately
	if err != nil {
		b.Fatal(err)
	}
}

// insertIntoReportedV3 function inserts one new record into reported
// table v3 where cluster is represented as byte array. In case of any error
// detected, benchmarks fail immediately.
func insertIntoReportedV3(b *testing.B, connection *sql.DB, insertStatement *string, i int, report *string) {
	// following columns needs to be updated with data:
	// 1 | org_id            | integer                     | not null  |
	// 2 | account_number    | integer                     | not null  |
	// 3 | cluster           | bytea                       | not null  |
	// 4 | notification_type | integer                     | not null  |
	// 5 | state             | integer                     | not null  |
	// 6 | report            | character varying           | not null  |
	// 7 | updated_at        | timestamp without time zone | not null  |
	// 8 | notified_at       | timestamp without time zone | not null  |
	// 9 | error_log         | character varying           |           |

	// fill-in data to be inserted into the table
	orgID := i % 1000          // limited number of org IDs
	accountNumber := orgID + 1 // can be different than org ID
	clusterName := uuid.New()  // unique

	// convert cluster name into bytes slice to be stored in database
	bytes, err := clusterName.MarshalBinary()
	if err != nil {
		b.Fatal(err)
	}

	notificationTypeID := 1  // instant report
	stateID := 1 + i%4       // just four states can be used
	updatedAt := time.Now()  // don't have to be unique
	notifiedAt := time.Now() // don't have to be unique
	errorLog := ""           // usually empty

	// perform insert
	_, err = connection.Exec(*insertStatement, orgID,
		accountNumber, bytes, notificationTypeID, stateID,
		report, updatedAt, notifiedAt, errorLog)

	// check for any error, possible to exit immediately
	if err != nil {
		b.Fatal(err)
	}
}

// selectFromReportedTableV1 function perform SELECT statement to retrieve
// reports from reported table when report or reports are identified by cluster
// name. Number of reports are returned from this function as the only return
// value.
func selectFromReportedTableV1(b *testing.B, connection *sql.DB, selectStatement *string, clusterName string) int {
	// execute SQL query statement
	rows, err := connection.Query(*selectStatement, clusterName)
	if err != nil {
		b.Fatal("Select from reported table error", err)
		return 0
	}

	// result set needs to be closed at the end
	defer func() {
		err := rows.Close()
		if err != nil {
			b.Fatal("Closing query error", err)
		}
	}()

	// report counter
	reports := 0

	// read all records
	for rows.Next() {
		reports++
		var orgID int
		var accountNumber int
		var notificationType int
		var state int
		var report string
		var updatedAt time.Time
		var notifiedAt time.Time
		var errorLog string

		// read one report from result set
		err := rows.Scan(&orgID, &accountNumber, &notificationType, &state, &report, &updatedAt, &notifiedAt, &errorLog)

		// check for any scan errors
		if err != nil {
			if closeErr := rows.Close(); closeErr != nil {
				b.Fatal("Unable to close DB row handle")
			}
			return reports
		}
	}
	// return number of reports read and parsed
	return reports
}

// All functions used to select reports from reported table are actually the same
var selectFromReportedTableV2 = selectFromReportedTableV1
var selectFromReportedTableV3 = selectFromReportedTableV1
var selectFromReportedTableV4 = selectFromReportedTableV1

// readClusterNames is helper function to read al cluster names stored in
// reported table
func readClusterNames(b *testing.B, connection *sql.DB, selectClusterNamesStatement string) []string {
	var clusterNames = make([]string, 0)

	// execute SQL query statement
	rows, err := connection.Query(selectClusterNamesStatement)
	if err != nil {
		b.Fatal("Select cluster names error", err)
		return clusterNames
	}

	// result set needs to be closed at the end
	defer func() {
		err := rows.Close()
		if err != nil {
			b.Fatal("Closing query error", err)
		}
	}()

	// read all records
	for rows.Next() {
		var clusterName string

		// read cluster name from result set
		err := rows.Scan(&clusterName)

		// check for any scan errors
		if err != nil {
			if closeErr := rows.Close(); closeErr != nil {
				b.Fatal("Unable to close DB row handle")
			}
			return clusterNames
		}
		clusterNames = append(clusterNames, clusterName)
	}

	return clusterNames
}

// deleteReportByClusterName function deletes report or reports identified by
// cluster name.
func deleteReportByClusterName(b *testing.B, connection *sql.DB, deleteStatement, clusterName string) {
	// execute SQL delete statement
	_, err := connection.Exec(deleteStatement, clusterName)
	if err != nil {
		b.Fatal("SQL delete statement '"+deleteStatement+"' error", err)
	}
}

// function to insert record into table v2 is the same as function to insert
// record into table v1
var insertIntoReportedV2 = insertIntoReportedV1

// function to insert record into table v4 is the same as function to insert
// record into table v1
var insertIntoReportedV4 = insertIntoReportedV1

// performInsertBenchmark function contains implementation of INSERTion
// benchmarks for all possible table variants.
func performInsertBenchmark(b *testing.B,
	createTableStatement, dropTableStatement, insertStatement string,
	insertFunction InsertFunction,
	report *string,
	dropTables bool) {

	// connect to database
	b.StopTimer()
	time.Sleep(TimeToBreath * time.Minute)
	connectionInfo := readConnectionInfoFromEnvVars(b)
	connection := connectToDatabase(b, &connectionInfo)

	// create table used by benchmark
	mustExecuteStatement(b, connection, createTableStatement)

	// perform benchmark - insert many reports into the table
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		insertFunction(b, connection, &insertStatement, i, report)
	}
	b.StopTimer()

	// drop table used by benchmark
	if dropTables {
		mustExecuteStatement(b, connection, dropTableStatement)
	}
}

// performDeleteBenchmark function contains implementation of DELETion
// benchmarks for all possible table variants.
func performDeleteBenchmark(b *testing.B,
	createTableStatement, createIndexStatement, dropTableStatement, insertStatement, deleteStatement, selectClusterNamesStatement string,
	insertFunction InsertFunction,
	report *string,
	dropTables bool) {

	// connect to database
	b.StopTimer()
	time.Sleep(TimeToBreath * time.Minute)
	connectionInfo := readConnectionInfoFromEnvVars(b)
	connection := connectToDatabase(b, &connectionInfo)

	// create table used by benchmark
	mustExecuteStatement(b, connection, dropTableStatement)
	mustExecuteStatement(b, connection, createTableStatement)

	// create index if required
	if createIndexStatement != "" {
		mustExecuteStatement(b, connection, createIndexStatement)
	}

	// fill-in some data
	for i := 0; i < b.N; i++ {
		insertFunction(b, connection, &insertStatement, i, report)
	}

	// retrieve cluster names
	clusterNames := readClusterNames(b, connection, selectClusterNamesStatement)

	// perform benchmark - delete selected rows from table one by one
	b.StartTimer()
	for _, clusterName := range clusterNames {
		deleteReportByClusterName(b, connection, deleteStatement, clusterName)
	}
	b.StopTimer()

	// drop table used by benchmark
	if dropTables {
		mustExecuteStatement(b, connection, dropTableStatement)
	}
}

// performSelectBenchmark function contains implementation of SELECTion
// benchmarks for all possible table variants.
func performSelectBenchmark(b *testing.B,
	createTableStatement, createIndexStatement, dropTableStatement, insertStatement, selectReportStatement, selectClusterNamesStatement string,
	insertFunction InsertFunction,
	selectFunction SelectFunction,
	report *string,
	dropTables bool) {

	// connect to database
	b.StopTimer()
	time.Sleep(TimeToBreath * time.Minute)
	connectionInfo := readConnectionInfoFromEnvVars(b)
	connection := connectToDatabase(b, &connectionInfo)

	// create table used by benchmark
	mustExecuteStatement(b, connection, createTableStatement)

	// create index if required
	if createIndexStatement != "" {
		mustExecuteStatement(b, connection, createIndexStatement)
	}

	// fill-in some data
	for i := 0; i < b.N; i++ {
		insertFunction(b, connection, &insertStatement, i, report)
	}

	// retrieve cluster names
	clusterNames := readClusterNames(b, connection, selectClusterNamesStatement)

	reports := 0

	// perform benchmark - select report or reports identified by cluster name
	b.StartTimer()
	for _, clusterName := range clusterNames {
		reports += selectFunction(b, connection, &selectReportStatement, clusterName)
	}
	b.StopTimer()

	// drop table used by benchmark
	if dropTables {
		mustExecuteStatement(b, connection, dropTableStatement)
	}

	// check total number of reports
	// we assume that cluster names (UUIDs) are really unique
	if reports != b.N {
		b.Fatal("Total number of reports differs from table size", reports, b.N)
	}
}

// BenchmarkInsertClusterAsChar function contains implementation of benchmark
// that performs INSERTion into report table where cluster name is represented
// as CHAR(36).
func BenchmarkInsertClusterAsChar(b *testing.B) {
	// report to be stored in a table
	report := Report

	// run the actual benchmark
	performInsertBenchmark(b,
		CreateTableReportedBenchmarkCharClusterID,
		DropTableReportedBenchmarkCharClusterID,
		InsertIntoReportedV1Statement,
		insertIntoReportedV1,
		&report, DropTables)
}

// BenchmarkInsertClusterAsVarchar function contains implementation of benchmark
// that performs INSERTion into report table where cluster name is represented
// as VARCHAR(36).
func BenchmarkInsertClusterAsVarchar(b *testing.B) {
	// report to be stored in a table
	report := Report

	// run the actual benchmark
	performInsertBenchmark(b,
		CreateTableReportedBenchmarkVarcharClusterID,
		DropTableReportedBenchmarkVarcharClusterID,
		InsertIntoReportedV2Statement,
		insertIntoReportedV2,
		&report, DropTables)
}

// BenchmarkInsertClusterAsBytea function contains implementation of benchmark
// that performs INSERTion into report table where cluster name is represented
// as BYTEA.
func BenchmarkInsertClusterAsBytea(b *testing.B) {
	// report to be stored in a table
	report := Report

	// run the actual benchmark
	performInsertBenchmark(b,
		CreateTableReportedBenchmarkByteArrayClusterID,
		DropTableReportedBenchmarkByteArrayClusterID,
		InsertIntoReportedV3Statement,
		insertIntoReportedV3,
		&report, DropTables)
}

// BenchmarkInsertClusterAsUUID function contains implementation of benchmark
// that performs INSERTion into report table where cluster name is represented
// as UUID.
func BenchmarkInsertClusterAsUUID(b *testing.B) {
	// report to be stored in a table
	report := Report

	// run the actual benchmark
	performInsertBenchmark(b,
		CreateTableReportedBenchmarkUUIDClusterID,
		DropTableReportedBenchmarkUUIDClusterID,
		InsertIntoReportedV4Statement,
		insertIntoReportedV4,
		&report, DropTables)
}

// BenchmarkDeleteClusterAsChar function contains implementation of benchmark
// that performs DELETion from report table where cluster name is represented
// as CHAR(36).
func BenchmarkDeleteClusterAsChar(b *testing.B) {
	// report to be stored in a table
	report := Report

	// run the actual benchmark
	performDeleteBenchmark(b,
		CreateTableReportedBenchmarkCharClusterID,
		"",
		DropTableReportedBenchmarkCharClusterID,
		InsertIntoReportedV1Statement,
		DeleteFromReportedV1Statement,
		SelectClusterNamesFromReportedV1Statement,
		insertIntoReportedV1,
		&report, DropTables)
}

// BenchmarkDeleteClusterAsVarchar function contains implementation of benchmark
// that performs DELETion from report table where cluster name is represented
// as VARCHAR(36).
func BenchmarkDeleteClusterAsVarchar(b *testing.B) {
	// report to be stored in a table
	report := Report

	// run the actual benchmark
	performDeleteBenchmark(b,
		CreateTableReportedBenchmarkVarcharClusterID,
		"",
		DropTableReportedBenchmarkVarcharClusterID,
		InsertIntoReportedV2Statement,
		DeleteFromReportedV2Statement,
		SelectClusterNamesFromReportedV2Statement,
		insertIntoReportedV2,
		&report, DropTables)
}

// BenchmarkDeleteClusterAsBytea function contains implementation of benchmark
// that performs DELETion from report table where cluster name is represented
// as BYTEA.
func BenchmarkDeleteClusterAsBytea(b *testing.B) {
	// report to be stored in a table
	report := Report

	// run the actual benchmark
	performDeleteBenchmark(b,
		CreateTableReportedBenchmarkByteArrayClusterID,
		"",
		DropTableReportedBenchmarkByteArrayClusterID,
		InsertIntoReportedV3Statement,
		DeleteFromReportedV3Statement,
		SelectClusterNamesFromReportedV3Statement,
		insertIntoReportedV3,
		&report, DropTables)
}

// BenchmarkDeleteClusterAsUUID function contains implementation of benchmark
// that performs DELETion from report table where cluster name is represented
// as UUID.
func BenchmarkDeleteClusterAsUUID(b *testing.B) {
	// report to be stored in a table
	report := Report

	// run the actual benchmark
	performDeleteBenchmark(b,
		CreateTableReportedBenchmarkUUIDClusterID,
		"",
		DropTableReportedBenchmarkUUIDClusterID,
		InsertIntoReportedV4Statement,
		DeleteFromReportedV4Statement,
		SelectClusterNamesFromReportedV4Statement,
		insertIntoReportedV4,
		&report, DropTables)
}

// BenchmarkSelectClusterAsChar function contains implementation of benchmark
// that performs SELECTion from report table where cluster name is represented
// as CHAR(36).
func BenchmarkSelectClusterAsChar(b *testing.B) {
	// report to be stored in a table
	report := Report

	// run the actual benchmark
	performSelectBenchmark(b,
		CreateTableReportedBenchmarkCharClusterID,
		"",
		DropTableReportedBenchmarkCharClusterID,
		InsertIntoReportedV1Statement,
		SelectReportFromReportedV1Statement,
		SelectClusterNamesFromReportedV1Statement,
		insertIntoReportedV1,
		selectFromReportedTableV1,
		&report, DropTables)
}

// BenchmarkSelectClusterAsVarchar function contains implementation of benchmark
// that performs SELECTion from report table where cluster name is represented
// as VARCHAR(36).
func BenchmarkSelectClusterAsVarchar(b *testing.B) {
	// report to be stored in a table
	report := Report

	// run the actual benchmark
	performSelectBenchmark(b,
		CreateTableReportedBenchmarkVarcharClusterID,
		"",
		DropTableReportedBenchmarkVarcharClusterID,
		InsertIntoReportedV2Statement,
		SelectReportFromReportedV2Statement,
		SelectClusterNamesFromReportedV2Statement,
		insertIntoReportedV2,
		selectFromReportedTableV2,
		&report, DropTables)
}

// BenchmarkSelectClusterAsBytea function contains implementation of benchmark
// that performs SELECTion from report table where cluster name is represented
// as BYTEA.
func BenchmarkSelectClusterAsBytea(b *testing.B) {
	// report to be stored in a table
	report := Report

	// run the actual benchmark
	performSelectBenchmark(b,
		CreateTableReportedBenchmarkByteArrayClusterID,
		"",
		DropTableReportedBenchmarkByteArrayClusterID,
		InsertIntoReportedV3Statement,
		SelectReportFromReportedV3Statement,
		SelectClusterNamesFromReportedV3Statement,
		insertIntoReportedV3,
		selectFromReportedTableV3,
		&report, DropTables)
}

// BenchmarkSelectClusterAsUUID function contains implementation of benchmark
// that performs SELECTion from report table where cluster name is represented
// as UUID.
func BenchmarkSelectClusterAsUUID(b *testing.B) {
	// report to be stored in a table
	report := Report

	// run the actual benchmark
	performSelectBenchmark(b,
		CreateTableReportedBenchmarkUUIDClusterID,
		"",
		DropTableReportedBenchmarkUUIDClusterID,
		InsertIntoReportedV4Statement,
		SelectReportFromReportedV4Statement,
		SelectClusterNamesFromReportedV4Statement,
		insertIntoReportedV4,
		selectFromReportedTableV4,
		&report, DropTables)
}

// BenchmarkDeleteClusterAsCharIndexed function contains implementation of benchmark
// that performs DELETion from report table where cluster name is represented
// as CHAR(36). Cluster column is used as index.
func BenchmarkDeleteClusterAsCharIndexed(b *testing.B) {
	// report to be stored in a table
	report := Report

	// run the actual benchmark
	performDeleteBenchmark(b,
		CreateTableReportedBenchmarkCharClusterID,
		CreateIndexReportedClusterV1,
		DropTableReportedBenchmarkCharClusterID,
		InsertIntoReportedV1Statement,
		DeleteFromReportedV1Statement,
		SelectClusterNamesFromReportedV1Statement,
		insertIntoReportedV1,
		&report, DropTables)
}

// BenchmarkDeleteClusterAsVarcharIndexed function contains implementation of benchmark
// that performs DELETion from report table where cluster name is represented
// as VARCHAR(36). Cluster column is used as index.
func BenchmarkDeleteClusterAsVarcharIndexed(b *testing.B) {
	// report to be stored in a table
	report := Report

	// run the actual benchmark
	performDeleteBenchmark(b,
		CreateTableReportedBenchmarkVarcharClusterID,
		CreateIndexReportedClusterV2,
		DropTableReportedBenchmarkVarcharClusterID,
		InsertIntoReportedV2Statement,
		DeleteFromReportedV2Statement,
		SelectClusterNamesFromReportedV2Statement,
		insertIntoReportedV2,
		&report, DropTables)
}

// BenchmarkDeleteClusterAsByteaIndexed function contains implementation of benchmark
// that performs DELETion from report table where cluster name is represented
// as BYTEA. Cluster column is used as index.
func BenchmarkDeleteClusterAsByteaIndexed(b *testing.B) {
	// report to be stored in a table
	report := Report

	// run the actual benchmark
	performDeleteBenchmark(b,
		CreateTableReportedBenchmarkByteArrayClusterID,
		CreateIndexReportedClusterV3,
		DropTableReportedBenchmarkByteArrayClusterID,
		InsertIntoReportedV3Statement,
		DeleteFromReportedV3Statement,
		SelectClusterNamesFromReportedV3Statement,
		insertIntoReportedV3,
		&report, DropTables)
}

// BenchmarkDeleteClusterAsUUIDIndexed function contains implementation of benchmark
// that performs DELETion from report table where cluster name is represented
// as UUID. Cluster column is used as index.
func BenchmarkDeleteClusterAsUUIDIndexed(b *testing.B) {
	// report to be stored in a table
	report := Report

	// run the actual benchmark
	performDeleteBenchmark(b,
		CreateTableReportedBenchmarkUUIDClusterID,
		CreateIndexReportedClusterV4,
		DropTableReportedBenchmarkUUIDClusterID,
		InsertIntoReportedV4Statement,
		DeleteFromReportedV4Statement,
		SelectClusterNamesFromReportedV4Statement,
		insertIntoReportedV4,
		&report, DropTables)
}

// BenchmarkSelectClusterAsCharIndexed function contains implementation of benchmark
// that performs SELECTion from report table where cluster name is represented
// as CHAR(36). Cluster column is used as index.
func BenchmarkSelectClusterAsCharIndexed(b *testing.B) {
	// report to be stored in a table
	report := Report

	// run the actual benchmark
	performSelectBenchmark(b,
		CreateTableReportedBenchmarkCharClusterID,
		CreateIndexReportedClusterV1,
		DropTableReportedBenchmarkCharClusterID,
		InsertIntoReportedV1Statement,
		SelectReportFromReportedV1Statement,
		SelectClusterNamesFromReportedV1Statement,
		insertIntoReportedV1,
		selectFromReportedTableV1,
		&report, DropTables)
}

// BenchmarkSelectClusterAsVarcharIndexed function contains implementation of benchmark
// that performs SELECTion from report table where cluster name is represented
// as VARCHAR(36). Cluster column is used as index.
func BenchmarkSelectClusterAsVarcharIndexed(b *testing.B) {
	// report to be stored in a table
	report := Report

	// run the actual benchmark
	performSelectBenchmark(b,
		CreateTableReportedBenchmarkVarcharClusterID,
		CreateIndexReportedClusterV2,
		DropTableReportedBenchmarkVarcharClusterID,
		InsertIntoReportedV2Statement,
		SelectReportFromReportedV2Statement,
		SelectClusterNamesFromReportedV2Statement,
		insertIntoReportedV2,
		selectFromReportedTableV2,
		&report, DropTables)
}

// BenchmarkSelectClusterAsByteaIndexed function contains implementation of benchmark
// that performs SELECTion from report table where cluster name is represented
// as BYTEA. Cluster column is used as index.
func BenchmarkSelectClusterAsByteaIndexed(b *testing.B) {
	// report to be stored in a table
	report := Report

	// run the actual benchmark
	performSelectBenchmark(b,
		CreateTableReportedBenchmarkByteArrayClusterID,
		CreateIndexReportedClusterV3,
		DropTableReportedBenchmarkByteArrayClusterID,
		InsertIntoReportedV3Statement,
		SelectReportFromReportedV3Statement,
		SelectClusterNamesFromReportedV3Statement,
		insertIntoReportedV3,
		selectFromReportedTableV3,
		&report, DropTables)
}

// BenchmarkSelectClusterAsUUIDIndexed function contains implementation of benchmark
// that performs SELECTion from report table where cluster name is represented
// as UUID. Cluster column is used as index.
func BenchmarkSelectClusterAsUUIDIndexed(b *testing.B) {
	// report to be stored in a table
	report := Report

	// run the actual benchmark
	performSelectBenchmark(b,
		CreateTableReportedBenchmarkUUIDClusterID,
		CreateIndexReportedClusterV4,
		DropTableReportedBenchmarkUUIDClusterID,
		InsertIntoReportedV4Statement,
		SelectReportFromReportedV4Statement,
		SelectClusterNamesFromReportedV4Statement,
		insertIntoReportedV4,
		selectFromReportedTableV4,
		&report, DropTables)
}
