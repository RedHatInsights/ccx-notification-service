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

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"updater/types"
)

// Configuration-related constants
const (
	configFileEnvVariableName = "NOTIFICATION_DIFFER_CONFIG_FILE"
	defaultConfigFileName     = "config"
)

// Messages
const (
	operationFailedMessage = "Operation failed"
)

// Exit codes
const (
	// ExitStatusOK means that the tool finished with success
	ExitStatusOK = iota
	// ExitStatusError is a general error code
	ExitStatusError
	// ExitStatusStorageError is returned in case of any consumer-related error
	ExitStatusStorageError
)

func unmarshalReport(reportAsString types.ClusterReport) (types.Report, error) {
	var report types.Report
	err := json.Unmarshal([]byte(reportAsString), &report)
	if err != nil {
		log.Err(err).Msg("Unmarshal")
		return report, err
	}
	return report, err
}

func process(storage Storage) {
	const testedOrgID = 1
	const testedCluster = "11111111-1111-1111-1111-111111111111"

	// read newest report for a cluster
	rNewAsString, timestamp, err := storage.ReadReportForCluster(testedOrgID, testedCluster)
	if err != nil {
		log.Error().Err(err).Msg("Read report for cluster")
		return
	}
	log.Info().Time("time", time.Time(timestamp)).Msg("Read")

	c := types.ClusterEntry{
		OrgID:       testedOrgID,
		ClusterName: testedCluster,
	}

	// read older reports for a cluster
	reported, err := storage.ReadLastNNotificationRecords(c, 10)
	if err != nil {
		log.Error().Err(err).Msg("Read last n reports")
	}
	for _, r := range reported {
		rOldAsString := r.Report
		log.Info().Time("notified at", time.Time(r.NotifiedAt)).Msg("notified record")

		notify, err := CompareReports(rOldAsString, rNewAsString)
		if err != nil {
			log.Error().Err(err).Msg("Unable to compare")
		}
		log.Info().Bool("resolution", notify).Msg("Should notify user")
		fmt.Println()
	}
}

// Function issuesEqual compares two issues from reports
func issuesEqual(issue1 types.ReportItem, issue2 types.ReportItem) bool {
	if issue1.Type == issue2.Type &&
		issue1.Module == issue2.Module &&
		issue1.ErrorKey == issue2.ErrorKey &&
		bytes.Equal(issue1.Details, issue2.Details) {
		return true
	}
	return false
}

// Function CompareReports compare old OCP report with the newest one and
// returns boolean flag indicating that new report contain new issues and thus
// user needs to be informed about them.
func CompareReports(
	oldReportAsString types.ClusterReport,
	newReportAsString types.ClusterReport) (bool, error) {

	// try to unmarshal the old report
	oldReport, err := unmarshalReport(oldReportAsString)
	if err != nil {
		log.Error().Err(err).Msg("Unmarshal old report")
		return false, err
	}

	// try to unmarshal the newest report
	newReport, err := unmarshalReport(newReportAsString)
	if err != nil {
		log.Error().Err(err).Msg("Unmarshal new report")
		return false, err
	}

	// check whether new report contains more issues than the older one
	if len(newReport.Reports) > len(oldReport.Reports) {
		log.Info().Msg("New report contains more issues")
		return true, nil
	}

	// now the number of issues is the same: time to compare them
	for _, issue1 := range oldReport.Reports {
		found := false
		// check if issue1 is found in newest report
		for _, issue2 := range newReport.Reports {
			if issuesEqual(issue1, issue2) {
				found = true
				break
			}
		}
		// issue can't be found -> there must be change user needs to
		// be notified about
		if !found {
			log.Info().
				Str("Type", string(issue1.Type)).
				Str("Module", string(issue1.Module)).
				Str("ErrorKey", string(issue1.ErrorKey)).
				Str("Details", string(issue1.Details)).
				Msg("Not found")
			return true, nil
		}
	}
	// seems like both old report and new report are the same
	return false, nil
}

func main() {
	// config has exactly the same structure as *.toml file
	config, err := conf.LoadConfiguration(configFileEnvVariableName, defaultConfigFileName)
	if err != nil {
		log.Err(err).Msg("Load configuration")
	}

	if config.Logging.Debug {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	log.Debug().Msg("Started")

	// prepare the storage
	storageConfiguration := conf.GetStorageConfiguration(config)
	storage, err := NewStorage(storageConfiguration)
	if err != nil {
		log.Err(err).Msg(operationFailedMessage)
		os.Exit(ExitStatusStorageError)
	}

	process(storage)

	err = storage.Close()
	if err != nil {
		log.Err(err).Msg(operationFailedMessage)
		os.Exit(ExitStatusStorageError)
	}

	log.Debug().Msg("Finished")
}
