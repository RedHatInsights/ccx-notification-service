// Copyright 2021 Red Hat, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"errors"
	"fmt"

	_ "github.com/lib/pq"           // PostgreSQL database driver
	_ "github.com/mattn/go-sqlite3" // SQLite database driver

	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/types"
	"github.com/rs/zerolog/log"
)

// Storage represents an interface to almost any database or storage system
type Storage interface {
	Close() error
	ReadReportForCluster(
		orgID types.OrgID, clusterName types.ClusterName) (types.ClusterReport, types.Timestamp, error,
	)
	ReadClusterList() ([]types.ClusterEntry, error)
}

// DBStorage is an implementation of Storage interface that use selected SQL like database
// like SQLite, PostgreSQL, MariaDB, RDS etc. That implementation is based on the standard
// sql package. It is possible to configure connection via Configuration structure.
// SQLQueriesLog is log for sql queries, default is nil which means nothing is logged
type DBStorage struct {
	connection   *sql.DB
	dbDriverType types.DBDriver
}

// ErrOldReport is an error returned if a more recent already
// exists on the storage while attempting to write a report for a cluster.
var ErrOldReport = errors.New("More recent report already exists in storage")

// error messages
const (
	unableToCloseDBRowsHandle = "Unable to close DB rows handle"
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
		clusterList = append(clusterList, types.ClusterEntry{orgID, accountNumber, clusterName, kafkaOffset, updatedAt})
	}

	return clusterList, nil
}

// ReadReportForCluster reads result (health status) for selected cluster
func (storage DBStorage) ReadReportForCluster(
	orgID types.OrgID, clusterName types.ClusterName,
) (types.ClusterReport, types.Timestamp, error) {
	var updatedAt types.Timestamp
	var report types.ClusterReport

	err := storage.connection.QueryRow(
		"SELECT report, updated_at FROM new_reports WHERE org_id = $1 AND cluster = $2;", orgID, clusterName,
	).Scan(&report, &updatedAt)

	if err != nil {
		return report, updatedAt, err
	}

	return report, updatedAt, nil
}
