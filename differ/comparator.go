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

func shouldNotify(storage Storage, cluster types.ClusterEntry, report types.Report) bool {
	// try to read older report for a cluster
	reported, err := storage.ReadLastNNotificationRecords(cluster, 1)
	if err != nil {
		log.Error().Err(err).Msg("Read last n reports")
	}

	// check if the new result has been stored for brand new cluster
	if len(reported) == 0 {
		log.Info().Str(clusterName, string(cluster.ClusterName)).Msg("New cluster -> send instant report")
		return true
	}

	// it is not a brand new cluster -> compare new report with older one
	var oldReport types.Report
	err = json.Unmarshal([]byte(reported[0].Report), &oldReport)
	if err != nil {
		log.Err(err).Msgf(
			"Deserialization error - Couldn't create report object for older report\n %s",
			string(reported[0].Report))
		os.Exit(ExitStatusStorageError)
	}
	notify, err := CompareReports(oldReport, report)
	if err != nil {
		log.Error().Err(err).Msg("Unable to compare old and new reports")
	}
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

// CompareReports compares old OCP report with the newest one and
// returns boolean flag indicating that new report contain new issues and thus
// user needs to be informed about them.
func CompareReports(
	oldReport types.Report,
	newReport types.Report) (bool, error) {

	// check whether new report contains more issues than the older one
	if len(newReport.Reports) > len(oldReport.Reports) {
		log.Info().Msg("New report contains more issues")
		return true, nil
	}

	// now the number of issues is the same: time to compare them
	notFoundIssues := len(newReport.Reports)
	for _, issue1 := range oldReport.Reports {
		found := false
		// check if issue1 is found in newest report
		for _, issue2 := range newReport.Reports {
			if issuesEqual(issue1, issue2) {
				found = true
				notFoundIssues--
				break
			}
		}
		// all issues in newReports have been found in oldReports -> return false
		if notFoundIssues == 0 {
			log.Info().Msg("New report does not contain new issues")
			return false, nil
		}
		// issue can't be found in this old report -> there might be change user needs to
		// be notified about. Let's keep the log for now, might help for debug
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
	// Some issues can't be found -> there must be change user needs to
	// be notified about
	if notFoundIssues > 0 {
		log.Info().Msg("All issues in new report have already been reported")
		return true, nil
	}
	// seems like both old report and new report are the same
	return false, nil
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
