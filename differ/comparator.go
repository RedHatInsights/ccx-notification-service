/*
Copyright © 2021, 2022 Red Hat, Inc.

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
	"encoding/json"
	"os"

	"github.com/rs/zerolog/log"

	"github.com/RedHatInsights/ccx-notification-service/types"
)

// Messages
const (
	clusterStr       = "cluster"
	resolutionKey    = "resolution"
	resolutionMsg    = "Should notify user"
	resolutionReason = "reason"

	notificationTypeInstant = "instant"

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

func getNotificationResolution(issue *types.EvaluatedReportItem, record *types.NotificationRecord) (resolution bool) {
	// check if detected issue was included in older report
	var oldReport types.Report
	err := json.Unmarshal([]byte(record.Report), &oldReport)
	if err != nil {
		log.Err(err).Msgf(
			"Deserialization error - Couldn't create issue object for older issue\n %s",
			string(record.Report))
		os.Exit(ExitStatusStorageError)
	}
	resolution = IssueNotInReport(oldReport, issue)
	return
}

// ShouldNotify asserts whether an issue has already been sent in a previous
// notification event
func (d *Differ) ShouldNotify(cluster types.ClusterEntry, issue *types.EvaluatedReportItem) bool {
	key := types.ClusterOrgKey{OrgID: cluster.OrgID, ClusterName: cluster.ClusterName}
	reported, ok := d.PreviouslyReported[key]
	if !ok {
		log.Info().Bool(resolutionKey, true).Str(resolutionReason, "report not in cool down").Msg(resolutionMsg)
		return true
	}

	return getNotificationResolution(issue, &reported)
}

func updateNotificationRecordSameState(storage Storage, cluster types.ClusterEntry, report types.ClusterReport, notifiedAt types.Timestamp, eventTarget types.EventTarget) {
	log.Debug().Msgf("No new issues to notify for cluster %s", cluster.ClusterName)
	NotificationNotSentSameState.Inc()
	// store notification info about not sending the notification
	err := storage.WriteNotificationRecordForCluster(cluster, notificationTypes.Instant, states.SameState, report, notifiedAt, "", eventTarget)
	if err != nil {
		writeNotificationRecordFailed(err)
	}
}

func updateNotificationRecordSentState(storage Storage, cluster types.ClusterEntry, report types.ClusterReport, notifiedAt types.Timestamp, eventTarget types.EventTarget) {
	log.Debug().Msgf("New issues notified for cluster %s", string(cluster.ClusterName))
	NotificationSent.Inc()
	err := storage.WriteNotificationRecordForCluster(cluster, notificationTypes.Instant, states.SentState, report, notifiedAt, "", eventTarget)
	if err != nil {
		writeNotificationRecordFailed(err)
	}
}

func updateNotificationRecordErrorState(storage Storage, err error, cluster types.ClusterEntry, report types.ClusterReport, notifiedAt types.Timestamp, eventTarget types.EventTarget) {
	log.Error().Err(err).Str(clusterStr, string(cluster.ClusterName)).Msg("New issues couldn't be notified")
	NotificationNotSentErrorState.Inc()
	err = storage.WriteNotificationRecordForCluster(cluster, notificationTypes.Instant, states.ErrorState, report, notifiedAt, err.Error(), eventTarget)
	if err != nil {
		writeNotificationRecordFailed(err)
	}
}

func updateNotificationRecordState(storage Storage, cluster types.ClusterEntry, report types.ClusterReport, numEvents int, notifiedAt types.Timestamp, eventTarget types.EventTarget, err error) {
	switch {
	case err != nil:
		log.Warn().Int("issues notified so far", numEvents).Msg("Error sending notification events")
		updateNotificationRecordErrorState(storage, err, cluster, report, notifiedAt, eventTarget)
	case numEvents == 0:
		updateNotificationRecordSameState(storage, cluster, report, notifiedAt, eventTarget)
	case numEvents > 0:
		updateNotificationRecordSentState(storage, cluster, report, notifiedAt, eventTarget)
	}
}

func writeNotificationRecordFailed(err error) {
	log.Error().Err(err).Msg("Write notification record failed")
}

// IssuesEqual compares two issues from reports
func IssuesEqual(issue1, issue2 *types.EvaluatedReportItem) bool {
	/* Removing the Details' comparison as a fix for https://issues.redhat.com/browse/CCXDEV-10817*/
	if issue1.Type == issue2.Type &&
		issue1.Module == issue2.Module &&
		issue1.ErrorKey == issue2.ErrorKey { /* &&
		bytes.Equal(issue1.Details, issue2.Details) */
		return true
	}
	return false
}

// IssueNotInReport searches for a specific issue in given OCP report.
// It returns a boolean flag indicating that the report does not
// contain the issue and thus user needs to be informed about it.
func IssueNotInReport(oldReport types.Report, issue *types.EvaluatedReportItem) bool {
	for _, oldIssue := range oldReport.Reports {
		if IssuesEqual(oldIssue, issue) {
			log.Debug().Bool(resolutionKey, false).Str(resolutionReason, "issue found in previously notified report").Msg(resolutionMsg)
			return false
		}
	}
	log.Info().Bool(resolutionKey, true).Str(resolutionReason, "issue not found in previously notified report").Msg(resolutionMsg)
	return true
}

func getNotificationTypes(storage Storage) error {
	rawNotificationTypes, err := storage.ReadNotificationTypes()
	if err != nil {
		return err
	}
	notificationTypes = types.NotificationTypes{
		Instant: getNotificationType(rawNotificationTypes, notificationTypeInstant),
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
