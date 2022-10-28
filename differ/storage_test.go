/*
Copyright © 2022 Red Hat, Inc.

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
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"

	"github.com/RedHatInsights/ccx-notification-service/differ"
	"github.com/RedHatInsights/ccx-notification-service/types"
)

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
	WHERE notified_at > NOW() - $1::INTERVAL;
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

	//If timeOffset is 0 or empty string, the WHERE clause is not included
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
