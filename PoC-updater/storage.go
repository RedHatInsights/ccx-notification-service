/*
Copyright © 2020 Red Hat, Inc.

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
	//"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"           // PostgreSQL database driver
	_ "github.com/mattn/go-sqlite3" // SQLite database driver

	"github.com/RedHatInsights/ccx-notification-service/conf"
	//"github.com/RedHatInsights/ccx-notification-service/types"
	"github.com/rs/zerolog/log"
	"updater/types"
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
	ReadLastNNotificationRecords(
		clusterEntry types.ClusterEntry,
		numberOfRecords int) ([]types.NotificationRecord, error)
	WriteNotificationRecord(
		notificationRecord types.NotificationRecord) error
	WriteNotificationRecordForCluster(
		clusterEntry types.ClusterEntry,
		notificationTypeID types.NotificationTypeID,
		stateID types.StateID,
		report types.ClusterReport,
		notifiedAt types.Timestamp,
		errorLog string) error
	WriteNotificationRecordImpl(
		orgID types.OrgID,
		accountNumber types.AccountNumber,
		clusterName types.ClusterName,
		notificationTypeID types.NotificationTypeID,
		stateID types.StateID,
		report types.ClusterReport,
		updatedAt types.Timestamp,
		notifiedAt types.Timestamp,
		errorLog string) error
	CleanupNewReportsForOrganization(orgID types.OrgID, maxAge string) (int, error)
	CleanupOldReportsForOrganization(orgID types.OrgID, maxAge string) (int, error)
	DeleteRowFromNewReports(
		orgID types.OrgID,
		clusterName types.ClusterName,
		updatedAt types.Timestamp) (int, error)
	DeleteRowFromReported(
		orgID types.OrgID,
		clusterName types.ClusterName,
		notifiedAt types.Timestamp) (int, error)
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
	OrgIDMessage       = "Organization ID"
	ClusterNameMessage = "Cluster name"
	TimestampMessage   = "Timestamp (notified_at/updated_at)"
	MaxAgeAttribute    = "max age"
)

// SQL statements
const (
	// Delete older records from new_reports table for given organization ID
	deleteOldRecordsFromNewReportsTable = `
                DELETE
		  FROM new_reports
		 WHERE org_id = $1
		   AND updated_at < NOW() - $2::INTERVAL 
`

	// Delete older records from reported table for given organization ID
	deleteOldRecordsFromReportedTable = `
                DELETE
		  FROM reported
		 WHERE org_id = $1
		   AND updated_at < NOW() - $2::INTERVAL
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
)

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
func initAndGetDriver(configuration conf.StorageConfiguration) (driverType types.DBDriver, driverName string, dataSource string, err error) {
	//var driver sql_driver.Driver
	driverName = configuration.Driver

	switch driverName {
	case "sqlite3":
		driverType = types.DBDriverSQLite3
		//driver = &sqlite3.SQLiteDriver{}
		// dataSource = configuration.SQLiteDataSource
	case "postgres":
		driverType = types.DBDriverPostgres
		//driver = &pq.Driver{}
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

	rows, err := storage.connection.Query("SELECT org_id, account_number, cluster, kafka_offset, updated_at FROM new_reports ORDER BY updated_at")
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

	err := storage.connection.QueryRow(
		"SELECT report FROM new_reports WHERE org_id = $1 AND cluster = $2 AND updated_at = $3;",
		orgID, clusterName, timestamp,
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

	err := storage.connection.QueryRow(
		"SELECT report FROM new_reports WHERE org_id = $1 AND cluster = $2 AND kafka_offset = $3;",
		orgID, clusterName, kafkaOffset,
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
	query := `
		SELECT report, updated_at
		  FROM new_reports
		 WHERE org_id = $1 AND cluster = $2
		 ORDER BY updated_at DESC
		 LIMIT 1
                `
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
		notificationRecord.NotifiedAt, notificationRecord.ErrorLog)
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
	errorLog string) error {

	const insertStatement = `
            INSERT INTO reported
            (org_id, account_number, cluster, notification_type, state, report, updated_at, notified_at, error_log)
            VALUES
            ($1, $2, $3, $4, $5, $6, $7, $8, $9)
        `
	_, err := storage.connection.Exec(insertStatement, orgID,
		accountNumber, clusterName, notificationTypeID, stateID,
		report, time.Time(updatedAt), time.Time(notifiedAt), errorLog)

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
	errorLog string) error {

	return storage.WriteNotificationRecordImpl(clusterEntry.OrgID,
		clusterEntry.AccountNumber, clusterEntry.ClusterName,
		notificationTypeID, stateID, report, clusterEntry.UpdatedAt,
		notifiedAt, errorLog)
}

// ReadLastNNotificationRecords method returns the last N notification records
// for given org ID and cluster name.
func (storage DBStorage) ReadLastNNotificationRecords(clusterEntry types.ClusterEntry,
	numberOfRecords int) ([]types.NotificationRecord, error) {
	var notificationRecords = make([]types.NotificationRecord, 0)

	query := `
                  select org_id, account_number, cluster, notification_type, state, report, updated_at, notified_at, error_log
		    from reported
		   where org_id = $1 and cluster = $2
		   order by notified_at desc
		   limit $3;
		   `

	rows, err := storage.connection.Query(query, clusterEntry.OrgID, clusterEntry.ClusterName, numberOfRecords)
	if err != nil {
		return notificationRecords, err
	}

	defer func() {
		err := rows.Close()
		if err != nil {
			log.Error().Err(err).Msg(unableToCloseDBRowsHandle)
		}
	}()

	for rows.Next() {
		var (
			orgID              types.OrgID
			accountNumber      types.AccountNumber
			clusterName        types.ClusterName
			updatedAt          types.Timestamp
			notificationTypeID types.NotificationTypeID
			stateID            types.StateID
			report             types.ClusterReport
			notifiedAt         types.Timestamp
			errorLog           string
		)

		err := rows.Scan(&orgID, &accountNumber, &clusterName, &notificationTypeID,
			&stateID, &report, &updatedAt, &notifiedAt, &errorLog)
		if err != nil {
			if closeErr := rows.Close(); closeErr != nil {
				log.Error().Err(closeErr).Msg(unableToCloseDBRowsHandle)
			}
			return notificationRecords, err
		}
		notificationRecords = append(notificationRecords, types.NotificationRecord{
			OrgID:              orgID,
			AccountNumber:      accountNumber,
			ClusterName:        clusterName,
			UpdatedAt:          updatedAt,
			NotificationTypeID: notificationTypeID,
			StateID:            stateID,
			Report:             report,
			NotifiedAt:         notifiedAt,
			ErrorLog:           errorLog,
		})
	}

	return notificationRecords, nil
}

// CleanupForOrganization method deletes all reports older than specified
// relative time for given organization ID.
//
// This method is to be used to cleanup older reports after weekly summary is
// sent.
//
// The method return number of deleted records.
func (storage DBStorage) CleanupForOrganization(orgID types.OrgID, maxAge string, statement string) (int, error) {
	printableStatement := getPrintableStatement(statement)

	log.Info().
		Str(MaxAgeAttribute, maxAge).
		Str("delete statement", printableStatement).
		Int(OrgIDMessage, int(orgID)).
		Msg("Cleanup operation")

	// perform the SQL statement
	result, err := storage.connection.Exec(statement, int(orgID), maxAge)
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

// CleanupNewReportsForOrganization method deletes all reports from
// `new_reports` table older than specified relative time. Delete operation is
// restricted for given organization ID.
//
// This method is to be used to cleanup older reports after weekly summary is
// sent.
//
// The method return number of deleted records.
func (storage DBStorage) CleanupNewReportsForOrganization(orgID types.OrgID, maxAge string) (int, error) {
	sqlStatement := deleteOldRecordsFromNewReportsTable
	return storage.CleanupForOrganization(orgID, maxAge, sqlStatement)
}

// CleanupOldReportsForOrganization method deletes all reports from `reported`
// table older than specified relative time. Delete operation is restricted for
// given organization ID.
//
// This method is to be used to cleanup older reports after weekly summary is
// sent.
//
// The method return number of deleted records.
func (storage DBStorage) CleanupOldReportsForOrganization(orgID types.OrgID, maxAge string) (int, error) {
	sqlStatement := deleteOldRecordsFromReportedTable
	return storage.CleanupForOrganization(orgID, maxAge, sqlStatement)
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
	s := strings.Replace(sqlStatement, "\n", " ", -1)
	s = strings.Replace(s, "\t", "", -1)
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
