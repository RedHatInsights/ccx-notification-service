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
	"github.com/RedHatInsights/ccx-notification-service/tests/mocks"
	"github.com/RedHatInsights/ccx-notification-service/types"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
	"time"
)

var (
	statesList = []types.State{
		{
			ID:      1,
			Value:   notificationStateSent,
			Comment: "sent state",
		},
		{
			ID:      2,
			Value:   notificationStateSame,
			Comment: "same state",
		},
		{
			ID:      3,
			Value:   notificationStateLower,
			Comment: "lower state",
		},
		{
			ID:      4,
			Value:   notificationStateError,
			Comment: "error state",
		},
	}

	notificationTypesList = []types.NotificationType{
		{
			ID:        1,
			Value:     notificationTypeInstant,
			Frequency: "instant",
			Comment:   "instant notifs",
		},
		{
			ID:        2,
			Value:     notificationTypeWeekly,
			Frequency: "weekly",
			Comment:   "weekly notifs",
		},
	}

	testCluster = types.ClusterEntry{
		OrgID:         1,
		AccountNumber: 0123456,
		ClusterName:   "test_cluster1",
		KafkaOffset:   0,
		UpdatedAt:     types.Timestamp(testTimestamp),
	}
)

func init() {
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	previouslyReported = make(types.NotifiedRecordsPerCluster)
}

func TestGetState(t *testing.T) {
	assert.Equal(t, types.StateID(1), getState(statesList, notificationStateSent))
	assert.Equal(t, types.StateID(2), getState(statesList, notificationStateSame))
	assert.Equal(t, types.StateID(3), getState(statesList, notificationStateLower))
	assert.Equal(t, types.StateID(4), getState(statesList, notificationStateError))
	assert.Equal(t, types.StateID(-1), getState(statesList, "any_other_state"))
}

func TestGetStates(t *testing.T) {
	storage := mocks.Storage{}
	storage.On("ReadStates").Return(
		func() []types.State {
			return statesList
		},
		func() error {
			return nil
		},
	)

	assert.Nil(t, getStates(&storage))
	assert.Equal(t, types.StateID(2), states.SameState)
	assert.Equal(t, types.StateID(1), states.SentState)
	assert.Equal(t, types.StateID(3), states.LowerIssueState)
	assert.Equal(t, types.StateID(4), states.ErrorState)
}

func TestGetNotificationType(t *testing.T) {
	assert.Equal(t, types.NotificationTypeID(1), getNotificationType(notificationTypesList, notificationTypeInstant))
	assert.Equal(t, types.NotificationTypeID(2), getNotificationType(notificationTypesList, notificationTypeWeekly))
	assert.Equal(t, types.NotificationTypeID(-1), getNotificationType(notificationTypesList, "any_other_type"))
}

func TestGetNotifications(t *testing.T) {
	storage := mocks.Storage{}
	storage.On("ReadNotificationTypes").Return(
		func() []types.NotificationType {
			return notificationTypesList
		},
		func() error {
			return nil
		},
	)

	assert.Nil(t, getNotificationTypes(&storage))
	assert.Equal(t, types.NotificationTypeID(1), notificationTypes.Instant)
	assert.Equal(t, types.NotificationTypeID(2), notificationTypes.Weekly)
}

func TestIssuesEqualSameIssues(t *testing.T) {
	issue1 := types.ReportItem{
		Type:     "rule",
		Module:   "ccx_rules_ocp.external.rules.rule_1.report",
		ErrorKey: "SOME_ERROR_KEY",
		Details:  []byte("details of the issue"),
	}
	issue2 := types.ReportItem{
		Type:     "rule",
		Module:   "ccx_rules_ocp.external.rules.rule_1.report",
		ErrorKey: "SOME_ERROR_KEY",
		Details:  []byte("details of the issue"),
	}
	assert.True(t, issuesEqual(issue1, issue2), "Compared issues should be equal")
}

