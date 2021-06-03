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
	//"fmt"
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
	clusterName            = "cluster"
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

func getState(states []types.State, value string) types.StateID {
	for _, state := range states {
		if state.Value == value {
			return state.ID
		}
	}
	return -1
}

func getNotificationType(notificationTypes []types.NotificationType, value string) types.NotificationTypeID {
	for _, notificatinType := range notificationTypes {
		if notificatinType.Value == value {
			return notificatinType.ID
		}
	}
	return -1
}

func process(storage Storage, states types.States, notificationTypes types.NotificationTypes) {
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
		UpdatedAt:   timestamp,
	}

	// try to read older report for a cluster
	reported, err := storage.ReadLastNNotificationRecords(c, 1)
	if err != nil {
		log.Error().Err(err).Msg("Read last n reports")
	}

	notifiedAt := types.Timestamp(time.Now())

	// check if the new result has been stored for brand new cluster
	if len(reported) == 0 {
		log.Info().Str(clusterName, string(c.ClusterName)).Msg("New cluster -> send instant report")
		processNotificationForCluster(storage, c, rNewAsString, notificationTypes, states, notifiedAt)
		// and we are done!
		return
	}

	// it is not a brand new cluster -> compare new report with older one
	rOldAsString := reported[0].Report
	notify, err := CompareReports(rOldAsString, rNewAsString)
	if err != nil {
		log.Error().Err(err).Msg("Unable to compare old and new reports")
	}
	log.Info().Bool("resolution", notify).Msg("Should notify user")

	// if new report differs from the older one -> send notification
	if notify {
		log.Info().Str(clusterName, string(c.ClusterName)).Msg("Different report from the last one")
		processNotificationForCluster(storage, c, rNewAsString, notificationTypes, states, notifiedAt)
		// and we are done!
	} else {
		log.Info().Str(clusterName, string(c.ClusterName)).Msg("Same report as before")
		// store notification info about not sending the notification
		err := storage.WriteNotificationRecordForCluster(c, notificationTypes.Daily, states.SameState, rNewAsString, notifiedAt, "")
		if err != nil {
			writeNotificationRecordFailed(err)
		}
	}
}

func processNotificationForCluster(storage Storage, c types.ClusterEntry,
	rNewAsString types.ClusterReport,
	notificationTypes types.NotificationTypes, states types.States,
	notifiedAt types.Timestamp) {
	instantIssue, err := fakeProcessReport(rNewAsString)
	if err != nil {
		err := storage.WriteNotificationRecordForCluster(c, notificationTypes.Daily, states.ErrorState, rNewAsString, notifiedAt, err.Error())
		if err != nil {
			writeNotificationRecordFailed(err)
		}
	}

	if instantIssue {
		err := storage.WriteNotificationRecordForCluster(c, notificationTypes.Daily, states.SentState, rNewAsString, notifiedAt, "")
		if err != nil {
			writeNotificationRecordFailed(err)
		}
	} else {
		err := storage.WriteNotificationRecordForCluster(c, notificationTypes.Daily, states.SameState, rNewAsString, notifiedAt, "")
		if err != nil {
			writeNotificationRecordFailed(err)
		}
	}
}

func writeNotificationRecordFailed(err error) {
	log.Error().Err(err).Msg("Write notification record failed")
}

func fakeProcessReport(rNewAsString types.ClusterReport) (bool, error) {
	return true, nil
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

func getNotificationTypes(storage Storage) (types.NotificationTypes, error) {
	rawNotificationTypes, err := storage.ReadNotificationTypes()
	if err != nil {
		return types.NotificationTypes{}, err
	}
	notificationTypes := types.NotificationTypes{
		Instant: getNotificationType(rawNotificationTypes, "daily"),
		Weekly:  getNotificationType(rawNotificationTypes, "weekly"),
	}
	return notificationTypes, nil
}

func getStates(storage Storage) (types.States, error) {
	rawStates, err := storage.ReadStates()
	if err != nil {
		return types.States{}, err
	}
	states := types.States{
		SameState:       getState(rawStates, "same"),
		SentState:       getState(rawStates, "sent"),
		LowerIssueState: getState(rawStates, "lower"),
		ErrorState:      getState(rawStates, "error"),
	}
	return states, nil
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

	states, err := getStates(storage)
	if err != nil {
		log.Err(err).Msg("Read states")
		return
	}

	notificationTypes, err := getNotificationTypes(storage)
	if err != nil {
		log.Err(err).Msg("Read notification types")
		return
	}

	process(storage, states, notificationTypes)

	err = storage.Close()
	if err != nil {
		log.Err(err).Msg(operationFailedMessage)
		os.Exit(ExitStatusStorageError)
	}

	log.Debug().Msg("Finished")
}
