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

// insertIntoReportedV1Statement function inserts one new record into reported
// table v1. In case of any error detected, benchmarks fail immediatelly.
func insertIntoReportedV1(b *testing.B, connection *sql.DB, i int, report *string) {
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

	orgID := i % 1000                  // limited number of org IDs
	accountNumber := orgID + 1         // can be different than org ID
	clusterName := uuid.New().String() // unique
	notificationTypeID := 1            // instant report
	stateID := 1 + i%4                 // just four states can be used
	updatedAt := time.Now()            // don't have to be unique
	notifiedAt := time.Now()           // don't have to be unique
	errorLog := ""                     // usually empty

	// perform insert
	_, err := connection.Exec(InsertIntoReportedV1Statement, orgID,
		accountNumber, clusterName, notificationTypeID, stateID,
		report, updatedAt, notifiedAt, errorLog)

	// check for any error, possible to exit immediatelly
	if err != nil {
		b.Fatal(err)
	}
}

func performInsertBenchmark(b *testing.B, createTableStatement, dropTableStatement string, report *string) {
	// connect to database
	b.StopTimer()
	connectionInfo := readConnectionInfoFromEnvVars(b)
	// log.Print(connectionInfo)
	connection := connectToDatabase(b, &connectionInfo)
	// log.Print(connection)

	// create table used by benchmark
	_, err := connection.Exec(createTableStatement)
	if err != nil {
		b.Error("Create table error", err)
		return
	}

	// perform benchmark
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		insertIntoReportedV1(b, connection, i, report)
	}
	b.StopTimer()

	// drop table used by benchmark
	_, err = connection.Exec(dropTableStatement)
	if err != nil {
		b.Error("Drop table error", err)
		return
	}
	b.StartTimer()
}

func BenchmarkInsertUUIDAsVarchar(b *testing.B) {
	report := ""
	performInsertBenchmark(b, CreateTableReportedBenchmarkVarcharClusterID, DropTableReportedBenchmarkVarcharClusterID, &report)
}

func BenchmarkInsertUUIDAsBytea(b *testing.B) {
}

func BenchmarkInsertUUIDAsUUID(b *testing.B) {
}

func BenchmarkDeleteUUIDAsVarchar(b *testing.B) {
}

func BenchmarkDeleteUUIDAsBytea(b *testing.B) {
}

func BenchmarkDeleteUUIDAsUUID(b *testing.B) {
}

func BenchmarkSelectUUIDAsVarchar(b *testing.B) {
}

func BenchmarkSelectUUIDAsBytea(b *testing.B) {
}

func BenchmarkSelectUUIDAsUUID(b *testing.B) {
}