func TestIssuesEqualDifferentDetails(t *testing.T) {
	issue1 := types.ReportItem{
		Type:     "rule",
		Module:   "ccx_rules_ocp.external.rules.rule_1.report",
		ErrorKey: "SOME_ERROR_KEY",
		Details:  []byte("details of the issue"),
	}
	issue2 := types.ReportItem{
		Type:     "rule",
		Module:   "ccx_rules_ocp.external.rules.rule_1.report",
		ErrorKey: "SOME_ERROR_KEY",
		Details:  []byte("details of the issue is different"),
	}
	assert.False(t, issuesEqual(issue1, issue2), "Compared issues should not be equal")
}

func TestIssuesEqualDifferentModule(t *testing.T) {
	issue1 := types.ReportItem{
		Type:     "rule",
		Module:   "ccx_rules_ocp.external.rules.rule_1.report",
		ErrorKey: "SOME_ERROR_KEY",
		Details:  []byte("details of the issue"),
	}
	issue2 := types.ReportItem{
		Type:     "rule",
		Module:   "ccx_rules_ocp.external.rules.rule_2.report",
		ErrorKey: "SOME_ERROR_KEY",
		Details:  []byte("details of the issue"),
	}
	assert.False(t, issuesEqual(issue1, issue2), "Compared issues should not be equal")
}

func TestNewIssueNotInOldReport(t *testing.T) {
	oldReport := types.Report{
		Reports: []types.ReportItem{
			{
				Type:     "rule",
				Module:   "ccx_rules_ocp.external.rules.rule_1.report",
				ErrorKey: "SOME_ERROR_KEY",
				Details:  []byte("details of the issue"),
			},
		},
	}

	issue := types.ReportItem{
		Type:     "rule",
		Module:   "ccx_rules_ocp.external.rules.rule_2.report",
		ErrorKey: "SOME_OTHER_ERROR_KEY",
		Details:  []byte("details of the other issue"),
	}

	assert.True(t,
		IssueNotInReport(oldReport, issue),
		"New issue not in old report old report, so result should be true.",
	)

}

func TestIssueNotInReportSameItemsInNewReport(t *testing.T) {
	oldReport := types.Report{
		Reports: []types.ReportItem{
			{
				Type:     "rule",
				Module:   "ccx_rules_ocp.external.rules.rule_1.report",
				ErrorKey: "SOME_ERROR_KEY",
				Details:  []byte("details of the issue"),
			},
			{
				Type:     "rule",
				Module:   "ccx_rules_ocp.external.rules.rule_2.report",
				ErrorKey: "SOME_OTHER_ERROR_KEY",
				Details:  []byte("details of the other issue"),
			},
		},
	}
	newReport := types.Report{
		Reports: []types.ReportItem{
			{
				Type:     "rule",
				Module:   "ccx_rules_ocp.external.rules.rule_1.report",
				ErrorKey: "SOME_ERROR_KEY",
				Details:  []byte("details of the issue"),
			},
			{
				Type:     "rule",
				Module:   "ccx_rules_ocp.external.rules.rule_2.report",
				ErrorKey: "SOME_OTHER_ERROR_KEY",
				Details:  []byte("details of the other issue"),
			},
		},
	}

	for _, issue := range newReport.Reports {
		assert.False(t,
			IssueNotInReport(oldReport, issue),
			"New report has the same items than old report, so comparison result should be false.",
		)
	}
}

func TestIssueNotInReportSameLengthDifferentItemsInNewReport(t *testing.T) {
	oldReport := types.Report{
		Reports: []types.ReportItem{
			{
				Type:     "rule",
				Module:   "ccx_rules_ocp.external.rules.rule_1.report",
				ErrorKey: "SOME_ERROR_KEY",
				Details:  []byte("details of the issue"),
			},
			{
				Type:     "rule",
				Module:   "ccx_rules_ocp.external.rules.rule_2.report",
				ErrorKey: "SOME_OTHER_ERROR_KEY",
				Details:  []byte("details of the other issue"),
			},
		},
	}
	newReport := types.Report{
		Reports: []types.ReportItem{
			{
				Type:     "rule",
				Module:   "ccx_rules_ocp.external.rules.rule_3.report",
				ErrorKey: "SOME_ERROR_KEY",
				Details:  []byte("details of the issue 3"),
			},
			{
				Type:     "rule",
				Module:   "ccx_rules_ocp.external.rules.rule_4.report",
				ErrorKey: "SOME_OTHER_ERROR_KEY",
				Details:  []byte("details of the other issue 4"),
			},
		},
	}

	for _, issue := range newReport.Reports {
		assert.True(t,
			IssueNotInReport(oldReport, issue),
			"New and old report have different items, so result should be true.",
		)
	}
}

