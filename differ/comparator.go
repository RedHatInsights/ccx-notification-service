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

// Generated documentation is available at:
// https://pkg.go.dev/github.com/RedHatInsights/ccx-notification-service/differ
//
// Documentation in literate-programming-style is available at:
// https://redhatinsights.github.io/ccx-notification-service/packages/differ/comparator.html

import (
	"bytes"
	"encoding/json"
	"os"

	"github.com/RedHatInsights/ccx-notification-service/types"
	"github.com/rs/zerolog/log"
)

// Messages
const (
	clusterName = "cluster"

	notificationTypeInstant = "instant"
	notificationTypeWeekly  = "weekly"

	notificationStateSent  = "sent"
	notificationStateSame  = "same"
	notificationStateLower = "lower"
	notificationStateError = "error"
)

var (
	states            types.States
	notificationTypes types.NotificationTypes
)

func getState(states []types.State, value string) types.StateID {
	for _, state := range states {
		if state.Value == value {
			return state.ID
		}
	}
	return -1
}

func getNotificationType(notificationTypes []types.NotificationType, value string) types.NotificationTypeID {
	for _, notificationType := range notificationTypes {
		if notificationType.Value == value {
			return notificationType.ID
		}
	}
	return -1
}

func shouldNotify(storage Storage, cluster types.ClusterEntry, issue types.ReportItem) bool {
	// check if the issue of the given cluster has previously be reported
	reported, err := storage.ReadLastNNotificationRecords(cluster, 1)
	if err != nil {
		log.Error().Err(err).Str(clusterName, string(cluster.ClusterName)).Msg("Read last report failed")
	}
	if len(reported) == 0 {
		log.Info().Bool("resolution", true).Msg("Should notify user")
		return true
	}

	// it is not a brand new cluster -> check if issue was included in older report
	var oldReport types.Report
	err = json.Unmarshal([]byte(reported[0].Report), &oldReport)
	if err != nil {
		log.Err(err).Msgf(
			"Deserialization error - Couldn't create issue object for older issue\n %s",
			string(reported[0].Report))
		os.Exit(ExitStatusStorageError)
	}

	notify := IssueNotInReport(oldReport, issue)
	log.Info().Bool("resolution", notify).Msg("Should notify user")
	return notify
}

func updateNotificationRecordSameState(storage Storage, cluster types.ClusterEntry, report types.ClusterReport, notifiedAt types.Timestamp) {
	log.Info().Msgf("No new issues to notify for cluster %s", cluster.ClusterName)
	NotificationNotSentSameState.Inc()
	// store notification info about not sending the notification
	err := storage.WriteNotificationRecordForCluster(cluster, notificationTypes.Instant, states.SameState, report, notifiedAt, "")
	if err != nil {
		writeNotificationRecordFailed(err)
	}
}

func updateNotificationRecordSentState(storage Storage, cluster types.ClusterEntry, report types.ClusterReport, notifiedAt types.Timestamp) {
	log.Info().Msgf("New issues notified for cluster %s", string(cluster.ClusterName))
	NotificationSent.Inc()
	err := storage.WriteNotificationRecordForCluster(cluster, notificationTypes.Instant, states.SentState, report, notifiedAt, "")
	if err != nil {
		writeNotificationRecordFailed(err)
	}
}

func updateNotificationRecordErrorState(storage Storage, err error, cluster types.ClusterEntry, report types.ClusterReport, notifiedAt types.Timestamp) {
	log.Info().Msgf("New issues couldn't be notified for cluster %s", string(cluster.ClusterName))
	NotificationNotSentErrorState.Inc()
	err = storage.WriteNotificationRecordForCluster(cluster, notificationTypes.Instant, states.ErrorState, report, notifiedAt, err.Error())
	if err != nil {
		writeNotificationRecordFailed(err)
	}
}

func writeNotificationRecordFailed(err error) {
	log.Error().Err(err).Msg("Write notification record failed")
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

// IssueNotInReport searches for a specific issue in given OCP report.
// It returns a boolean flag indicating that the report does not
// contain the issue and thus user needs to be informed about it.
func IssueNotInReport(oldReport types.Report, issue types.ReportItem) bool {
	for _, oldIssue := range oldReport.Reports {
		if issuesEqual(oldIssue, issue) {
			return false
		}
	}

	log.Info().Msg("New report does not contain the new issue")
	return true
}

func getNotificationTypes(storage Storage) error {
	rawNotificationTypes, err := storage.ReadNotificationTypes()
	if err != nil {
		return err
	}
	notificationTypes = types.NotificationTypes{
		Instant: getNotificationType(rawNotificationTypes, notificationTypeInstant),
		Weekly:  getNotificationType(rawNotificationTypes, notificationTypeWeekly),
	}
	return nil
}

func getStates(storage Storage) error {
	rawStates, err := storage.ReadStates()
	if err != nil {
		return err
	}
	states = types.States{
		SameState:       getState(rawStates, notificationStateSame),
		SentState:       getState(rawStates, notificationStateSent),
		LowerIssueState: getState(rawStates, notificationStateLower),
		ErrorState:      getState(rawStates, notificationStateError),
	}
	return nil
}
