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

package differ_test

// Documentation in literate-programming-style is available at:
// https://redhatinsights.github.io/ccx-notification-writer/packages/differ/comparator_test.html

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/RedHatInsights/ccx-notification-service/differ"
	"github.com/RedHatInsights/ccx-notification-service/tests/mocks"
	"github.com/RedHatInsights/ccx-notification-service/types"
)

var (
	statesList = []types.State{
		{
			ID:      1,
			Value:   differ.NotificationStateSent,
			Comment: "sent state",
		},
		{
			ID:      2,
			Value:   differ.NotificationStateSame,
			Comment: "same state",
		},
		{
			ID:      3,
			Value:   differ.NotificationStateLower,
			Comment: "lower state",
		},
		{
			ID:      4,
			Value:   differ.NotificationStateError,
			Comment: "error state",
		},
	}

	notificationTypesListInstantOnly = []types.NotificationType{
		{
			ID:        1,
			Value:     differ.NotificationTypeInstant,
			Frequency: "instant",
			Comment:   "instant notifs",
		},
	}

	testCluster = types.ClusterEntry{
		OrgID:         1,
		AccountNumber: 0o123456,
		ClusterName:   "test_cluster1",
		KafkaOffset:   0,
		UpdatedAt:     types.Timestamp(testTimestamp),
	}
)

func TestGetState(t *testing.T) {
	assert.Equal(t, types.StateID(1), differ.GetState(statesList, differ.NotificationStateSent))
	assert.Equal(t, types.StateID(2), differ.GetState(statesList, differ.NotificationStateSame))
	assert.Equal(t, types.StateID(3), differ.GetState(statesList, differ.NotificationStateLower))
	assert.Equal(t, types.StateID(4), differ.GetState(statesList, differ.NotificationStateError))
	assert.Equal(t, types.StateID(-1), differ.GetState(statesList, "any_other_state"))
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

	assert.Nil(t, differ.GetStates(&storage))
	assert.Equal(t, types.StateID(2), differ.States.SameState)
	assert.Equal(t, types.StateID(1), differ.States.SentState)
	assert.Equal(t, types.StateID(3), differ.States.LowerIssueState)
	assert.Equal(t, types.StateID(4), differ.States.ErrorState)
}

func TestGetNotificationType(t *testing.T) {
	assert.Equal(t, types.NotificationTypeID(1), differ.GetNotificationType(notificationTypesListInstantOnly, differ.NotificationTypeInstant))
	assert.Equal(t, types.NotificationTypeID(-1), differ.GetNotificationType(notificationTypesListInstantOnly, "any_other_type"))
}

func TestGetNotifications(t *testing.T) {
	storage := mocks.Storage{}
	storage.On("ReadNotificationTypes").Return(
		func() []types.NotificationType {
			return notificationTypesListInstantOnly
		},
		func() error {
			return nil
		},
	)

	assert.Nil(t, differ.GetNotificationTypes(&storage))
	assert.Equal(t, types.NotificationTypeID(1), differ.NotificationTypes.Instant)
	// reset it to not affect other tests
	differ.NotificationTypes.Instant = 0
}

func TestIssuesEqualSameIssues(t *testing.T) {
	issue1 := types.EvaluatedReportItem{ReportItem: types.ReportItem{
		Type:     "rule",
		Module:   "ccx_rules_ocp.external.rules.rule_1.report",
		ErrorKey: "SOME_ERROR_KEY",
		Details:  []byte("details of the issue"),
	}}
	issue2 := types.EvaluatedReportItem{ReportItem: types.ReportItem{
		Type:     "rule",
		Module:   "ccx_rules_ocp.external.rules.rule_1.report",
		ErrorKey: "SOME_ERROR_KEY",
		Details:  []byte("details of the issue"),
	}}
	assert.True(t, differ.IssuesEqual(&issue1, &issue2), "Compared issues should be equal")
}