func TestIssueNotInReportLessItemsInNewReportAndIssueNotFoundInOldReports(t *testing.T) {
	oldReport := types.Report{
		Reports: []types.ReportItem{
			{
				Type:     "rule",
				Module:   "ccx_rules_ocp.external.rules.rule_1.report",
				ErrorKey: "SOME_ERROR_KEY",
				Details:  []byte("details of the issue"),
			},
			{
				Type:     "rule",
				Module:   "ccx_rules_ocp.external.rules.rule_2.report",
				ErrorKey: "SOME_OTHER_ERROR_KEY",
				Details:  []byte("details of the other issue"),
			},
		},
	}
	newReport := types.Report{
		Reports: []types.ReportItem{
			{
				Type:     "rule",
				Module:   "ccx_rules_ocp.external.rules.rule_3.report",
				ErrorKey: "SOME_ERROR_KEY",
				Details:  []byte("details of the new issue"),
			},
		},
	}

	for _, issue := range newReport.Reports {
		assert.True(t,
			IssueNotInReport(oldReport, issue),
			"New report's issue has not been found in old report, so result should be true.",
		)
	}
}

func TestIssueNotInReportLessItemsInNewReportAndIssueFoundInOldReports(t *testing.T) {
	oldReport := types.Report{
		Reports: []types.ReportItem{
			{
				Type:     "rule",
				Module:   "ccx_rules_ocp.external.rules.rule_1.report",
				ErrorKey: "SOME_ERROR_KEY",
				Details:  []byte("details of the issue"),
			},
			{
				Type:     "rule",
				Module:   "ccx_rules_ocp.external.rules.rule_2.report",
				ErrorKey: "SOME_OTHER_ERROR_KEY",
				Details:  []byte("details of the other issue"),
			},
		},
	}
	newReport := types.Report{
		Reports: []types.ReportItem{
			{
				Type:     "rule",
				Module:   "ccx_rules_ocp.external.rules.rule_1.report",
				ErrorKey: "SOME_ERROR_KEY",
				Details:  []byte("details of the issue"),
			},
		},
	}

	for _, issue := range newReport.Reports {
		assert.False(t,
			IssueNotInReport(oldReport, issue),
			"New report's issue has been found in old report, so result should be false.",
		)
	}
}

func TestShouldNotifyNoPreviousRecord(t *testing.T) {
	storage := mocks.Storage{}
	storage.On("ReadLastNotifiedRecordForClusterList",
		mock.AnythingOfType("[]types.ClusterEntry")).Return(
		func(clusterEntries []types.ClusterEntry) types.NotifiedRecordsPerCluster {
			return types.NotifiedRecordsPerCluster{}
		},
		func(clusterEntries []types.ClusterEntry) error {
			return nil
		},
	)
	newReport := types.Report{
		Reports: []types.ReportItem{
			{
				Type:     "rule",
				Module:   "ccx_rules_ocp.external.rules.rule_1.report",
				ErrorKey: "SOME_ERROR_KEY",
				Details:  []byte("details of the issue"),
			},
		},
	}

	previouslyReported, _ = storage.ReadLastNotifiedRecordForClusterList(make([]types.ClusterEntry, 0))
	for _, issue := range newReport.Reports {
		assert.True(t, shouldNotify(testCluster, issue))
	}
}

