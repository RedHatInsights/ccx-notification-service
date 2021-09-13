/*
Copyright © 2021 Red Hat, Inc.

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

import (
	"errors"

	"github.com/rs/zerolog/log"

	"github.com/RedHatInsights/ccx-notification-service/types"
)

// Messages
const (
	databasePrintNewReportsForCleanupOperationFailedMessage = "Print records from `new_reports` table prepared for cleanup failed"
	databasePrintOldReportsForCleanupOperationFailedMessage = "Print records from `reported` table prepared for cleanup failed"
	databaseCleanupNewReportsOperationFailedMessage         = "Cleanup records from `new_reports` table failed"
	databaseCleanupOldReportsOperationFailedMessage         = "Cleanup records from `reported` table failed"
	rowsDeletedMessage                                      = "Rows deleted"
)

// PerformCleanupOperation function performs selected cleanup operation
func PerformCleanupOperation(storage *DBStorage, cliFlags types.CliFlags) error {
	switch {
	case cliFlags.PrintNewReportsForCleanup:
		return printNewReportsForCleanup(storage, cliFlags)
	case cliFlags.PerformNewReportsCleanup:
		return performNewReportsCleanup(storage, cliFlags)
	case cliFlags.PrintOldReportsForCleanup:
		return printOldReportsForCleanup(storage, cliFlags)
	case cliFlags.PerformOldReportsCleanup:
		return performOldReportsCleanup(storage, cliFlags)
	default:
		return errors.New("Unknown operation selected")
	}
}

// printNewReportsForCleanup function print all reports for `new_reports` table
// that are older than specified max age.
func printNewReportsForCleanup(storage *DBStorage, cliFlags types.CliFlags) error {
	err := storage.PrintNewReportsForCleanup(cliFlags.MaxAge)
	if err != nil {
		log.Error().Err(err).Msg(databasePrintNewReportsForCleanupOperationFailedMessage)
		return err
	}

	return nil
}

// performNewReportsCleanup function deletes all reports from `new_reports`
// table that are older than specified max age.
func performNewReportsCleanup(storage *DBStorage, cliFlags types.CliFlags) error {
	affected, err := storage.CleanupNewReports(cliFlags.MaxAge)
	if err != nil {
		log.Error().Err(err).Msg(databaseCleanupNewReportsOperationFailedMessage)
		return err
	}
	log.Info().Int(rowsDeletedMessage, affected).Msg("Cleanup `new_reports` finished")

	return nil
}

// printOldReportsForCleanup function print all reports for `reported` table
// that are older than specified max age.
func printOldReportsForCleanup(storage *DBStorage, cliFlags types.CliFlags) error {
	err := storage.PrintOldReportsForCleanup(cliFlags.MaxAge)
	if err != nil {
		log.Error().Err(err).Msg(databasePrintOldReportsForCleanupOperationFailedMessage)
		return err
	}

	return nil
}

// performOldReportsCleanup function deletes all reports from `reported` table
// that are older than specified max age.
func performOldReportsCleanup(storage *DBStorage, cliFlags types.CliFlags) error {
	affected, err := storage.CleanupOldReports(cliFlags.MaxAge)
	if err != nil {
		log.Error().Err(err).Msg(databaseCleanupOldReportsOperationFailedMessage)
		return err
	}
	log.Info().Int(rowsDeletedMessage, affected).Msg("Cleanup `reported` finished")

	return nil
}
