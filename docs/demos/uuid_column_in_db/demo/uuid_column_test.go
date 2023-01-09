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
	CreateTableReportedBenchmarkVarcharClusterID = `
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

	DropTableReportedBenchmarkVarcharClusterID = `
	        DROP TABLE IF EXISTS reported_benchmark_1;
        `
	// Index for the reported table used in benchmarks for
	// notified_at column
	CreateIndexReportedNotifiedAtDescV1 = `
                CREATE INDEX IF NOT EXISTS notified_at_desc_idx
		    ON reported_benchmark_1
		 USING btree (notified_at DESC);
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

	CreateTableReportedBenchmarkByteArrayClusterID = `
		CREATE TABLE IF NOT EXISTS reported_benchmark_2 (
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

	DropTableReportedBenchmarkByteArrayClusterID = `
	        DROP TABLE IF EXISTS reported_benchmark_2;
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

	CreateTableReportedBenchmarkUUIDClusterID = `
		CREATE TABLE IF NOT EXISTS reported_benchmark_3 (
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

	DropTableReportedBenchmarkUUIDClusterID = `
	        DROP TABLE IF EXISTS reported_benchmark_3;
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
)

// Tests configuration
const (
	// Should tables used by benchmarks be dropped at the end?
	DropTables = false
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

// readEnvVariable function tries to read content of specified environment
// variable with check if the variable exists
func readEnvVariable(b *testing.B, variableName string) string {
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
	if err != nil {
		b.Fatal("SQL statement '"+statement+"' error", err)
	}
}

// insertIntoReportedV1 function inserts one new record into reported table v1
// or v3 where cluster is represented as character(36) or as UUID. In case of
// any error detected, benchmarks fail immediatelly.
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
	// 1 | org_id            | integer                     | not null  |
	// 2 | account_number    | integer                     | not null  |
	// 3 | cluster           | UUID                        | not null  |
	// 4 | notification_type | integer                     | not null  |
	// 5 | state             | integer                     | not null  |
	// 6 | report            | character varying           | not null  |
	// 7 | updated_at        | timestamp without time zone | not null  |
	// 8 | notified_at       | timestamp without time zone | not null  |
	// 9 | error_log         | character varying           |           |

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

	// check for any error, possible to exit immediatelly
	if err != nil {
		b.Fatal(err)
	}
}

// insertIntoReportedV2 function inserts one new record into reported
// table v2 where cluster is represented as byte array. In case of any error
// detected, benchmarks fail immediatelly.
func insertIntoReportedV2(b *testing.B, connection *sql.DB, insertStatement *string, i int, report *string) {
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

	// check for any error, possible to exit immediatelly
	if err != nil {
		b.Fatal(err)
	}
}

func readClusterNames(b *testing.B, connection *sql.DB, selectStatement string) []string {
	var clusterNames = make([]string, 0)

	rows, err := connection.Query(selectStatement)
	if err != nil {
		b.Fatal("Select cluster names error", err)
		return clusterNames
	}

	defer func() {
		err := rows.Close()
		if err != nil {
			b.Fatal("Closing query error", err)
		}
	}()

	// read all records
	for rows.Next() {
		var clusterName string

		err := rows.Scan(&clusterName)
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

func deleteReportByClusterName(b *testing.B, connection *sql.DB, deleteStatement string, clusterName string) {
	_, err := connection.Exec(deleteStatement, clusterName)
	if err != nil {
		b.Fatal("SQL delete statement '"+deleteStatement+"' error", err)
	}
}

// function to insert record into table v3 is the same as function to insert
// record into table v1
var insertIntoReportedV3 = insertIntoReportedV1

func performInsertBenchmark(b *testing.B,
	createTableStatement, dropTableStatement, insertStatement string,
	insertFunction InsertFunction,
	report *string,
	dropTables bool) {

	// connect to database
	b.StopTimer()
	connectionInfo := readConnectionInfoFromEnvVars(b)
	connection := connectToDatabase(b, &connectionInfo)

	// create table used by benchmark
	mustExecuteStatement(b, connection, createTableStatement)

	// perform benchmark
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

func performDeleteBenchmark(b *testing.B,
	createTableStatement, dropTableStatement, insertStatement, deleteStatement, selectStatement string,
	insertFunction InsertFunction,
	report *string,
	dropTables bool) {

	// connect to database
	b.StopTimer()
	connectionInfo := readConnectionInfoFromEnvVars(b)
	connection := connectToDatabase(b, &connectionInfo)

	// create table used by benchmark
	mustExecuteStatement(b, connection, createTableStatement)

	// fill-in some data
	for i := 0; i < b.N; i++ {
		insertFunction(b, connection, &insertStatement, i, report)
	}

	// retrieve cluster names
	clusterNames := readClusterNames(b, connection, selectStatement)

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

func BenchmarkInsertClusterAsVarchar(b *testing.B) {
	report := ""
	performInsertBenchmark(b,
		CreateTableReportedBenchmarkVarcharClusterID,
		DropTableReportedBenchmarkVarcharClusterID,
		InsertIntoReportedV1Statement,
		insertIntoReportedV1,
		&report, DropTables)
}

func BenchmarkInsertClusterAsBytea(b *testing.B) {
	report := ""
	performInsertBenchmark(b,
		CreateTableReportedBenchmarkByteArrayClusterID,
		DropTableReportedBenchmarkByteArrayClusterID,
		InsertIntoReportedV2Statement,
		insertIntoReportedV2,
		&report, DropTables)
}

func BenchmarkInsertClusterAsUUID(b *testing.B) {
	report := ""
	performInsertBenchmark(b,
		CreateTableReportedBenchmarkUUIDClusterID,
		DropTableReportedBenchmarkUUIDClusterID,
		InsertIntoReportedV3Statement,
		insertIntoReportedV3,
		&report, DropTables)
}

func BenchmarkDeleteClusterAsVarchar(b *testing.B) {
	report := ""
	performDeleteBenchmark(b,
		CreateTableReportedBenchmarkVarcharClusterID,
		DropTableReportedBenchmarkVarcharClusterID,
		InsertIntoReportedV1Statement,
		DeleteFromReportedV1Statement,
		SelectClusterNamesFromReportedV1Statement,
		insertIntoReportedV1,
		&report, DropTables)
}

func BenchmarkDeleteClusterAsBytea(b *testing.B) {
	report := ""
	performDeleteBenchmark(b,
		CreateTableReportedBenchmarkByteArrayClusterID,
		DropTableReportedBenchmarkByteArrayClusterID,
		InsertIntoReportedV2Statement,
		DeleteFromReportedV2Statement,
		SelectClusterNamesFromReportedV2Statement,
		insertIntoReportedV2,
		&report, DropTables)
}

func BenchmarkDeleteClusterAsUUID(b *testing.B) {
	report := ""
	performDeleteBenchmark(b,
		CreateTableReportedBenchmarkUUIDClusterID,
		DropTableReportedBenchmarkUUIDClusterID,
		InsertIntoReportedV3Statement,
		DeleteFromReportedV3Statement,
		SelectClusterNamesFromReportedV3Statement,
		insertIntoReportedV3,
		&report, DropTables)
}

func BenchmarkSelectClusterAsVarchar(b *testing.B) {
}

func BenchmarkSelectClusterAsBytea(b *testing.B) {
}

func BenchmarkSelectClusterAsUUID(b *testing.B) {
}