func TestShouldNotifyPreviousRecordForGivenClusterIsIdenticalCooldownNotPassed(t *testing.T) {
	storage := mocks.Storage{}
	storage.On("ReadLastNotifiedRecordForClusterList",
		mock.AnythingOfType("[]types.ClusterEntry")).Return(
		func(clusterEntries []types.ClusterEntry) types.NotifiedRecordsPerCluster {
			return types.NotifiedRecordsPerCluster{
				types.ClusterOrgKey{testCluster.OrgID, testCluster.ClusterName}.ToString(): {
					OrgID:              testCluster.OrgID,
					AccountNumber:      testCluster.AccountNumber,
					ClusterName:        testCluster.ClusterName,
					UpdatedAt:          types.Timestamp(time.Now().Add(1 * time.Hour)),
					NotificationTypeID: 1,
					StateID:            1,
					Report:             "{\"analysis_metadata\":{\"metadata\":\"some metadata\"},\"reports\":[{\"rule_id\":\"rule_4|RULE_4\",\"component\":\"ccx_rules_ocp.external.rules.rule_4.report\",\"type\":\"rule\",\"key\":\"RULE_4\",\"details\":\"the same details\",\"tags\":[],\"links\":{\"kcs\":[\"https://access.redhat.com/solutions/4849711\"]}}]}",
					NotifiedAt:         types.Timestamp(time.Now().Add(1 * time.Hour)),
					ErrorLog:           "",
				},
			}
		},
		func(clusterEntries []types.ClusterEntry) error {
			return nil
		},
	)
	newReport := types.Report{
		Reports: []types.ReportItem{
			{
				Type:     "rule",
				Module:   "ccx_rules_ocp.external.rules.rule_4.report",
				ErrorKey: "RULE_4",
				Details:  []byte("\"the same details\""),
			},
		},
	}

	notificationCooldown = 61 * time.Minute
	previouslyReported, _ = storage.ReadLastNotifiedRecordForClusterList(make([]types.ClusterEntry, 0))

	for _, issue := range newReport.Reports {
		assert.False(t, shouldNotify(testCluster, issue))
	}
	notificationCooldown = 0
}

func TestShouldNotifySameRuleDifferentDetails(t *testing.T) {
	storage := mocks.Storage{}
	storage.On("ReadLastNotifiedRecordForClusterList",
		mock.AnythingOfType("[]types.ClusterEntry")).Return(
		func(clusterEntries []types.ClusterEntry) types.NotifiedRecordsPerCluster {
			return types.NotifiedRecordsPerCluster{
				types.ClusterOrgKey{testCluster.OrgID, testCluster.ClusterName}.ToString(): {
					OrgID:              testCluster.OrgID,
					AccountNumber:      testCluster.AccountNumber,
					ClusterName:        testCluster.ClusterName,
					UpdatedAt:          types.Timestamp(time.Now().Add(-1 * time.Hour)),
					NotificationTypeID: 1,
					StateID:            1,
					Report:             "{\"analysis_metadata\":{\"metadata\":\"some metadata\"},\"reports\":[{\"rule_id\":\"rule_4|RULE_4\",\"component\":\"ccx_rules_ocp.external.rules.rule_4.report\",\"type\":\"rule\",\"key\":\"RULE_4\",\"details\":{\"degraded_operators\":[{\"available\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:45:10Z\",\"reason\":\"AsExpected\",\"message\":\"Available: 2 nodes are active; 1 nodes are at revision 0; 2 nodes are at revision 2; 0 nodes have achieved new revision 3\"},\"degraded\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:46:14Z\",\"reason\":\"NodeInstallerDegradedInstallerPodFailed\",\"message\":\"NodeControllerDegraded: All master nodes are ready\\nStaticPodsDegraded: nodes/ip-10-0-137-172.us-east-2.compute.internal pods/kube-apiserver-ip-10-0-137-172.us-east-2.compute.internal container=\\\"kube-apiserver-3\\\" is not ready\"},\"name\":\"kube-apiserver\",\"progressing\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:43:00Z\",\"reason\":\"\",\"message\":\"Progressing: 1 nodes are at revision 0; 2 nodes are at revision 2; 0 nodes have achieved new revision 3\"},\"upgradeable\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:42:52Z\",\"reason\":\"AsExpected\",\"message\":\"\"},\"version\":\"4.3.13\"}],\"type\":\"rule\",\"error_key\":\"NODE_INSTALLER_DEGRADED\"},\"tags\":[],\"links\":{\"kcs\":[\"https://access.redhat.com/solutions/4849711\"]}}]}",
					NotifiedAt:         types.Timestamp(time.Now().Add(-1 * time.Hour)),
					ErrorLog:           "",
				},
			}
		},
		func(clusterEntries []types.ClusterEntry) error {
			return nil
		},
	)
	newReport := types.Report{
		Reports: []types.ReportItem{
			{
				Type:     "rule",
				Module:   "ccx_rules_ocp.external.rules.rule_4.report",
				ErrorKey: "RULE_4",
				Details:  []byte("The details differ somehow"),
			},
		},
	}

	previouslyReported, _ = storage.ReadLastNotifiedRecordForClusterList(make([]types.ClusterEntry, 0))
	for _, issue := range newReport.Reports {
		assert.True(t, shouldNotify(testCluster, issue))
	}
}

