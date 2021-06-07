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

package differ

import (
	"bytes"
	"github.com/RedHatInsights/ccx-notification-service/tests/mocks"
	"github.com/RedHatInsights/ccx-notification-service/types"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

var (

	storage = mocks.Storage{}

	statesList = []types.State{
		{
			ID:      1,
			Value:   "sent",
			Comment: "sent state",
		},
		{
			ID:      2,
			Value:   "same",
			Comment: "same state",
		},
		{
			ID:      3,
			Value:   "lower",
			Comment: "lower state",
		},
		{
			ID:      4,
			Value:   "error",
			Comment: "error state",
		},
	}

	notificationTypesList = []types.NotificationType{
		{
			ID:        1,
			Value:     "instant",
			Frequency: "instant",
			Comment:   "instant notifs",
		},
		{
			ID:        2,
			Value:     "weekly",
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
}

func TestGetState(t *testing.T) {
	assert.Equal(t, types.StateID(1), getState(statesList, "sent"))
	assert.Equal(t, types.StateID(2), getState(statesList, "same"))
	assert.Equal(t, types.StateID(3), getState(statesList, "lower"))
	assert.Equal(t, types.StateID(4), getState(statesList, "error"))
	assert.Equal(t, -1, getNotificationType(notificationTypesList, "any_other_state"))
}

func TestGetStates(t *testing.T) {
	storage.On("ReadStates").Return(
		func() []types.State {
			return statesList
		},
		func() error {
			return nil
		},
	)

	getStates(&storage)
	assert.Equal(t, types.StateID(1), states.SentState)
	assert.Equal(t, types.StateID(2), states.SameState)
	assert.Equal(t, types.StateID(3), states.LowerIssueState)
	assert.Equal(t, types.StateID(4), states.ErrorState)
}

func TestGetNotificationType(t *testing.T) {
	assert.Equal(t, types.NotificationTypeID(1), getNotificationType(notificationTypesList, "instant"))
	assert.Equal(t, types.NotificationTypeID(2), getNotificationType(notificationTypesList, "weekly"))
	assert.Equal(t, types.NotificationTypeID(-1), getNotificationType(notificationTypesList, "any_other_type"))
}

func TestGetNotifications(t *testing.T) {
	storage.On("ReadNotificationTypes").Return(
		func() []types.NotificationType {
			return notificationTypesList
		},
		func() error {
			return nil
		},
	)

	getNotificationTypes(&storage)
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

func TestCompareReportsMoreItemsInNewReport(t *testing.T) {
	oldReport := types.Report {
		Reports: []types.ReportItem{
			{
				Type:     "rule",
				Module:   "ccx_rules_ocp.external.rules.rule_1.report",
				ErrorKey: "SOME_ERROR_KEY",
				Details:  []byte("details of the issue"),
			},
		},
	}
	newReport := types.Report {
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

	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	result, err := CompareReports(oldReport, newReport)
	assert.True(t, result, "New report ha more items than old report, so comparison result should be true.")
	assert.Nil(t, err)
	assert.Contains(t, buf.String(), "New report contains more issues")
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
}

func TestCompareReportsSameItemsInNewReport(t *testing.T) {
	oldReport := types.Report {
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
	newReport := types.Report {
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

	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	result, err := CompareReports(oldReport, newReport)
	assert.False(t, result, "New report has the same items than old report, so comparison result should be false.")
	assert.Nil(t, err)
	assert.Contains(t, buf.String(), "New report does not contain new issues")
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
}

func TestCompareReportsSameLengthDifferentItemsInNewReport(t *testing.T) {
	oldReport := types.Report {
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
	newReport := types.Report {
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

	result, err := CompareReports(oldReport, newReport)
	assert.True(t, result, "New and olf report have different items, so comparison result should be true.")
	assert.Nil(t, err)
}

func TestCompareReportsLessItemsInNewReportAndIssueNotFoundInOldReports(t *testing.T) {
	oldReport := types.Report {
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
	newReport := types.Report {
		Reports: []types.ReportItem{
			{
				Type:     "rule",
				Module:   "ccx_rules_ocp.external.rules.rule_3.report",
				ErrorKey: "SOME_ERROR_KEY",
				Details:  []byte("details of the new issue"),
			},
		},
	}

	result, err := CompareReports(oldReport, newReport)
	assert.True(t, result, "New report's issue has not been found in old report, so comparison result should be true.")
	assert.Nil(t, err)
}

func TestCompareReportsLessItemsInNewReportAndIssueFoundInOldReports(t *testing.T) {
	oldReport := types.Report {
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
	newReport := types.Report {
		Reports: []types.ReportItem{
			{
				Type:     "rule",
				Module:   "ccx_rules_ocp.external.rules.rule_1.report",
				ErrorKey: "SOME_ERROR_KEY",
				Details:  []byte("details of the issue"),
			},
		},
	}

	result, err := CompareReports(oldReport, newReport)
	assert.False(t, result, "New report's issue has been found in old report, so comparison result should be false.")
	assert.Nil(t, err)
}

func TestShouldNotifyNoPreviousRecord(t *testing.T) {
	storage.On("ReadLastNNotificationRecords",
		mock.AnythingOfType("types.ClusterEntry"),
		mock.AnythingOfType("int")).Return(
		func(clusterEntry types.ClusterEntry, numberOfRecords int) []types.NotificationRecord {
			return make([]types.NotificationRecord, 0, 0)
		},
		func(clusterEntry types.ClusterEntry, numberOfRecords int) error {
			return nil
		},
	)
	newReport := types.Report {
		Reports: []types.ReportItem{
			{
				Type:     "rule",
				Module:   "ccx_rules_ocp.external.rules.rule_1.report",
				ErrorKey: "SOME_ERROR_KEY",
				Details:  []byte("details of the issue"),
			},
		},
	}
	assert.True(t, shouldNotify(&storage, testCluster, newReport))
}

func TestShouldNotifyPreviousRecordForGivenClusterIsIdentical(t *testing.T) {
	storage.On("ReadLastNNotificationRecords",
		mock.AnythingOfType("types.ClusterEntry"),
		mock.AnythingOfType("int")).Return(
		func(clusterEntry types.ClusterEntry, numberOfRecords int) []types.NotificationRecord {
			return []types.NotificationRecord{
				{
					OrgID:              testCluster.OrgID,
					AccountNumber:      testCluster.AccountNumber,
					ClusterName:        testCluster.ClusterName,
					UpdatedAt:          types.Timestamp(testTimestamp.Add(-2)),
					NotificationTypeID: 1,
					StateID:            1,
					Report:             "{\"analysis_metadata\":{\"metadata\":\"some metadata\"},\"reports\":[{\"rule_id\":\"rule_4|RULE_4\",\"component\":\"ccx_rules_ocp.external.rules.rule_4.report\",\"type\":\"rule\",\"key\":\"RULE_4\",\"details\":{\"degraded_operators\":[{\"available\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:45:10Z\",\"reason\":\"AsExpected\",\"message\":\"Available: 2 nodes are active; 1 nodes are at revision 0; 2 nodes are at revision 2; 0 nodes have achieved new revision 3\"},\"degraded\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:46:14Z\",\"reason\":\"NodeInstallerDegradedInstallerPodFailed\",\"message\":\"NodeControllerDegraded: All master nodes are ready\\nStaticPodsDegraded: nodes/ip-10-0-137-172.us-east-2.compute.internal pods/kube-apiserver-ip-10-0-137-172.us-east-2.compute.internal container=\\\"kube-apiserver-3\\\" is not ready\"},\"name\":\"kube-apiserver\",\"progressing\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:43:00Z\",\"reason\":\"\",\"message\":\"Progressing: 1 nodes are at revision 0; 2 nodes are at revision 2; 0 nodes have achieved new revision 3\"},\"upgradeable\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:42:52Z\",\"reason\":\"AsExpected\",\"message\":\"\"},\"version\":\"4.3.13\"}],\"type\":\"rule\",\"error_key\":\"NODE_INSTALLER_DEGRADED\"},\"tags\":[],\"links\":{\"kcs\":[\"https://access.redhat.com/solutions/4849711\"]}}]}",
					NotifiedAt:         types.Timestamp(testTimestamp.Add(-2)),
					ErrorLog:           "",
				},
			}
		},
		func(clusterEntry types.ClusterEntry, numberOfRecords int) error {
			return nil
		},
	)
	newReport := types.Report {
		Reports: []types.ReportItem{
			{
				Type:     "rule",
				Module:   "ccx_rules_ocp.external.rules.rule_4.report",
				ErrorKey: "RULE_4",
				Details:  []byte("{\"degraded_operators\":[{\"available\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:45:10Z\",\"reason\":\"AsExpected\",\"message\":\"Available: 2 nodes are active; 1 nodes are at revision 0; 2 nodes are at revision 2; 0 nodes have achieved new revision 3\"},\"degraded\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:46:14Z\",\"reason\":\"NodeInstallerDegradedInstallerPodFailed\",\"message\":\"NodeControllerDegraded: All master nodes are ready\\nStaticPodsDegraded: nodes/ip-10-0-137-172.us-east-2.compute.internal pods/kube-apiserver-ip-10-0-137-172.us-east-2.compute.internal container=\\\"kube-apiserver-3\\\" is not ready\"},\"name\":\"kube-apiserver\",\"progressing\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:43:00Z\",\"reason\":\"\",\"message\":\"Progressing: 1 nodes are at revision 0; 2 nodes are at revision 2; 0 nodes have achieved new revision 3\"},\"upgradeable\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:42:52Z\",\"reason\":\"AsExpected\",\"message\":\"\"},\"version\":\"4.3.13\"}],\"type\":\"rule\",\"error_key\":\"NODE_INSTALLER_DEGRADED\"}"),
			},
		},
	}
	assert.False(t, shouldNotify(&storage, testCluster, newReport))
}

func TestShouldNotifySameRuleDifferentDetails(t *testing.T) {
	storage.On("ReadLastNNotificationRecords",
		mock.AnythingOfType("types.ClusterEntry"),
		mock.AnythingOfType("int")).Return(
		func(clusterEntry types.ClusterEntry, numberOfRecords int) []types.NotificationRecord {
			return []types.NotificationRecord{
				{
					OrgID:              testCluster.OrgID,
					AccountNumber:      testCluster.AccountNumber,
					ClusterName:        testCluster.ClusterName,
					UpdatedAt:          types.Timestamp(testTimestamp.Add(-2)),
					NotificationTypeID: 1,
					StateID:            1,
					Report:             "{\"analysis_metadata\":{\"metadata\":\"some metadata\"},\"reports\":[{\"rule_id\":\"rule_4|RULE_4\",\"component\":\"ccx_rules_ocp.external.rules.rule_4.report\",\"type\":\"rule\",\"key\":\"RULE_4\",\"details\":{\"degraded_operators\":[{\"available\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:45:10Z\",\"reason\":\"AsExpected\",\"message\":\"Available: 2 nodes are active; 1 nodes are at revision 0; 2 nodes are at revision 2; 0 nodes have achieved new revision 3\"},\"degraded\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:46:14Z\",\"reason\":\"NodeInstallerDegradedInstallerPodFailed\",\"message\":\"NodeControllerDegraded: All master nodes are ready\\nStaticPodsDegraded: nodes/ip-10-0-137-172.us-east-2.compute.internal pods/kube-apiserver-ip-10-0-137-172.us-east-2.compute.internal container=\\\"kube-apiserver-3\\\" is not ready\"},\"name\":\"kube-apiserver\",\"progressing\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:43:00Z\",\"reason\":\"\",\"message\":\"Progressing: 1 nodes are at revision 0; 2 nodes are at revision 2; 0 nodes have achieved new revision 3\"},\"upgradeable\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:42:52Z\",\"reason\":\"AsExpected\",\"message\":\"\"},\"version\":\"4.3.13\"}],\"type\":\"rule\",\"error_key\":\"NODE_INSTALLER_DEGRADED\"},\"tags\":[],\"links\":{\"kcs\":[\"https://access.redhat.com/solutions/4849711\"]}}]}",
					NotifiedAt:         types.Timestamp(testTimestamp.Add(-2)),
					ErrorLog:           "",
				},
			}
		},
		func(clusterEntry types.ClusterEntry, numberOfRecords int) error {
			return nil
		},
	)
	newReport := types.Report {
		Reports: []types.ReportItem{
			{
				Type:     "rule",
				Module:   "ccx_rules_ocp.external.rules.rule_4.report",
				ErrorKey: "RULE_4",
				Details:  []byte("The details differ somehow"),
			},
		},
	}
	assert.True(t, shouldNotify(&storage, testCluster, newReport))
}

func TestShouldNotifyIssueNotFoundInPreviousRecords(t *testing.T) {
	storage.On("ReadLastNNotificationRecords",
		mock.AnythingOfType("types.ClusterEntry"),
		mock.AnythingOfType("int")).Return(
		func(clusterEntry types.ClusterEntry, numberOfRecords int) []types.NotificationRecord {
			return []types.NotificationRecord{
				{
					OrgID:              testCluster.OrgID,
					AccountNumber:      testCluster.AccountNumber,
					ClusterName:        testCluster.ClusterName,
					UpdatedAt:          types.Timestamp(testTimestamp.Add(-2)),
					NotificationTypeID: 1,
					StateID:            1,
					Report:             "{\"analysis_metadata\":{\"metadata\":\"some metadata\"},\"reports\":[{\"rule_id\":\"rule_4|RULE_4\",\"component\":\"ccx_rules_ocp.external.rules.rule_4.report\",\"type\":\"rule\",\"key\":\"RULE_4\",\"details\":{\"degraded_operators\":[{\"available\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:45:10Z\",\"reason\":\"AsExpected\",\"message\":\"Available: 2 nodes are active; 1 nodes are at revision 0; 2 nodes are at revision 2; 0 nodes have achieved new revision 3\"},\"degraded\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:46:14Z\",\"reason\":\"NodeInstallerDegradedInstallerPodFailed\",\"message\":\"NodeControllerDegraded: All master nodes are ready\\nStaticPodsDegraded: nodes/ip-10-0-137-172.us-east-2.compute.internal pods/kube-apiserver-ip-10-0-137-172.us-east-2.compute.internal container=\\\"kube-apiserver-3\\\" is not ready\"},\"name\":\"kube-apiserver\",\"progressing\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:43:00Z\",\"reason\":\"\",\"message\":\"Progressing: 1 nodes are at revision 0; 2 nodes are at revision 2; 0 nodes have achieved new revision 3\"},\"upgradeable\":{\"status\":true,\"last_trans_time\":\"2020-04-21T12:42:52Z\",\"reason\":\"AsExpected\",\"message\":\"\"},\"version\":\"4.3.13\"}],\"type\":\"rule\",\"error_key\":\"NODE_INSTALLER_DEGRADED\"},\"tags\":[],\"links\":{\"kcs\":[\"https://access.redhat.com/solutions/4849711\"]}}]}",
					NotifiedAt:         types.Timestamp(testTimestamp.Add(-2)),
					ErrorLog:           "",
				},
			}
		},
		func(clusterEntry types.ClusterEntry, numberOfRecords int) error {
			return nil
		},
	)
	newReport := types.Report {
		Reports: []types.ReportItem{
			{
				Type:     "rule",
				Module:   "ccx_rules_ocp.external.rules.new_rule.report",
				ErrorKey: "NEW_RULE_NOT_PREVIOUSLY_REPORTED",
				Details:  []byte("New rule with bunch of details"),
			},
		},
	}
	assert.True(t, shouldNotify(&storage, testCluster, newReport))
}