func TestIssuesEqualDifferentDetails(t *testing.T) {
	issue1 := types.EvaluatedReportItem{ReportItem: types.ReportItem{
		Type:     "rule",
		Module:   "ccx_rules_ocp.external.rules.rule_1.report",
		ErrorKey: "SOME_ERROR_KEY",
		Details:  []byte("details of the issue"),
	}}
	issue2 := types.EvaluatedReportItem{ReportItem: types.ReportItem{
		Type:     "rule",
		Module:   "ccx_rules_ocp.external.rules.rule_1.report",
		ErrorKey: "SOME_ERROR_KEY",
		Details:  []byte("details of the issue is different"),
	}}
	assert.True(t, differ.IssuesEqual(&issue1, &issue2), "Compared issues should be equal")
}

func TestIssuesEqualDifferentModule(t *testing.T) {
	issue1 := types.EvaluatedReportItem{ReportItem: types.ReportItem{
		Type:     "rule",
		Module:   "ccx_rules_ocp.external.rules.rule_1.report",
		ErrorKey: "SOME_ERROR_KEY",
		Details:  []byte("details of the issue"),
	}}
	issue2 := types.EvaluatedReportItem{ReportItem: types.ReportItem{
		Type:     "rule",
		Module:   "ccx_rules_ocp.external.rules.rule_2.report",
		ErrorKey: "SOME_ERROR_KEY",
		Details:  []byte("details of the issue"),
	}}
	assert.False(t, differ.IssuesEqual(&issue1, &issue2), "Compared issues should not be equal")
}

func TestNewIssueNotInOldReport(t *testing.T) {
	oldReport := types.Report{
		Reports: types.ReportContent{
			{
				ReportItem: types.ReportItem{
					Type:     "rule",
					Module:   "ccx_rules_ocp.external.rules.rule_1.report",
					ErrorKey: "SOME_ERROR_KEY",
					Details:  []byte("details of the issue"),
				},
			},
		},
	}

	issue := types.EvaluatedReportItem{ReportItem: types.ReportItem{
		Type:     "rule",
		Module:   "ccx_rules_ocp.external.rules.rule_2.report",
		ErrorKey: "SOME_OTHER_ERROR_KEY",
		Details:  []byte("details of the other issue"),
	}}

	assert.True(t,
		differ.IssueNotInReport(oldReport, &issue),
		"New issue not in old report old report, so result should be true.",
	)
}

func TestIssueNotInReportSameItemsInNewReport(t *testing.T) {
	oldReport := types.Report{
		Reports: types.ReportContent{
			{
				ReportItem: types.ReportItem{
					Type:     "rule",
					Module:   "ccx_rules_ocp.external.rules.rule_1.report",
					ErrorKey: "SOME_ERROR_KEY",
					Details:  []byte("details of the issue"),
				},
			},
			{
				ReportItem: types.ReportItem{
					Type:     "rule",
					Module:   "ccx_rules_ocp.external.rules.rule_2.report",
					ErrorKey: "SOME_OTHER_ERROR_KEY",
					Details:  []byte("details of the other issue"),
				},
			},
		},
	}
	newReport := types.Report{
		Reports: types.ReportContent{
			{
				ReportItem: types.ReportItem{Type: "rule",
					Module:   "ccx_rules_ocp.external.rules.rule_1.report",
					ErrorKey: "SOME_ERROR_KEY",
					Details:  []byte("details of the issue"),
				},
			},
			{
				ReportItem: types.ReportItem{Type: "rule",
					Module:   "ccx_rules_ocp.external.rules.rule_2.report",
					ErrorKey: "SOME_OTHER_ERROR_KEY",
					Details:  []byte("details of the other issue"),
				},
			},
		},
	}

	for _, issue := range newReport.Reports {
		assert.False(t,
			differ.IssueNotInReport(oldReport, issue),
			"New report has the same items than old report, so comparison result should be false.",
		)
	}
}