func TestShouldNotifyIssueNotFoundInPreviousRecords(t *testing.T) {
	storage := mocks.Storage{}
	storage.On("ReadLastNotifiedRecordForClusterList",
		mock.AnythingOfType("[]types.ClusterEntry")).Return(
		func(clusterEntries []types.ClusterEntry) types.NotifiedRecordsPerCluster {
			return types.NotifiedRecordsPerCluster{
				types.ClusterOrgKey{testCluster.OrgID, testCluster.ClusterName}.ToString(): {
					OrgID:              testCluster.OrgID,
					AccountNumber:      testCluster.AccountNumber,
					ClusterName:        testCluster.ClusterName,
					UpdatedAt:          types.Timestamp(time.Now().Add(-1 * time.Hour)),
					NotificationTypeID: 1,
					StateID:            1,
					Report:             "{\"analysis_metadata\":{\"metadata\":\"some metadata\"},\"reports\":[{\"rule_id\":\"rule_4|RULE_4\",\"component\":\"ccx_rules_ocp.external.rules.rule_4.report\",\"type\":\"rule\",\"key\":\"RULE_4\",\"details\":{\"degraded_operators\":[{\"available\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:45:10Z\",\"reason\":\"AsExpected\",\"message\":\"Available: 2 nodes are active; 1 nodes are at revision 0; 2 nodes are at revision 2; 0 nodes have achieved new revision 3\"},\"degraded\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:46:14Z\",\"reason\":\"NodeInstallerDegradedInstallerPodFailed\",\"message\":\"NodeControllerDegraded: All master nodes are ready\\nStaticPodsDegraded: nodes/ip-10-0-137-172.us-east-2.compute.internal pods/kube-apiserver-ip-10-0-137-172.us-east-2.compute.internal container=\\\"kube-apiserver-3\\\" is not ready\"},\"name\":\"kube-apiserver\",\"progressing\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:43:00Z\",\"reason\":\"\",\"message\":\"Progressing: 1 nodes are at revision 0; 2 nodes are at revision 2; 0 nodes have achieved new revision 3\"},\"upgradeable\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:42:52Z\",\"reason\":\"AsExpected\",\"message\":\"\"},\"version\":\"4.3.13\"}],\"type\":\"rule\",\"error_key\":\"NODE_INSTALLER_DEGRADED\"},\"tags\":[],\"links\":{\"kcs\":[\"https://access.redhat.com/solutions/4849711\"]}}]}",
					NotifiedAt:         types.Timestamp(time.Now().Add(-1 * time.Hour)),
					ErrorLog:           "",
				},
			}
		},
		func(clusterEntries []types.ClusterEntry) error {
			return nil
		},
	)
	newReport := types.Report{
		Reports: []types.ReportItem{
			{
				Type:     "rule",
				Module:   "ccx_rules_ocp.external.rules.new_rule.report",
				ErrorKey: "NEW_RULE_NOT_PREVIOUSLY_REPORTED",
				Details:  []byte("New rule with bunch of details"),
			},
		},
	}

	previouslyReported, _ = storage.ReadLastNotifiedRecordForClusterList(make([]types.ClusterEntry, 0))

	for _, issue := range newReport.Reports {
		assert.True(t, shouldNotify(testCluster, issue))
	}
}

