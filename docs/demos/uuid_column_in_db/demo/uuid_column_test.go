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
)

func BenchmarkInsertUUIDAsVarchar(b *testing.B) {
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
