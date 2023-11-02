package differ

import (
	"bytes"
	"database/sql"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/RedHatInsights/ccx-notification-service/tests/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/RedHatInsights/ccx-notification-service/types"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	testPartitionID = 0
	testOffset      = 0
)

func init() {
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
}

func newMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	return db, mock
}

func TestRetrievePreviouslyReportedForEventTarget(t *testing.T) {
	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.InfoLevel)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
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
		timeOffset = "1 day"
	)
	producerMock := mocks.Producer{}
	producerMock.On("ProduceMessage", mock.AnythingOfType("types.ProducerMessage")).Return(
		func(msg types.ProducerMessage) int32 {
			testPartitionID++
			return int32(testPartitionID)
		},
		func(msg types.ProducerMessage) int64 {
			testOffset++
			return int64(testOffset)
		},
		func(msg types.ProducerMessage) error {
			return nil
		},
	)
	db, mock := newMock(t)
	defer func() { _ = db.Close() }()

	sut := NewFromConnection(db, types.DBDriverPostgres)
	d := Differ{
		Storage:          sut,
		NotificationType: types.InstantNotif,
		Target:           types.NotificationBackendTarget,
		Thresholds: EventThresholds{
			TotalRisk: DefaultTotalRiskThreshold,
		},
		Filter:   DefaultEventFilter,
		Notifier: &producerMock,
	}
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
		AddRow(1, "second cluster", "test", now)

	mock.ExpectQuery(regexp.QuoteMeta(expectedQuery)).
		WithArgs(timeOffset).
		WillReturnRows(rows)

	d.retrievePreviouslyReportedForEventTarget(timeOffset, types.NotificationBackendTarget, clusterEntries)
	executionLog := buf.String()
	assert.Contains(t, executionLog, "{\"level\":\"info\",\"message\":\"Reading previously reported issues for given cluster list...\"}\n{\"level\":\"info\",\"target\":1,\"retrieved\":2,\"message\":\"Done reading previously reported issues still in cool down\"}\n")
}

func TestRetrievePreviouslyReportedForEventTargetEmptyClusterEntries(t *testing.T) {
	clusterEntries := []types.ClusterEntry{}
	timeOffset := "1 day"
	buf := new(bytes.Buffer)

	log.Logger = zerolog.New(buf).Level(zerolog.InfoLevel)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	producerMock := mocks.Producer{}
	producerMock.On("ProduceMessage", mock.AnythingOfType("types.ProducerMessage")).Return(
		func(msg types.ProducerMessage) int32 {
			testPartitionID++
			return int32(testPartitionID)
		},
		func(msg types.ProducerMessage) int64 {
			testOffset++
			return int64(testOffset)
		},
		func(msg types.ProducerMessage) error {
			return nil
		},
	)

	// prepare database mock
	db, _ := newMock(t)
	defer func() { _ = db.Close() }()

	// establish connection to mocked database
	sut := NewFromConnection(db, types.DBDriverPostgres)

	d := Differ{
		Storage:          sut,
		NotificationType: types.InstantNotif,
		Target:           types.NotificationBackendTarget,
		Thresholds: EventThresholds{
			TotalRisk: DefaultTotalRiskThreshold,
		},
		Filter:   DefaultEventFilter,
		Notifier: &producerMock,
	}

	// call tested method
	d.retrievePreviouslyReportedForEventTarget(timeOffset, types.NotificationBackendTarget, clusterEntries)
	// test returned values
	executionLog := buf.String()
	assert.Contains(t, executionLog, "{\"level\":\"info\",\"message\":\"Reading previously reported issues for given cluster list...\"}\n{\"level\":\"info\",\"target\":1,\"retrieved\":0,\"message\":\"Done reading previously reported issues still in cool down\"}\n")
}