func TestIssueNotInReportSameLengthDifferentItemsInNewReport(t *testing.T) {
	oldReport := types.Report{
		Reports: types.ReportContent{
			{
				ReportItem: types.ReportItem{
					Type:     "rule",
					Module:   "ccx_rules_ocp.external.rules.rule_1.report",
					ErrorKey: "SOME_ERROR_KEY",
					Details:  []byte("details of the issue"),
				},
			},
			{
				ReportItem: types.ReportItem{
					Type:     "rule",
					Module:   "ccx_rules_ocp.external.rules.rule_2.report",
					ErrorKey: "SOME_OTHER_ERROR_KEY",
					Details:  []byte("details of the other issue"),
				},
			},
		},
	}
	newReport := types.Report{
		Reports: types.ReportContent{
			{
				ReportItem: types.ReportItem{
					Type:     "rule",
					Module:   "ccx_rules_ocp.external.rules.rule_3.report",
					ErrorKey: "SOME_ERROR_KEY",
					Details:  []byte("details of the issue 3"),
				},
			},
			{
				ReportItem: types.ReportItem{Type: "rule",
					Module:   "ccx_rules_ocp.external.rules.rule_4.report",
					ErrorKey: "SOME_OTHER_ERROR_KEY",
					Details:  []byte("details of the other issue 4"),
				},
			},
		},
	}

	for _, issue := range newReport.Reports {
		assert.True(t,
			differ.IssueNotInReport(oldReport, issue),
			"New and old report have different items, so result should be true.",
		)
	}
}

func TestIssueNotInReportLessItemsInNewReportAndIssueNotFoundInOldReports(t *testing.T) {
	oldReport := types.Report{
		Reports: types.ReportContent{
			{
				ReportItem: types.ReportItem{Type: "rule",
					Module:   "ccx_rules_ocp.external.rules.rule_1.report",
					ErrorKey: "SOME_ERROR_KEY",
					Details:  []byte("details of the issue"),
				},
			},
			{
				ReportItem: types.ReportItem{Type: "rule",
					Module:   "ccx_rules_ocp.external.rules.rule_2.report",
					ErrorKey: "SOME_OTHER_ERROR_KEY",
					Details:  []byte("details of the other issue"),
				},
			},
		},
	}
	newReport := types.Report{
		Reports: types.ReportContent{
			{
				ReportItem: types.ReportItem{
					Type:     "rule",
					Module:   "ccx_rules_ocp.external.rules.rule_3.report",
					ErrorKey: "SOME_ERROR_KEY",
					Details:  []byte("details of the new issue"),
				},
			},
		},
	}

	for _, issue := range newReport.Reports {
		assert.True(t,
			differ.IssueNotInReport(oldReport, issue),
			"New report's issue has not been found in old report, so result should be true.",
		)
	}
}

func TestIssueNotInReportLessItemsInNewReportAndIssueFoundInOldReports(t *testing.T) {
	oldReport := types.Report{
		Reports: types.ReportContent{
			{
				ReportItem: types.ReportItem{Type: "rule",
					Module:   "ccx_rules_ocp.external.rules.rule_1.report",
					ErrorKey: "SOME_ERROR_KEY",
					Details:  []byte("details of the issue"),
				},
			},
			{
				ReportItem: types.ReportItem{
					Type:     "rule",
					Module:   "ccx_rules_ocp.external.rules.rule_2.report",
					ErrorKey: "SOME_OTHER_ERROR_KEY",
					Details:  []byte("details of the other issue"),
				},
			},
		},
	}
	newReport := types.Report{
		Reports: types.ReportContent{
			{
				ReportItem: types.ReportItem{Type: "rule",
					Module:   "ccx_rules_ocp.external.rules.rule_1.report",
					ErrorKey: "SOME_ERROR_KEY",
					Details:  []byte("details of the issue"),
				},
			},
		},
	}

	for _, issue := range newReport.Reports {
		assert.False(t,
			differ.IssueNotInReport(oldReport, issue),
			"New report's issue has been found in old report, so result should be false.",
		)
	}
}