func TestGetNotificationResolutionIssueNotInOldRecord(t *testing.T) {
	record := types.NotificationRecord{
		OrgID:              testCluster.OrgID,
		AccountNumber:      testCluster.AccountNumber,
		ClusterName:        testCluster.ClusterName,
		NotificationTypeID: 1,
		StateID:            1,
		Report:             "{\"analysis_metadata\":{\"metadata\":\"some metadata\"},\"reports\":[{\"rule_id\":\"rule_4|RULE_4\",\"component\":\"ccx_rules_ocp.external.rules.rule_4.report\",\"type\":\"rule\",\"key\":\"RULE_4\",\"details\":\"some details\"}]}",
		ErrorLog:           "",
	}

	issue := types.ReportItem{
		Type:     "rule",
		Module:   "ccx_rules_ocp.external.rules.new_rule.report",
		ErrorKey: "NEW_RULE_NOT_PREVIOUSLY_REPORTED",
		Details:  []byte("New rule with bunch of details"),
	}

	assert.True(t, getNotificationResolution(issue, record))
}

func TestGetNotificationResolutionIssueInOldRecordDifferentDetails(t *testing.T) {
	record := types.NotificationRecord{
		OrgID:              testCluster.OrgID,
		AccountNumber:      testCluster.AccountNumber,
		ClusterName:        testCluster.ClusterName,
		NotificationTypeID: 1,
		StateID:            1,
		Report:             "{\"analysis_metadata\":{\"metadata\":\"some metadata\"},\"reports\":[{\"rule_id\":\"rule_4|RULE_4\",\"component\":\"ccx_rules_ocp.external.rules.rule_4.report\",\"type\":\"rule\",\"key\":\"RULE_4\",\"details\":\"some details\"}]}",
		ErrorLog:           "",
	}

	issue := types.ReportItem{
		Type:     "rule",
		Module:   "ccx_rules_ocp.external.rules.rule_4.report",
		ErrorKey: "RULE_4",
		Details:  []byte("Previously reported rule with difference in details"),
	}

	assert.True(t, getNotificationResolution(issue, record))
}

func TestGetNotificationResolutionIssueInOldRecord(t *testing.T) {
	notificationCooldown = 61 * time.Minute

	record := types.NotificationRecord{
		OrgID:              testCluster.OrgID,
		AccountNumber:      testCluster.AccountNumber,
		ClusterName:        testCluster.ClusterName,
		UpdatedAt:          types.Timestamp(time.Now()),
		NotificationTypeID: 1,
		StateID:            1,
		Report:             "{\"analysis_metadata\":{\"metadata\":\"some metadata\"},\"reports\":[{\"rule_id\":\"rule_4|RULE_4\",\"component\":\"ccx_rules_ocp.external.rules.rule_4.report\",\"type\":\"rule\",\"key\":\"RULE_4\",\"details\":\"some details\"}]}",
		NotifiedAt:         types.Timestamp(time.Now()),
		ErrorLog:           "",
	}

	issue := types.ReportItem{
		Type:     "rule",
		Module:   "ccx_rules_ocp.external.rules.rule_4.report",
		ErrorKey: "RULE_4",
		Details:  []byte("\"some details\""),
	}

	assert.False(t, getNotificationResolution(issue, record))

	// Test what happens when cooldown time has passed
	record = types.NotificationRecord{
		OrgID:              testCluster.OrgID,
		AccountNumber:      testCluster.AccountNumber,
		ClusterName:        testCluster.ClusterName,
		UpdatedAt:          types.Timestamp(time.Now()),
		NotificationTypeID: 1,
		StateID:            1,
		Report:             "{\"analysis_metadata\":{\"metadata\":\"some metadata\"},\"reports\":[{\"rule_id\":\"rule_4|RULE_4\",\"component\":\"ccx_rules_ocp.external.rules.rule_4.report\",\"type\":\"rule\",\"key\":\"RULE_4\",\"details\":\"some details\"}]}",
		NotifiedAt:         types.Timestamp(time.Now().Add(-2 * time.Hour)),
		ErrorLog:           "",
	}

	assert.True(t, getNotificationResolution(issue, record))

	notificationCooldown = 0
}
