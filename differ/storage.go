/*
Copyright © 2021, 2022, 2023 Red Hat, Inc.

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

package differ

// Generated documentation is available at:
// https://pkg.go.dev/github.com/RedHatInsights/ccx-notification-service/differ
//
// Documentation in literate-programming-style is available at:
// https://redhatinsights.github.io/ccx-notification-service/packages/differ/storage.html

// This source file contains an implementation of interface between Go code and
// (almost any) SQL database like PostgreSQL, SQLite, or MariaDB.
//
// It is possible to configure connection to selected database by using
// StorageConfiguration structure. Currently that structure contains two
// configurable parameter:
//
// Driver - a SQL driver, like "sqlite3", "pq" etc.
// DataSource - specification of data source. The content of this parameter depends on the database used.

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"           // PostgreSQL database driver
	_ "github.com/mattn/go-sqlite3" // SQLite database driver

	"github.com/rs/zerolog/log"

	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/types"
)

// Storage represents an interface to almost any database or storage system
type Storage interface {
	Close() error
	ReadReportForCluster(
		orgID types.OrgID, clusterName types.ClusterName) (types.ClusterReport, types.Timestamp, error,
	)
	ReadClusterList() ([]types.ClusterEntry, error)
	ReadNotificationTypes() ([]types.NotificationType, error)
	ReadStates() ([]types.State, error)
	ReadReportForClusterAtTime(
		orgID types.OrgID, clusterName types.ClusterName,
		updatedAt types.Timestamp) (types.ClusterReport, error,
	)
	ReadReportForClusterAtOffset(
		orgID types.OrgID, clusterName types.ClusterName,
		offset types.KafkaOffset) (types.ClusterReport, error,
	)
	ReadLastNotifiedRecordForClusterList(
		clusterEntries []types.ClusterEntry, timeOffset string, eventTarget types.EventTarget) (types.NotifiedRecordsPerCluster, error)
	WriteNotificationRecord(
		notificationRecord types.NotificationRecord) error
	WriteNotificationRecordForCluster(
		clusterEntry types.ClusterEntry,
		notificationTypeID types.NotificationTypeID,
		stateID types.StateID,
		report types.ClusterReport,
		notifiedAt types.Timestamp,
		errorLog string,
		eventTarget types.EventTarget) error
	WriteNotificationRecordImpl(
		orgID types.OrgID,
		accountNumber types.AccountNumber,
		clusterName types.ClusterName,
		notificationTypeID types.NotificationTypeID,
		stateID types.StateID,
		report types.ClusterReport,
		updatedAt types.Timestamp,
		notifiedAt types.Timestamp,
		errorLog string,
		eventType types.EventTarget) error
	ReadErrorExists(
		orgID types.OrgID,
		clusterName types.ClusterName,
		lastCheckedTime time.Time,
	) (bool, error)
	WriteReadError(
		orgID types.OrgID,
		clusterName types.ClusterName,
		lastCheckedTime time.Time,
		e error) error
	DeleteRowFromNewReports(
		orgID types.OrgID,
		clusterName types.ClusterName,
		updatedAt types.Timestamp) (int, error)
	DeleteRowFromReported(
		orgID types.OrgID,
		clusterName types.ClusterName,
		notifiedAt types.Timestamp) (int, error)
	PrintNewReportsForCleanup(maxAge string) error
	CleanupNewReports(maxAge string) (int, error)
	PrintOldReportsForCleanup(maxAge string) error
	CleanupOldReports(maxAge string) (int, error)
}

// DBStorage is an implementation of Storage interface that use selected SQL like database
// like SQLite, PostgreSQL, MariaDB, RDS etc. That implementation is based on the standard
// sql package. It is possible to configure connection via Configuration structure.
// SQLQueriesLog is log for sql queries, default is nil which means nothing is logged
type DBStorage struct {
	connection   *sql.DB
	dbDriverType types.DBDriver
}

// error messages
const (
	unableToCloseDBRowsHandle = "Unable to close DB rows handle"
)

// other messages
const (
	OrgIDMessage         = "Organization ID"
	ClusterNameMessage   = "Cluster name"
	TimestampMessage     = "Timestamp (notified_at/updated_at)"
	AccountNumberMessage = "Account number"
	UpdatedAtMessage     = "Updated at"
	AgeMessage           = "Age"
	MaxAgeAttribute      = "max age"
	DeleteStatement      = "delete statement"
)

// SQL statements
const (
	// Delete older records from new_reports table
	deleteOldRecordsFromNewReportsTable = `
                DELETE
		  FROM new_reports
		 WHERE updated_at < NOW() - $1::INTERVAL
`

	// Delete older records from reported table
	deleteOldRecordsFromReportedTable = `
                DELETE
		  FROM reported
		 WHERE updated_at < NOW() - $1::INTERVAL
`

	// Delete one row from new_reports table for given organization ID, cluster name, and updated at timestamp
	deleteRowFromNewReportsTable = `
                DELETE
		  FROM new_reports
		 WHERE org_id = $1
		   AND cluster = $2
		   AND updated_at = $3
`

	// Delete one row from reported table for given organization ID, cluster name, and notified at timestamp
	deleteRowFromReportedTable = `
                DELETE
		  FROM reported
		 WHERE org_id = $1
		   AND cluster = $2
		   AND notified_at = $3
`

	// Display older records from new_reports table
	displayOldRecordsFromNewReportsTable = `
                SELECT org_id, account_number, cluster, updated_at, kafka_offset
		  FROM new_reports
		 WHERE updated_at < NOW() - $1::INTERVAL
		 ORDER BY updated_at
`

	// Display older records from reported table
	displayOldRecordsFromReportedTable = `
                SELECT org_id, account_number, cluster, updated_at, 0
		  FROM reported
		 WHERE updated_at < NOW() - $1::INTERVAL
		 ORDER BY updated_at
`

	QueryRecordExistsInReadErrors = `
		SELECT exists(SELECT 1 FROM read_errors WHERE org_id=$1 and cluster=$2 and updated_at=$3);
`

	InsertReadErrorsStatement = `
		INSERT INTO read_errors(org_id, cluster, updated_at, created_at, error_text)
		VALUES ($1, $2, $3, $4, $5);
	`

	ReadReportForClusterQuery = `
		SELECT report, updated_at
		  FROM new_reports
		 WHERE org_id = $1 AND cluster = $2
		 ORDER BY updated_at DESC
		 LIMIT 1
       `

	ReadReportForClusterAtOffsetQuery = `
		"SELECT report
		   FROM new_reports
		  WHERE org_id = $1 AND cluster = $2 AND kafka_offset = $3;
       `

	ReadReportForClusterAtTimeQuery = `
		"SELECT report
		   FROM new_reports
		  WHERE org_id = $1 AND cluster = $2 AND updated_at = $3;
       `
)

// inClauseFromStringSlice is a helper function to construct `in` clause for SQL
// statement from a given slice of string items. If the slice is empty, an
// empty string will be returned, making the in clause fail.
func inClauseFromStringSlice(slice []string) string {
	if len(slice) == 0 {
		return ""
	}
	return "'" + strings.Join(slice, `','`) + `'`
}

// NewStorage function creates and initializes a new instance of Storage interface
func NewStorage(configuration conf.StorageConfiguration) (*DBStorage, error) {
	driverType, driverName, dataSource, err := initAndGetDriver(configuration)
	if err != nil {
		return nil, err
	}

	log.Info().Msgf(
		"Making connection to data storage, driver=%s datasource=%s",
		driverName, dataSource,
	)

	connection, err := sql.Open(driverName, dataSource)
	if err != nil {
		log.Error().Err(err).Msg("Can not connect to data storage")
		return nil, err
	}

	return NewFromConnection(connection, driverType), nil
}

// NewFromConnection function creates and initializes a new instance of Storage interface from prepared connection
func NewFromConnection(connection *sql.DB, dbDriverType types.DBDriver) *DBStorage {
	return &DBStorage{
		connection:   connection,
		dbDriverType: dbDriverType,
	}
}

// initAndGetDriver initializes driver(with logs if logSQLQueries is true),
// checks if it's supported and returns driver type, driver name, dataSource and error
func initAndGetDriver(configuration conf.StorageConfiguration) (driverType types.DBDriver, driverName, dataSource string, err error) {
	driverName = configuration.Driver

	switch driverName {
	case "sqlite3":
		driverType = types.DBDriverSQLite3
	case "postgres":
		driverType = types.DBDriverPostgres
		dataSource = fmt.Sprintf(
			"postgresql://%v:%v@%v:%v/%v?%v",
			configuration.PGUsername,
			configuration.PGPassword,
			configuration.PGHost,
			configuration.PGPort,
			configuration.PGDBName,
			configuration.PGParams,
		)
	default:
		err = fmt.Errorf("driver %v is not supported", driverName)
		return
	}

	return
}

// Close method closes the connection to database. Needs to be called at the end of application lifecycle.
func (storage DBStorage) Close() error {
	log.Info().Msg("Closing connection to data storage")
	if storage.connection != nil {
		err := storage.connection.Close()
		if err != nil {
			log.Error().Err(err).Msg("Can not close connection to data storage")
			return err
		}
	}
	return nil
}

// ReadNotificationTypes method read all notification types from the database.
func (storage DBStorage) ReadNotificationTypes() ([]types.NotificationType, error) {
	var notificationTypes = make([]types.NotificationType, 0)

	rows, err := storage.connection.Query("SELECT id, value, frequency, comment FROM notification_types ORDER BY id")
	if err != nil {
		return notificationTypes, err
	}

	defer func() {
		err := rows.Close()
		if err != nil {
			log.Error().Err(err).Msg(unableToCloseDBRowsHandle)
		}
	}()

	for rows.Next() {
		var (
			id        types.NotificationTypeID
			value     string
			frequency string
			comment   string
		)

		if err := rows.Scan(&id, &value, &frequency, &comment); err != nil {
			if closeErr := rows.Close(); closeErr != nil {
				log.Error().Err(closeErr).Msg(unableToCloseDBRowsHandle)
			}
			return notificationTypes, err
		}
		notificationTypes = append(notificationTypes, types.NotificationType{
			ID: id, Value: value, Frequency: frequency, Comment: comment})
	}

	return notificationTypes, nil
}

// ReadStates method reads all possible notification states from the database.
func (storage DBStorage) ReadStates() ([]types.State, error) {
	var states = make([]types.State, 0)

	rows, err := storage.connection.Query("SELECT id, value, comment FROM states ORDER BY id")
	if err != nil {
		return states, err
	}

	defer func() {
		err := rows.Close()
		if err != nil {
			log.Error().Err(err).Msg(unableToCloseDBRowsHandle)
		}
	}()

	for rows.Next() {
		var (
			id      types.StateID
			value   string
			comment string
		)

		if err := rows.Scan(&id, &value, &comment); err != nil {
			if closeErr := rows.Close(); closeErr != nil {
				log.Error().Err(closeErr).Msg(unableToCloseDBRowsHandle)
			}
			return states, err
		}
		states = append(states, types.State{
			ID: id, Value: value, Comment: comment})
	}

	return states, nil
}

// ReadClusterList method creates list of clusters from all the rows in new_reports table.
func (storage DBStorage) ReadClusterList() ([]types.ClusterEntry, error) {
	var clusterList = make([]types.ClusterEntry, 0)

	rows, err := storage.connection.Query(`
		SELECT DISTINCT ON (cluster)
		org_id, account_number, cluster, kafka_offset, updated_at
		FROM new_reports
		ORDER BY cluster, updated_at DESC`)
	if err != nil {
		return clusterList, err
	}

	defer func() {
		err := rows.Close()
		if err != nil {
			log.Error().Err(err).Msg(unableToCloseDBRowsHandle)
		}
	}()

	for rows.Next() {
		var (
			clusterName   types.ClusterName
			orgID         types.OrgID
			accountNumber types.AccountNumber
			kafkaOffset   types.KafkaOffset
			updatedAt     types.Timestamp
		)

		if err := rows.Scan(&orgID, &accountNumber, &clusterName, &kafkaOffset, &updatedAt); err != nil {
			if closeErr := rows.Close(); closeErr != nil {
				log.Error().Err(closeErr).Msg(unableToCloseDBRowsHandle)
			}
			return clusterList, err
		}
		clusterList = append(clusterList, types.ClusterEntry{
			OrgID:         orgID,
			AccountNumber: accountNumber,
			ClusterName:   clusterName,
			KafkaOffset:   kafkaOffset,
			UpdatedAt:     updatedAt})
	}

	return clusterList, nil
}

// ReadReportForClusterAtTime reads result (health status) for selected cluster
// and given timestamp. It is possible that there are more reports stored for a
// cluster, so it is needed to specify timestamp to select just one such
// report.
//
// See also: ReadReportForClusterAtOffset
func (storage DBStorage) ReadReportForClusterAtTime(
	orgID types.OrgID, clusterName types.ClusterName,
	updatedAt types.Timestamp,
) (types.ClusterReport, error) {
	var report types.ClusterReport

	// explicit conversion is needed there - SQL library issue
	timestamp := time.Time(updatedAt)

	query := ReadReportForClusterAtTimeQuery
	err := storage.connection.QueryRow(
		query, orgID, clusterName, timestamp,
	).Scan(&report)

	if err != nil {
		return report, err
	}

	return report, nil
}

// ReadReportForClusterAtOffset reads result (health status) for selected cluster
// and given Kafka offset. It is possible that there are more reports stored for a
// cluster, so it is needed to specify Kafka offset to select just one such
// report.
//
// See also: ReadReportForClusterAtTime
func (storage DBStorage) ReadReportForClusterAtOffset(
	orgID types.OrgID, clusterName types.ClusterName,
	kafkaOffset types.KafkaOffset,
) (types.ClusterReport, error) {
	var report types.ClusterReport

	query := ReadReportForClusterAtOffsetQuery
	err := storage.connection.QueryRow(
		query, orgID, clusterName, kafkaOffset,
	).Scan(&report)

	if err != nil {
		return report, err
	}

	return report, nil
}

// ReadReportForCluster reads the latest result (health status) for selected
// cluster
func (storage DBStorage) ReadReportForCluster(
	orgID types.OrgID, clusterName types.ClusterName,
) (types.ClusterReport, types.Timestamp, error) {
	var updatedAt types.Timestamp
	var report types.ClusterReport

	// query to retrieve just the latest report for given cluster
	query := ReadReportForClusterQuery
	err := storage.connection.QueryRow(query, orgID, clusterName).Scan(&report, &updatedAt)

	if err != nil {
		return report, updatedAt, err
	}

	return report, updatedAt, nil
}

// WriteNotificationRecord method writes a report (with given state and
// notification type) into the database table `reported`. Data for several
// columns are passed via NotificationRecord structure.
//
// See also: WriteNotificationRecordForCluster, WriteNotificationRecordImpl
func (storage DBStorage) WriteNotificationRecord(
	notificationRecord types.NotificationRecord) error {

	return storage.WriteNotificationRecordImpl(notificationRecord.OrgID,
		notificationRecord.AccountNumber, notificationRecord.ClusterName,
		notificationRecord.NotificationTypeID, notificationRecord.StateID,
		notificationRecord.Report, notificationRecord.UpdatedAt,
		notificationRecord.NotifiedAt, notificationRecord.ErrorLog,
		types.NotificationBackendTarget)
}

// WriteNotificationRecordImpl method writes a report (with given state and
// notification type) into the database table `reported`. Data for all columns
// are passed explicitly.
//
// See also: WriteNotificationRecord, WriteNotificationRecordForCluster
func (storage DBStorage) WriteNotificationRecordImpl(
	orgID types.OrgID,
	accountNumber types.AccountNumber,
	clusterName types.ClusterName,
	notificationTypeID types.NotificationTypeID,
	stateID types.StateID,
	report types.ClusterReport,
	updatedAt types.Timestamp,
	notifiedAt types.Timestamp,
	errorLog string,
	eventTarget types.EventTarget) error {

	const insertStatement = `
            INSERT INTO reported
            (org_id, account_number, cluster, notification_type, state, report, updated_at, notified_at, error_log, event_type_id)
            VALUES
            ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
        `
	_, err := storage.connection.Exec(insertStatement, orgID,
		accountNumber, clusterName, notificationTypeID, stateID,
		report, time.Time(updatedAt), time.Time(notifiedAt), errorLog, eventTarget)

	return err
}

// WriteNotificationRecordForCluster method writes a report (with given state
// and notification type) into the database table `reported`. Data for several
// columns are passed via ClusterEntry structure (as returned by
// ReadReportForClusterAtTime and ReadReportForClusterAtOffset methods).
//
// See also: WriteNotificationRecord, WriteNotificationRecordImpl
func (storage DBStorage) WriteNotificationRecordForCluster(
	clusterEntry types.ClusterEntry,
	notificationTypeID types.NotificationTypeID,
	stateID types.StateID,
	report types.ClusterReport,
	notifiedAt types.Timestamp,
	errorLog string,
	eventTarget types.EventTarget) error {

	return storage.WriteNotificationRecordImpl(clusterEntry.OrgID,
		clusterEntry.AccountNumber, clusterEntry.ClusterName,
		notificationTypeID, stateID, report, clusterEntry.UpdatedAt,
		notifiedAt, errorLog, eventTarget)
}

// ReadErrorExists method checks if read_errors table contains given
// combination org_id+cluster_name+updated_at
func (storage DBStorage) ReadErrorExists(
	orgID types.OrgID,
	clusterName types.ClusterName,
	lastCheckedTime time.Time,
) (bool, error) {
	// perform query
	rows, err := storage.connection.Query(QueryRecordExistsInReadErrors, orgID, clusterName, lastCheckedTime)

	// check for any error during query
	if err != nil {
		return false, err
	}

	// be sure to close result set properly
	defer func() {
		err := rows.Close()
		if err != nil {
			log.Error().Err(err).Msg(unableToCloseDBRowsHandle)
		}
	}()

	// only one record should returned
	if rows.Next() {
		var exists bool
		err := rows.Scan(&exists)
		if err != nil {
			return false, err
		}
		return exists, nil
	}

	return false, errors.New("Unable to read from table read_errors")
}

// WriteReadError method writes information about read error into table
// read_errors.
func (storage DBStorage) WriteReadError(
	orgID types.OrgID,
	clusterName types.ClusterName,
	lastCheckedTime time.Time,
	e error,
) error {
	if storage.dbDriverType != types.DBDriverSQLite3 && storage.dbDriverType != types.DBDriverPostgres {
		return fmt.Errorf("writing error with DB %v is not supported", storage.dbDriverType)
	}

	createdAt := time.Now()
	errorText := e.Error()

	_, err := storage.connection.Exec(InsertReadErrorsStatement, orgID, clusterName, lastCheckedTime, createdAt, errorText)
	if err != nil {
		log.Err(err).
			Int("org", int(orgID)).
			Str("cluster", string(clusterName)).
			Str("last checked", lastCheckedTime.String()).
			Str("created at", createdAt.String()).
			Str("error text", errorText).
			Msg("Unable to write record into read_error table")
		return err
	}
	return nil
}

// ReadLastNotifiedRecordForClusterList method returns the last notification
// with state = 'sent' for given event target, organization IDs and clusters.
func (storage DBStorage) ReadLastNotifiedRecordForClusterList(
	clusterEntries []types.ClusterEntry, timeOffset string, eventTarget types.EventTarget,
) (types.NotifiedRecordsPerCluster, error) {
	if len(clusterEntries) == 0 {
		return types.NotifiedRecordsPerCluster{}, nil
	}
	if strings.TrimSpace(timeOffset) == "" || strings.Split(timeOffset, " ")[0] == "0" {
		return types.NotifiedRecordsPerCluster{}, nil
	}

	orgIDs := make([]string, len(clusterEntries))
	clusterIDs := make([]string, len(clusterEntries))

	for idx, entry := range clusterEntries {
		orgIDs[idx] = strconv.FormatUint(uint64(entry.OrgID), 10)
		clusterIDs[idx] = string(entry.ClusterName)
	}

	selectQuery := `SELECT org_id, cluster, report, notified_at FROM ( SELECT DISTINCT ON (cluster) * FROM reported`
	whereClause := fmt.Sprintf(` WHERE event_type_id = %v AND state = 1 AND org_id IN (%v) AND cluster IN (%v)`,
		eventTarget, inClauseFromStringSlice(orgIDs), inClauseFromStringSlice(clusterIDs))
	orderByClause := `ORDER BY cluster, notified_at DESC) t`
	filterByTimeClause := "WHERE notified_at > NOW() - $1::INTERVAL"

	query := strings.Join([]string{selectQuery, whereClause, orderByClause, filterByTimeClause, ";"}, " ")
	rows, err := storage.connection.Query(query, timeOffset)

	if err != nil {
		return nil, err
	}
	defer func() {
		err := rows.Close()
		if err != nil {
			log.Error().Err(err).Msg(unableToCloseDBRowsHandle)
		}
	}()

	notificationRecords := make(types.NotifiedRecordsPerCluster)

	for rows.Next() {
		var (
			orgID       types.OrgID
			clusterName types.ClusterName
			report      types.ClusterReport
			notifiedAt  types.Timestamp
		)

		err := rows.Scan(&orgID, &clusterName, &report, &notifiedAt)
		if err != nil {
			if closeErr := rows.Close(); closeErr != nil {
				log.Error().Err(closeErr).Msg(unableToCloseDBRowsHandle)
			}
			return notificationRecords, err
		}
		key := types.ClusterOrgKey{OrgID: orgID, ClusterName: clusterName}
		notificationRecords[key] = types.NotificationRecord{
			OrgID:       orgID,
			ClusterName: clusterName,
			Report:      report,
			NotifiedAt:  notifiedAt,
		}
	}

	return notificationRecords, nil
}

// DeleteRowFromNewReports deletes one selected row from `new_reports` table.
// Number of deleted rows (zero or one) is returned.
func (storage DBStorage) DeleteRowFromNewReports(
	orgID types.OrgID,
	clusterName types.ClusterName,
	updatedAt types.Timestamp) (int, error) {

	sqlStatement := deleteRowFromNewReportsTable
	return storage.deleteRowImpl(sqlStatement, orgID, clusterName, updatedAt)
}

// DeleteRowFromReported deletes one selected row from `reported` table.
// Number of deleted rows (zero or one) is returned.
func (storage DBStorage) DeleteRowFromReported(
	orgID types.OrgID,
	clusterName types.ClusterName,
	notifiedAt types.Timestamp) (int, error) {

	sqlStatement := deleteRowFromReportedTable
	return storage.deleteRowImpl(sqlStatement, orgID, clusterName, notifiedAt)
}

// getPrintableStatement returns SQL statement in form prepared for logging
func getPrintableStatement(sqlStatement string) string {
	s := strings.ReplaceAll(sqlStatement, "\n", " ")
	s = strings.ReplaceAll(s, "\t", "")
	return strings.Trim(s, " ")
}

// deleteRowImpl method deletes one row from selected table.
// Number of deleted rows (zero or one) is returned.
func (storage DBStorage) deleteRowImpl(
	sqlStatement string,
	orgID types.OrgID,
	clusterName types.ClusterName,
	timestamp types.Timestamp) (int, error) {

	printableStatement := getPrintableStatement(sqlStatement)

	log.Info().
		Str("delete one row", printableStatement).
		Int(OrgIDMessage, int(orgID)).
		Str(ClusterNameMessage, string(clusterName)).
		Str(TimestampMessage, time.Time(timestamp).String()).
		Msg("Selected row cleanup")

	// perform the SQL statement to delete one row
	result, err := storage.connection.Exec(sqlStatement, int(orgID), string(clusterName), time.Time(timestamp))
	if err != nil {
		return 0, err
	}

	// read number of affected (deleted) rows
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(affected), nil
}

// PrintNewReports method prints all reports from selected table older than
// specified relative time
func (storage DBStorage) PrintNewReports(maxAge, query, tableName string) error {
	log.Info().
		Str(MaxAgeAttribute, maxAge).
		Str("select statement", query).
		Msg("PrintReportsForCleanup operation")

	rows, err := storage.connection.Query(query, maxAge)
	if err != nil {
		return err
	}
	// used to compute a real record age
	now := time.Now()

	// iterate over all old records
	for rows.Next() {
		var (
			orgID         int
			accountNumber int
			clusterName   string
			updatedAt     time.Time
			kafkaOffset   int64
		)

		// read one old record from the report table
		if err := rows.Scan(&orgID, &accountNumber, &clusterName, &updatedAt, &kafkaOffset); err != nil {
			// close the result set in case of any error
			if closeErr := rows.Close(); closeErr != nil {
				log.Error().Err(closeErr).Msg(unableToCloseDBRowsHandle)
			}
			return err
		}

		// compute the real record age
		age := int(math.Ceil(now.Sub(updatedAt).Hours() / 24)) // in days

		// prepare for the report
		updatedAtF := updatedAt.Format(time.RFC3339)

		// just print the report
		log.Info().
			Int(OrgIDMessage, orgID).
			Int(AccountNumberMessage, accountNumber).
			Str(ClusterNameMessage, clusterName).
			Str(UpdatedAtMessage, updatedAtF).
			Int(AgeMessage, age).
			Msg("Old report from `" + tableName + "` table")
	}
	return nil
}

// PrintNewReportsForCleanup method prints all reports from `new_reports` table
// older than specified relative time
func (storage DBStorage) PrintNewReportsForCleanup(maxAge string) error {
	return storage.PrintNewReports(maxAge, displayOldRecordsFromNewReportsTable, "new_reports")
}

// PrintOldReportsForCleanup method prints all reports from `reported` table
// older than specified relative time
func (storage DBStorage) PrintOldReportsForCleanup(maxAge string) error {
	return storage.PrintNewReports(maxAge, displayOldRecordsFromReportedTable, "reported")
}

// Cleanup method deletes all reports older than specified
// relative time
func (storage DBStorage) Cleanup(maxAge, statement string) (int, error) {
	log.Info().
		Str(MaxAgeAttribute, maxAge).
		Str(DeleteStatement, statement).
		Msg("Cleanup operation for all organizations")

	// perform the SQL statement
	result, err := storage.connection.Exec(statement, maxAge)
	if err != nil {
		return 0, err
	}

	// read number of affected (deleted) rows
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(affected), nil
}

// CleanupNewReports method deletes all reports from `new_reports` table older
// than specified relative time
func (storage DBStorage) CleanupNewReports(maxAge string) (int, error) {
	return storage.Cleanup(maxAge, deleteOldRecordsFromNewReportsTable)
}

// CleanupOldReports method deletes all reports from `reported` table older
// than specified relative time
func (storage DBStorage) CleanupOldReports(maxAge string) (int, error) {
	return storage.Cleanup(maxAge, deleteOldRecordsFromReportedTable)
}