func TestShouldNotifyNoPreviousRecord(t *testing.T) {
	storage := mocks.Storage{}
	storage.On("ReadLastNotifiedRecordForClusterList",
		mock.AnythingOfType("[]types.ClusterEntry"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("types.EventTarget")).
		Return(
			func(clusterEntries []types.ClusterEntry, timeOffset string, eventTarget types.EventTarget) types.NotifiedRecordsPerCluster {
				return types.NotifiedRecordsPerCluster{}
			},
			func(clusterEntries []types.ClusterEntry, timeOffset string, eventTarget types.EventTarget) error {
				return nil
			},
		)
	newReport := types.Report{
		Reports: types.ReportContent{
			{
				ReportItem: types.ReportItem{
					Type:     "rule",
					Module:   "ccx_rules_ocp.external.rules.rule_1.report",
					ErrorKey: "SOME_ERROR_KEY",
					Details:  []byte("details of the issue"),
				},
			},
		},
	}

	d := differ.Differ{
		Target: types.NotificationBackendTarget,
	}

	d.PreviouslyReported, _ = storage.ReadLastNotifiedRecordForClusterList(make([]types.ClusterEntry, 0), "24 hours", types.NotificationBackendTarget)
	for _, issue := range newReport.Reports {
		assert.True(t, d.ShouldNotify(testCluster, issue))
	}
}

func TestShouldNotNotifySameRuleDifferentDetails(t *testing.T) {
	storage := mocks.Storage{}
	storage.On("ReadLastNotifiedRecordForClusterList",
		mock.AnythingOfType("[]types.ClusterEntry"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("types.EventTarget")).
		Return(
			func(clusterEntries []types.ClusterEntry, timeOffset string, eventTarget types.EventTarget) types.NotifiedRecordsPerCluster {
				return types.NotifiedRecordsPerCluster{
					types.ClusterOrgKey{OrgID: testCluster.OrgID, ClusterName: testCluster.ClusterName}: {
						OrgID:              testCluster.OrgID,
						AccountNumber:      testCluster.AccountNumber,
						ClusterName:        testCluster.ClusterName,
						UpdatedAt:          types.Timestamp(testTimestamp),
						NotificationTypeID: 1,
						StateID:            1,
						Report:             "{\"analysis_metadata\":{\"metadata\":\"some metadata\"},\"reports\":[{\"rule_id\":\"rule_4|RULE_4\",\"component\":\"ccx_rules_ocp.external.rules.rule_4.report\",\"type\":\"rule\",\"key\":\"RULE_4\",\"details\":{\"degraded_operators\":[{\"available\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:45:10Z\",\"reason\":\"AsExpected\",\"message\":\"Available: 2 nodes are active; 1 nodes are at revision 0; 2 nodes are at revision 2; 0 nodes have achieved new revision 3\"},\"degraded\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:46:14Z\",\"reason\":\"NodeInstallerDegradedInstallerPodFailed\",\"message\":\"NodeControllerDegraded: All master nodes are ready\\nStaticPodsDegraded: nodes/ip-10-0-137-172.us-east-2.compute.internal pods/kube-apiserver-ip-10-0-137-172.us-east-2.compute.internal container=\\\"kube-apiserver-3\\\" is not ready\"},\"name\":\"kube-apiserver\",\"progressing\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:43:00Z\",\"reason\":\"\",\"message\":\"Progressing: 1 nodes are at revision 0; 2 nodes are at revision 2; 0 nodes have achieved new revision 3\"},\"upgradeable\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:42:52Z\",\"reason\":\"AsExpected\",\"message\":\"\"},\"version\":\"4.3.13\"}],\"type\":\"rule\",\"error_key\":\"NODE_INSTALLER_DEGRADED\"},\"tags\":[],\"links\":{\"kcs\":[\"https://access.redhat.com/solutions/4849711\"]}}]}",
						NotifiedAt:         types.Timestamp(testTimestamp),
						ErrorLog:           "",
					},
				}
			},
			func(clusterEntries []types.ClusterEntry, timeOffset string, eventTarget types.EventTarget) error {
				return nil
			},
		)
	newReport := types.Report{
		Reports: types.ReportContent{
			{
				ReportItem: types.ReportItem{Type: "rule",
					Module:   "ccx_rules_ocp.external.rules.rule_4.report",
					ErrorKey: "RULE_4",
					Details:  []byte("The details differ somehow"),
				},
			},
		},
	}

	d := differ.Differ{
		Target: types.NotificationBackendTarget,
	}

	d.PreviouslyReported, _ = storage.ReadLastNotifiedRecordForClusterList(make([]types.ClusterEntry, 0), "24 hours", types.NotificationBackendTarget)

	for _, issue := range newReport.Reports {
		assert.False(t, d.ShouldNotify(testCluster, issue))
	}
}

func TestShouldNotifyIssueNotFoundInPreviousRecords(t *testing.T) {
	storage := mocks.Storage{}
	storage.On("ReadLastNotifiedRecordForClusterList",
		mock.AnythingOfType("[]types.ClusterEntry"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("types.EventTarget")).
		Return(
			func(clusterEntries []types.ClusterEntry, timeOffset string, eventTarget types.EventTarget) types.NotifiedRecordsPerCluster {
				return types.NotifiedRecordsPerCluster{
					types.ClusterOrgKey{OrgID: testCluster.OrgID, ClusterName: testCluster.ClusterName}: {
						OrgID:              testCluster.OrgID,
						AccountNumber:      testCluster.AccountNumber,
						ClusterName:        testCluster.ClusterName,
						UpdatedAt:          types.Timestamp(testTimestamp),
						NotificationTypeID: 1,
						StateID:            1,
						Report:             "{\"analysis_metadata\":{\"metadata\":\"some metadata\"},\"reports\":[{\"rule_id\":\"rule_4|RULE_4\",\"component\":\"ccx_rules_ocp.external.rules.rule_4.report\",\"type\":\"rule\",\"key\":\"RULE_4\",\"details\":{\"degraded_operators\":[{\"available\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:45:10Z\",\"reason\":\"AsExpected\",\"message\":\"Available: 2 nodes are active; 1 nodes are at revision 0; 2 nodes are at revision 2; 0 nodes have achieved new revision 3\"},\"degraded\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:46:14Z\",\"reason\":\"NodeInstallerDegradedInstallerPodFailed\",\"message\":\"NodeControllerDegraded: All master nodes are ready\\nStaticPodsDegraded: nodes/ip-10-0-137-172.us-east-2.compute.internal pods/kube-apiserver-ip-10-0-137-172.us-east-2.compute.internal container=\\\"kube-apiserver-3\\\" is not ready\"},\"name\":\"kube-apiserver\",\"progressing\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:43:00Z\",\"reason\":\"\",\"message\":\"Progressing: 1 nodes are at revision 0; 2 nodes are at revision 2; 0 nodes have achieved new revision 3\"},\"upgradeable\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:42:52Z\",\"reason\":\"AsExpected\",\"message\":\"\"},\"version\":\"4.3.13\"}],\"type\":\"rule\",\"error_key\":\"NODE_INSTALLER_DEGRADED\"},\"tags\":[],\"links\":{\"kcs\":[\"https://access.redhat.com/solutions/4849711\"]}}]}",
						NotifiedAt:         types.Timestamp(testTimestamp),
						ErrorLog:           "",
					},
				}
			},
			func(clusterEntries []types.ClusterEntry, timeOffset string, eventTarget types.EventTarget) error {
				return nil
			},
		)
	newReport := types.Report{
		Reports: types.ReportContent{
			{
				ReportItem: types.ReportItem{
					Type:     "rule",
					Module:   "ccx_rules_ocp.external.rules.new_rule.report",
					ErrorKey: "NEW_RULE_NOT_PREVIOUSLY_REPORTED",
					Details:  []byte("New rule with bunch of details"),
				},
			},
		},
	}

	d := differ.Differ{
		Target: types.NotificationBackendTarget,
	}

	d.PreviouslyReported, _ = storage.ReadLastNotifiedRecordForClusterList(make([]types.ClusterEntry, 0), "24 hours", types.NotificationBackendTarget)

	for _, issue := range newReport.Reports {
		assert.True(t, d.ShouldNotify(testCluster, issue))
	}
}
