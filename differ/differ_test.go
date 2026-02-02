/*
Copyright © 2021, 2022, 2023 Red Hat, Inc.

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
// https://redhatinsights.github.io/ccx-notification-writer/packages/differ/differ_test.html

import (
	"bytes"
	"database/sql"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/RedHatInsights/ccx-notification-service/tests/mocks"
	utypes "github.com/RedHatInsights/insights-results-types"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/mock"

	"github.com/IBM/sarama"
	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/differ"
	"github.com/RedHatInsights/ccx-notification-service/producer/kafka"
	"github.com/RedHatInsights/ccx-notification-service/types"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

var (
	brokerCfg = conf.KafkaConfiguration{
		Addresses:   "localhost:9092",
		Topic:       "platform.notifications.ingress",
		Timeout:     time.Duration(30*10 ^ 9),
		Enabled:     true,
		EventFilter: "totalRisk >= totalRiskThreshold",
	}
	// Base UNIX time plus approximately 50 years (not long before year 2020).
	testTimestamp   = time.Unix(50*365*24*60*60, 0)
	testPartitionID = 0
	testOffset      = 0
)

func init() {
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
}

// ---------------------------------------------------------------------------------------
func TestSetupNotificationTypesReadOk(t *testing.T) {
	assert.Equal(t, &types.NotificationTypes{Instant: 0, Weekly: 0}, differ.NotificationTypes)

	// if there is something in the DB - it should set properly
	stor := &mocks.Storage{}
	stor.On("ReadNotificationTypes").Return(
		func() []types.NotificationType {
			return notificationTypesListInstantOnly
		},
		func() error {
			return nil
		},
	)
	err := differ.SetupNotificationTypes(stor)
	assert.Equal(t, &types.NotificationTypes{Instant: 1, Weekly: 0}, differ.NotificationTypes)
	assert.Nil(t, err)
	// reset it to not affect other tests
	differ.NotificationTypes.Instant = 0
}

func TestSetupNotificationTypesError(t *testing.T) {
	stor := &mocks.Storage{}
	stor.On("ReadNotificationTypes").Return(
		func() []types.NotificationType {
			n := make([]types.NotificationType, 0)
			return n
		},
		func() error {
			return sql.ErrNoRows
		},
	)
	err := differ.SetupNotificationTypes(stor)
	assert.ErrorIs(t, err, &differ.StatusStorageError{})
}

// ---------------------------------------------------------------------------------------
func TestGenerateInstantReportNotificationMessage(t *testing.T) {
	clusterURI := "the_cluster_uri_in_ocm_for_{cluster_id}"
	accountID := "a_stringified_account_id"
	orgID := "a_stringified_org_id"
	clusterID := "the_displayed_cluster_ID"

	notificationMsg := differ.GenerateInstantNotificationMessage(&clusterURI, accountID, orgID, clusterID)

	assert.NotEmpty(t, notificationMsg, "the generated notification message is empty")
	assert.Empty(t, notificationMsg.Events, "the generated notification message should not have any events")
	assert.Equal(t, types.InstantNotif.ToString(), notificationMsg.EventType, "the generated notification message should be for instant notifications")
	assert.Equal(t, types.NotificationContext{"display_name": "the_displayed_cluster_ID", "host_url": "the_cluster_uri_in_ocm_for_the_displayed_cluster_ID"}, notificationMsg.Context, "Notification context is different from expected.")
	assert.Equal(t, differ.NotificationBundleName, notificationMsg.Bundle, "Generated notifications should indicate 'openshift' as bundle")
	assert.Equal(t, differ.NotificationApplicationName, notificationMsg.Application, "Generated notifications should indicate 'openshift' as application name")
	assert.Equal(t, accountID, notificationMsg.AccountID, "Generated notifications does not have expected account ID")
	assert.Equal(t, orgID, notificationMsg.OrgID, "Generated notifications does not have expected org ID")
}

func TestAppendEventsToExistingInstantReportNotificationMsg(t *testing.T) {
	clusterURI := "the_cluster_uri_in_ocm"
	accountID := "a_stringified_account_id"
	orgID := "a_stringified_org_id"
	clusterID := "the_displayed_cluster_ID"
	notificationMsg := differ.GenerateInstantNotificationMessage(&clusterURI, accountID, orgID, clusterID)

	assert.Empty(t, notificationMsg.Events, "the generated notification message should not have any events")

	ruleURI := "a_given_uri/{cluster_id}/{module}/{error_key}"
	ruleDescription := "a_given_rule_name"
	publishDate := "the_date_of_today"
	totalRisk := differ.TotalRiskLow
	module := "a.module"
	errorKey := "an_error_key"

	notificationPayloadURL := differ.GenerateNotificationPayloadURL(&ruleURI, clusterID, types.ModuleName(module), types.ErrorKey(errorKey))
	differ.AppendEventToNotificationMessage(notificationPayloadURL, &notificationMsg, ruleDescription, totalRisk, publishDate)
	assert.Equal(t, len(notificationMsg.Events), 1, "the notification message should have 1 event")
	assert.Equal(t, notificationMsg.Events[0].Metadata, types.EventMetadata{}, "All notification messages should have empty metadata")

	payload := notificationMsg.Events[0].Payload
	assert.Equal(t, payload[differ.NotificationPayloadRuleURL], "a_given_uri/the_displayed_cluster_ID/a|module/an_error_key", fmt.Sprintf("The rule URL %s is not correct", payload[differ.NotificationPayloadRuleURL]))

	differ.AppendEventToNotificationMessage(notificationPayloadURL, &notificationMsg, ruleDescription, totalRisk, publishDate)
	assert.Equal(t, len(notificationMsg.Events), 2, "the notification message should have 2 events")
	assert.Equal(t, notificationMsg.Events[1].Metadata, types.EventMetadata{}, "All notification messages should have empty metadata")
}

// ---------------------------------------------------------------------------------------
func TestTotalRiskCalculation(t *testing.T) {
	type testStruct struct {
		impact       int
		likelihood   int
		expectedRisk int
	}
	testVals := []testStruct{
		{0, 0, 0},
		{3, 1, 2},
		{1, 0, 0},
		{0, 3, 1},
		{2, 2, 2},
		{3, 1, 2},
		{2, 3, 2},
		{3, 3, 3},
		{4, 3, 3},
		{3, 4, 3},
	}
	for _, item := range testVals {
		assert.Equal(t, item.expectedRisk, differ.CalculateTotalRisk(item.impact, item.likelihood))
	}
}

func TestModuleNameToRuleNameValidRuleName(t *testing.T) {
	moduleName := types.ModuleName("ccx_rules_ocp.external.rules.cluster_wide_proxy_auth_check.report")
	ruleName := types.RuleName("cluster_wide_proxy_auth_check")
	assert.Equal(t, ruleName, differ.ModuleToRuleName(moduleName))
}

// ---------------------------------------------------------------------------------------
func TestSetupNotificationProducerInvalidBrokerConf(t *testing.T) {
	testConfig := conf.ConfigStruct{
		Kafka: conf.KafkaConfiguration{
			Addresses:   "invalid_address",
			Topic:       "",
			Timeout:     0,
			Enabled:     true,
			EventFilter: "totalRisk >= totalRiskThreshold",
		},
	}

	d := differ.Differ{}
	err := d.SetupKafkaProducer(&testConfig)
	assert.ErrorIs(t, err, &differ.KafkaBrokerError{})
}

func TestAssertNotificationDestinationNone(t *testing.T) {
	testConfig := conf.ConfigStruct{}
	err := differ.AssertNotificationDestination(&testConfig)
	assert.ErrorIs(t, err, &differ.StatusConfiguration{})
}

func TestAssertNotificationDestinationMoreThanOne(t *testing.T) {
	testConfig := conf.ConfigStruct{
		Kafka: conf.KafkaConfiguration{
			Enabled: true,
		},
		ServiceLog: conf.ServiceLogConfiguration{
			Enabled: true,
		},
	}
	err := differ.AssertNotificationDestination(&testConfig)
	assert.ErrorIs(t, err, &differ.StatusConfiguration{})
}

func TestSetupNotificationProducerValidBrokerConf(t *testing.T) {
	defaultVersion := kafka.SaramaVersion
	kafka.SaramaVersion = sarama.V0_10_2_0
	defer func() {
		kafka.SaramaVersion = defaultVersion
	}()

	mockBroker := sarama.NewMockBroker(t, 0)
	defer mockBroker.Close()

	mockBroker.SetHandlerByMap(
		map[string]sarama.MockResponse{
			"MetadataRequest": sarama.NewMockMetadataResponse(t).
				SetBroker(mockBroker.Addr(), mockBroker.BrokerID()).
				SetLeader(brokerCfg.Topic, 0, mockBroker.BrokerID()),
			"OffsetRequest": sarama.NewMockOffsetResponse(t).
				SetOffset(brokerCfg.Topic, 0, -1, 0).
				SetOffset(brokerCfg.Topic, 0, -2, 0),
			"FetchRequest": sarama.NewMockFetchResponse(t, 1),
			"FindCoordinatorRequest": sarama.NewMockFindCoordinatorResponse(t).
				SetCoordinator(sarama.CoordinatorGroup, "", mockBroker),
			"OffsetFetchRequest": sarama.NewMockOffsetFetchResponse(t).
				SetOffset("", brokerCfg.Topic, 0, 0, "", sarama.ErrNoError),
		})

	testConfig := conf.ConfigStruct{
		Kafka: conf.KafkaConfiguration{
			Addresses: mockBroker.Addr(),
			Topic:     brokerCfg.Topic,
			Timeout:   brokerCfg.Timeout,
			Enabled:   brokerCfg.Enabled,
		},
	}

	kafkaProducer := kafka.Producer{
		Configuration: conf.GetKafkaBrokerConfiguration(&testConfig),
		Producer:      nil,
	}

	d := differ.Differ{}

	err := d.SetupKafkaProducer(&testConfig)
	assert.Nil(t, err)

	producer := d.Notifier.(*kafka.Producer)

	assert.Equal(t, kafkaProducer.Configuration.Addresses, producer.Configuration.Addresses)
	assert.Equal(t, kafkaProducer.Configuration.Topic, producer.Configuration.Topic)
	assert.Equal(t, kafkaProducer.Configuration.Timeout, producer.Configuration.Timeout)
	assert.Nil(t, kafkaProducer.Producer, "Unexpected behavior: Producer was not set up correctly")
	assert.NotNil(t, producer.Producer, "Unexpected behavior: Producer was not set up correctly")

	err = d.Notifier.Close()
	assert.Nil(t, err, "Unexpected behavior: Producer was not closed successfully")
}

func TestNewDifferNoDestination(t *testing.T) {
	testConfig := conf.ConfigStruct{}
	_, err := differ.New(&testConfig, nil)
	assert.ErrorIs(t, err, &differ.StatusConfiguration{})
}

// ---------------------------------------------------------------------------------------
// TestProcessClustersNoReportForClusterEntry tests that when no report is found for
// a given cluster entry, the processing is not stopped
func TestProcessClustersNoReportForClusterEntry(t *testing.T) {
	storage := mocks.Storage{}
	storage.On("ReadReportForClusterAtTime",
		mock.MatchedBy(func(orgID types.OrgID) bool { return orgID == 1 }),
		mock.AnythingOfType("types.ClusterName"),
		mock.AnythingOfType("types.Timestamp")).Return(
		func(orgID types.OrgID, clusterName types.ClusterName, updatedAt types.Timestamp) types.ClusterReport {
			return ""
		},
		func(orgID types.OrgID, clusterName types.ClusterName, updatedAt types.Timestamp) error {
			return sql.ErrNoRows
		},
	)
	storage.On("ReadReportForClusterAtTime",
		mock.MatchedBy(func(orgID types.OrgID) bool { return orgID == 2 }),
		mock.AnythingOfType("types.ClusterName"),
		mock.AnythingOfType("types.Timestamp")).Return(
		func(orgID types.OrgID, clusterName types.ClusterName, updatedAt types.Timestamp) types.ClusterReport {
			return "{\"reports\":[{\"rule_id\":\"rule_1|RULE_1\",\"component\":\"ccx_rules_ocp.external.rules.rule_1.report\",\"type\":\"rule\",\"key\":\"RULE_1\",\"details\":\"some details\"}]}"
		},
		func(orgID types.OrgID, clusterName types.ClusterName, updatedAt types.Timestamp) error {
			return nil
		},
	)
	storage.On("WriteNotificationRecordForCluster",
		mock.AnythingOfType("types.ClusterEntry"),
		mock.AnythingOfType("types.NotificationTypeID"),
		mock.AnythingOfType("types.StateID"),
		mock.AnythingOfType("types.ClusterReport"),
		mock.AnythingOfType("types.Timestamp"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("types.EventTarget")).Return(
		func(clusterEntry types.ClusterEntry, notificationTypeID types.NotificationTypeID, stateID types.StateID, report types.ClusterReport, notifiedAt types.Timestamp, errorLog string, eventTarget types.EventTarget) error {
			return nil
		},
	)

	errorKeys := map[string]utypes.RuleErrorKeyContent{
		"RULE_1": {
			Metadata: utypes.ErrorKeyMetadata{
				Description: "rule 1 error key description",
				Impact: utypes.Impact{
					Name:   "impact_1",
					Impact: 1,
				},
				Likelihood: 2,
			},
			Reason:    "rule 1 reason",
			HasReason: true,
		},
		"RULE_2": {
			Metadata: utypes.ErrorKeyMetadata{
				Description: "rule 2 error key description",
				Impact: utypes.Impact{
					Name:   "impact_2",
					Impact: 2,
				},
				Likelihood: 1,
			},
			HasReason: false,
		},
	}
	ruleContent := types.RulesMap{
		"rule_1": {
			Summary:    "rule 1 summary",
			Reason:     "rule 1 reason",
			Resolution: "rule 1 resolution",
			MoreInfo:   "rule 1 more info",
			ErrorKeys:  errorKeys,
			HasReason:  true,
		},
		"rule_2": {
			Summary:    "rule 2 summary",
			Reason:     "",
			Resolution: "rule 2 resolution",
			MoreInfo:   "rule 2 more info",
			ErrorKeys:  errorKeys,
			HasReason:  false,
		},
	}
	clusters := []types.ClusterEntry{
		{
			OrgID:         1,
			AccountNumber: 1,
			ClusterName:   "first_cluster",
			KafkaOffset:   0,
			UpdatedAt:     types.Timestamp(testTimestamp),
		},
		{
			OrgID:         2,
			AccountNumber: 2,
			ClusterName:   "second_cluster",
			KafkaOffset:   100,
			UpdatedAt:     types.Timestamp(testTimestamp),
		},
	}

	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.DebugLevel)
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	d := differ.Differ{
		Storage:          &storage,
		NotificationType: types.InstantNotif,
		Target:           types.NotificationBackendTarget,
	}
	d.ProcessClusters(&conf.ConfigStruct{Kafka: conf.KafkaConfiguration{Enabled: true}}, ruleContent, clusters)

	executionLog := buf.String()
	assert.Contains(t, executionLog, "No new issues to notify for cluster second_cluster", "the processReportsByCluster loop did not continue as expected")
	assert.Contains(t, executionLog, "Number of reports not retrieved/deserialized: 1", "the first cluster should have been skipped")
	assert.Contains(t, executionLog, "Number of empty reports skipped: 0")

	zerolog.SetGlobalLevel(zerolog.WarnLevel)
}

// TestProcessClustersInvalidReportFormatForClusterEntry tests that when the report found
// for a given cluster entry cannot be deserialized, the processing is not stopped
func TestProcessClustersInvalidReportFormatForClusterEntry(t *testing.T) {
	storage := mocks.Storage{}
	storage.On("ReadReportForClusterAtTime",
		mock.MatchedBy(func(orgID types.OrgID) bool { return orgID == 1 }),
		mock.AnythingOfType("types.ClusterName"),
		mock.AnythingOfType("types.Timestamp")).Return(
		func(orgID types.OrgID, clusterName types.ClusterName, updatedAt types.Timestamp) types.ClusterReport {
			return "{\"reports\":{\"rule_id\":\"rule_1|RULE_1\",\"component\":\"ccx_rules_ocp.external.rules.rule_1.report\",\"type\":\"rule\",\"key\":\"RULE_1\",\"details\":\"some details\"}}"
		},
		func(orgID types.OrgID, clusterName types.ClusterName, updatedAt types.Timestamp) error {
			return nil
		},
	)
	storage.On("ReadReportForClusterAtTime",
		mock.MatchedBy(func(orgID types.OrgID) bool { return orgID == 2 }),
		mock.AnythingOfType("types.ClusterName"),
		mock.AnythingOfType("types.Timestamp")).Return(
		func(orgID types.OrgID, clusterName types.ClusterName, updatedAt types.Timestamp) types.ClusterReport {
			return "{\"reports\":[{\"rule_id\":\"rule_1|RULE_1\",\"component\":\"ccx_rules_ocp.external.rules.rule_1.report\",\"type\":\"rule\",\"key\":\"RULE_1\",\"details\":\"some details\"}]}"
		},
		func(orgID types.OrgID, clusterName types.ClusterName, updatedAt types.Timestamp) error {
			return nil
		},
	)
	storage.On("WriteNotificationRecordForCluster",
		mock.AnythingOfType("types.ClusterEntry"),
		mock.AnythingOfType("types.NotificationTypeID"),
		mock.AnythingOfType("types.StateID"),
		mock.AnythingOfType("types.ClusterReport"),
		mock.AnythingOfType("types.Timestamp"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("types.EventTarget")).Return(
		func(clusterEntry types.ClusterEntry, notificationTypeID types.NotificationTypeID, stateID types.StateID, report types.ClusterReport, notifiedAt types.Timestamp, errorLog string, eventTarget types.EventTarget) error {
			return nil
		},
	)

	errorKeys := map[string]utypes.RuleErrorKeyContent{
		"RULE_1": {
			Metadata: utypes.ErrorKeyMetadata{
				Description: "rule 1 error key description",
				Impact: utypes.Impact{
					Name:   "impact_1",
					Impact: 1,
				},
				Likelihood: 2,
			},
			Reason:    "rule 1 reason",
			HasReason: true,
		},
		"RULE_2": {
			Metadata: utypes.ErrorKeyMetadata{
				Description: "rule 2 error key description",
				Impact: utypes.Impact{
					Name:   "impact_2",
					Impact: 2,
				},
				Likelihood: 1,
			},
			HasReason: false,
		},
	}
	ruleContent := types.RulesMap{
		"rule_1": {
			Summary:    "rule 1 summary",
			Reason:     "rule 1 reason",
			Resolution: "rule 1 resolution",
			MoreInfo:   "rule 1 more info",
			ErrorKeys:  errorKeys,
			HasReason:  true,
		},
		"rule_2": {
			Summary:    "rule 2 summary",
			Reason:     "",
			Resolution: "rule 2 resolution",
			MoreInfo:   "rule 2 more info",
			ErrorKeys:  errorKeys,
			HasReason:  false,
		},
	}
	clusters := []types.ClusterEntry{
		{
			OrgID:         1,
			AccountNumber: 1,
			ClusterName:   "first_cluster",
			KafkaOffset:   0,
			UpdatedAt:     types.Timestamp(testTimestamp),
		},
		{
			OrgID:         2,
			AccountNumber: 2,
			ClusterName:   "second_cluster",
			KafkaOffset:   100,
			UpdatedAt:     types.Timestamp(testTimestamp),
		},
	}

	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.DebugLevel)
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	d := differ.Differ{
		Storage:          &storage,
		NotificationType: types.InstantNotif,
		Target:           types.NotificationBackendTarget,
	}
	d.ProcessClusters(&conf.ConfigStruct{Kafka: conf.KafkaConfiguration{Enabled: true}}, ruleContent, clusters)

	executionLog := buf.String()
	assert.Contains(t, executionLog, "cannot unmarshal object into Go struct field Report.reports of type types.ReportContent", "The string retrieved is not a list of reports. It should not deserialize correctly")
	assert.Contains(t, executionLog, "No new issues to notify for cluster second_cluster", "the processReportsByCluster loop did not continue as expected")
	assert.Contains(t, executionLog, "Number of reports not retrieved/deserialized: 1", "the first cluster should have been skipped")
	assert.Contains(t, executionLog, "Number of empty reports skipped: 0")

	zerolog.SetGlobalLevel(zerolog.WarnLevel)
}

func TestProcessClustersInstantNotifsAndTotalRiskInferiorToThreshold(t *testing.T) {
	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.DebugLevel)
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	errorKeys := map[string]utypes.RuleErrorKeyContent{
		"RULE_1": {
			Metadata: utypes.ErrorKeyMetadata{
				Description: "rule 1 error key description",
				Impact: utypes.Impact{
					Name:   "impact_1",
					Impact: 1,
				},
				Likelihood: 2,
			},
			Reason:    "rule 1 reason",
			HasReason: true,
		},
		"RULE_2": {
			Metadata: utypes.ErrorKeyMetadata{
				Description: "rule 2 error key description",
				Impact: utypes.Impact{
					Name:   "impact_2",
					Impact: 2,
				},
				Likelihood: 1,
			},
			HasReason: false,
		},
	}

	ruleContent := types.RulesMap{
		"rule_1": {
			Summary:    "rule 1 summary",
			Reason:     "rule 1 reason",
			Resolution: "rule 1 resolution",
			MoreInfo:   "rule 1 more info",
			ErrorKeys:  errorKeys,
			HasReason:  true,
		},
		"rule_2": {
			Summary:    "rule 2 summary",
			Reason:     "",
			Resolution: "rule 2 resolution",
			MoreInfo:   "rule 2 more info",
			ErrorKeys:  errorKeys,
			HasReason:  false,
		},
	}

	clusters := []types.ClusterEntry{
		{
			OrgID:         1,
			AccountNumber: 1,
			ClusterName:   "first_cluster",
			KafkaOffset:   0,
			UpdatedAt:     types.Timestamp(testTimestamp),
		},
		{
			OrgID:         2,
			AccountNumber: 2,
			ClusterName:   "second_cluster",
			KafkaOffset:   100,
			UpdatedAt:     types.Timestamp(testTimestamp),
		},
	}

	storage := mocks.Storage{}
	storage.On("ReadReportForClusterAtTime",
		mock.AnythingOfType("types.OrgID"),
		mock.AnythingOfType("types.ClusterName"),
		mock.AnythingOfType("types.Timestamp")).Return(
		func(orgID types.OrgID, clusterName types.ClusterName, updatedAt types.Timestamp) types.ClusterReport {
			return "{\"analysis_metadata\":{\"metadata\":\"some metadata\"},\"reports\":[{\"rule_id\":\"rule_4|RULE_4\",\"component\":\"ccx_rules_ocp.external.rules.rule_1.report\",\"type\":\"rule\",\"key\":\"RULE_4\",\"details\":\"some details\"},{\"rule_id\":\"rule_4|RULE_4\",\"component\":\"ccx_rules_ocp.external.rules.rule_2.report\",\"type\":\"rule\",\"key\":\"RULE_2\",\"details\":\"some details\"},{\"rule_id\":\"rule_5|RULE_5\",\"component\":\"ccx_rules_ocp.external.rules.rule_5.report\",\"type\":\"rule\",\"key\":\"RULE_3\",\"details\":\"some details\"}]}"
		},
		func(orgID types.OrgID, clusterName types.ClusterName, updatedAt types.Timestamp) error {
			return nil
		},
	)

	storage.On("WriteNotificationRecordForCluster",
		mock.AnythingOfType("types.ClusterEntry"),
		mock.AnythingOfType("types.NotificationTypeID"),
		mock.AnythingOfType("types.StateID"),
		mock.AnythingOfType("types.ClusterReport"),
		mock.AnythingOfType("types.Timestamp"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("types.EventTarget")).Return(
		func(clusterEntry types.ClusterEntry, notificationTypeID types.NotificationTypeID, stateID types.StateID, report types.ClusterReport, notifiedAt types.Timestamp, errorLog string, eventTarget types.EventTarget) error {
			return nil
		},
	)

	d := differ.Differ{
		Storage:          &storage,
		NotificationType: types.InstantNotif,
		Target:           types.NotificationBackendTarget,
		Thresholds: differ.EventThresholds{
			TotalRisk: differ.DefaultTotalRiskThreshold,
		},
		Filter: differ.DefaultEventFilter,
	}
	d.ProcessClusters(&conf.ConfigStruct{Kafka: conf.KafkaConfiguration{Enabled: true}}, ruleContent, clusters)

	executionLog := buf.String()
	assert.Contains(t, executionLog, "No new issues to notify for cluster first_cluster", "processClusters shouldn't generate any notification for 'first_cluster' with given data")
	assert.Contains(t, executionLog, "No new issues to notify for cluster second_cluster", "processClusters shouldn't generate any notification for 'second_cluster' with given data")

	zerolog.SetGlobalLevel(zerolog.WarnLevel)
}

func TestProcessClustersInstantNotifsAndTotalRiskImportant(t *testing.T) {
	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.DebugLevel)
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	mockBroker := sarama.NewMockBroker(t, 0)
	defer mockBroker.Close()

	mockBroker.SetHandlerByMap(
		map[string]sarama.MockResponse{
			"MetadataRequest": sarama.NewMockMetadataResponse(t).
				SetBroker(mockBroker.Addr(), mockBroker.BrokerID()).
				SetLeader(brokerCfg.Topic, 0, mockBroker.BrokerID()),
			"OffsetRequest": sarama.NewMockOffsetResponse(t).
				SetOffset(brokerCfg.Topic, 0, -1, 0).
				SetOffset(brokerCfg.Topic, 0, -2, 0),
			"FetchRequest": sarama.NewMockFetchResponse(t, 1),
			"FindCoordinatorRequest": sarama.NewMockFindCoordinatorResponse(t).
				SetCoordinator(sarama.CoordinatorGroup, "", mockBroker),
			"OffsetFetchRequest": sarama.NewMockOffsetFetchResponse(t).
				SetOffset("", brokerCfg.Topic, 0, 0, "", sarama.ErrNoError),
		})

	producerMock := mocks.Producer{}
	producerMock.On("ProduceMessage", mock.AnythingOfType("types.ProducerMessage")).Return(
		func(msg types.ProducerMessage) int32 {
			testPartitionID++
			return int32(testPartitionID)
		},
		func(msg types.ProducerMessage) int64 {
			testOffset++
			return int64(testOffset)
		},
		func(msg types.ProducerMessage) error {
			return nil
		},
	)

	errorKeys := map[string]utypes.RuleErrorKeyContent{
		"RULE_1": {
			Metadata: utypes.ErrorKeyMetadata{
				Description: "rule 1 error key description",
				Impact: utypes.Impact{
					Name:   "impact_1",
					Impact: 3,
				},
				Likelihood: 4,
			},
			Reason:    "rule 1 reason",
			HasReason: true,
		},
		"RULE_2": {
			Metadata: utypes.ErrorKeyMetadata{
				Description: "rule 2 error key description",
				Impact: utypes.Impact{
					Name:   "impact_2",
					Impact: 3,
				},
				Likelihood: 3,
			},
			HasReason: false,
		},
	}

	ruleContent := types.RulesMap{
		"rule_1": {
			Summary:    "rule 1 summary",
			Reason:     "rule 1 reason",
			Resolution: "rule 1 resolution",
			MoreInfo:   "rule 1 more info",
			ErrorKeys:  errorKeys,
			HasReason:  true,
		},
		"rule_2": {
			Summary:    "rule 2 summary",
			Reason:     "",
			Resolution: "rule 2 resolution",
			MoreInfo:   "rule 2 more info",
			ErrorKeys:  errorKeys,
			HasReason:  false,
		},
	}

	clusters := []types.ClusterEntry{
		{
			OrgID:         1,
			AccountNumber: 1,
			ClusterName:   "first_cluster",
			KafkaOffset:   0,
			UpdatedAt:     types.Timestamp(testTimestamp),
		},
		{
			OrgID:         2,
			AccountNumber: 2,
			ClusterName:   "second_cluster",
			KafkaOffset:   100,
			UpdatedAt:     types.Timestamp(testTimestamp),
		},
	}

	storage := mocks.Storage{}
	storage.On("ReadReportForClusterAtTime",
		mock.AnythingOfType("types.OrgID"),
		mock.AnythingOfType("types.ClusterName"),
		mock.AnythingOfType("types.Timestamp")).Return(
		func(orgID types.OrgID, clusterName types.ClusterName, updatedAt types.Timestamp) types.ClusterReport {
			return "{\"analysis_metadata\":{\"metadata\":\"some metadata\"},\"reports\":[{\"rule_id\":\"rule_1|RULE_1\",\"component\":\"ccx_rules_ocp.external.rules.rule_1.report\",\"type\":\"rule\",\"key\":\"RULE_1\",\"details\":\"some details\"}]}"
		},
		func(orgID types.OrgID, clusterName types.ClusterName, updatedAt types.Timestamp) error {
			return nil
		},
	)
	storage.On("ReadLastNNotifiedRecords", mock.AnythingOfType("types.ClusterEntry"), mock.AnythingOfType("int")).Return(
		func(clusterEntry types.ClusterEntry, numberOfRecords int) []types.NotificationRecord {
			// Return a record that is different from the one that will be processed so notification is sent
			return []types.NotificationRecord{
				{
					OrgID:              3,
					AccountNumber:      4,
					ClusterName:        "a cluster",
					UpdatedAt:          types.Timestamp(testTimestamp),
					NotificationTypeID: 0,
					StateID:            0,
					Report:             "{\"analysis_metadata\":{\"metadata\":\"some metadata\"},\"reports\":[{\"rule_id\":\"rule_4|RULE_4\",\"component\":\"ccx_rules_ocp.external.rules.rule_4.report\",\"type\":\"rule\",\"key\":\"RULE_4\",\"details\":\"some details\"},{\"rule_id\":\"rule_4|RULE_4\",\"component\":\"ccx_rules_ocp.external.rules.rule_2.report\",\"type\":\"rule\",\"key\":\"RULE_2\",\"details\":\"some details\"},{\"rule_id\":\"rule_5|RULE_5\",\"component\":\"ccx_rules_ocp.external.rules.rule_5.report\",\"type\":\"rule\",\"key\":\"RULE_3\",\"details\":\"some details\"}]}",
					NotifiedAt:         types.Timestamp(testTimestamp.Add(-2)),
					ErrorLog:           "",
				},
			}
		},
		func(clusterEntry types.ClusterEntry, numberOfRecords int) error {
			return nil
		},
	)
	storage.On("WriteNotificationRecordForCluster",
		mock.AnythingOfType("types.ClusterEntry"),
		mock.AnythingOfType("types.NotificationTypeID"),
		mock.AnythingOfType("types.StateID"),
		mock.AnythingOfType("types.ClusterReport"),
		mock.AnythingOfType("types.Timestamp"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("types.EventTarget")).Return(
		func(clusterEntry types.ClusterEntry, notificationTypeID types.NotificationTypeID, stateID types.StateID, report types.ClusterReport, notifiedAt types.Timestamp, errorLog string, eventTarget types.EventTarget) error {
			return nil
		},
	)

	d := differ.Differ{
		Storage:          &storage,
		NotificationType: types.InstantNotif,
		Target:           types.NotificationBackendTarget,
		Thresholds: differ.EventThresholds{
			TotalRisk: differ.DefaultTotalRiskThreshold,
		},
		Filter:   differ.DefaultEventFilter,
		Notifier: &producerMock,
	}
	d.ProcessClusters(&conf.ConfigStruct{Kafka: conf.KafkaConfiguration{Enabled: true}}, ruleContent, clusters)

	executionLog := buf.String()
	assert.Contains(t, executionLog, differ.ReportWithHighImpactMessage, "processClusters should create a notification for 'first_cluster' with given data")
	assert.Contains(t, executionLog, "{\"level\":\"debug\",\"cluster\":\"first_cluster\",\"number of events\":1,\"message\":\"Producing instant notification\"}", "processClusters should generate one notification for 'first_cluster' with given data")
	assert.Contains(t, executionLog, "{\"level\":\"debug\",\"cluster\":\"second_cluster\",\"number of events\":1,\"message\":\"Producing instant notification\"}", "processClusters should generate one notification for 'first_cluster' with given data")

	zerolog.SetGlobalLevel(zerolog.WarnLevel)
}

func TestProcessClustersInstantNotifsAndTotalRiskCritical(t *testing.T) {
	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.DebugLevel)
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	mockBroker := sarama.NewMockBroker(t, 0)
	defer mockBroker.Close()

	mockBroker.SetHandlerByMap(
		map[string]sarama.MockResponse{
			"MetadataRequest": sarama.NewMockMetadataResponse(t).
				SetBroker(mockBroker.Addr(), mockBroker.BrokerID()).
				SetLeader(brokerCfg.Topic, 0, mockBroker.BrokerID()),
			"OffsetRequest": sarama.NewMockOffsetResponse(t).
				SetOffset(brokerCfg.Topic, 0, -1, 0).
				SetOffset(brokerCfg.Topic, 0, -2, 0),
			"FetchRequest": sarama.NewMockFetchResponse(t, 1),
			"FindCoordinatorRequest": sarama.NewMockFindCoordinatorResponse(t).
				SetCoordinator(sarama.CoordinatorGroup, "", mockBroker),
			"OffsetFetchRequest": sarama.NewMockOffsetFetchResponse(t).
				SetOffset("", brokerCfg.Topic, 0, 0, "", sarama.ErrNoError),
		})

	producerMock := mocks.Producer{}
	producerMock.On("ProduceMessage", mock.AnythingOfType("types.ProducerMessage")).Return(
		func(msg types.ProducerMessage) int32 {
			testPartitionID++
			return int32(testPartitionID)
		},
		func(msg types.ProducerMessage) int64 {
			testOffset++
			return int64(testOffset)
		},
		func(msg types.ProducerMessage) error {
			return nil
		},
	)

	errorKeys := map[string]utypes.RuleErrorKeyContent{
		"RULE_1": {
			Metadata: utypes.ErrorKeyMetadata{
				Description: "rule 1 error key description",
				Impact: utypes.Impact{
					Name:   "impact_1",
					Impact: 4,
				},
				Likelihood: 4,
			},
			Reason:    "rule 1 reason",
			HasReason: true,
		},
		"RULE_2": {
			Metadata: utypes.ErrorKeyMetadata{
				Description: "rule 2 error key description",
				Impact: utypes.Impact{
					Name:   "impact_2",
					Impact: 4,
				},
				Likelihood: 4,
			},
			HasReason: false,
		},
	}

	ruleContent := types.RulesMap{
		"rule_1": {
			Summary:    "rule 1 summary",
			Reason:     "rule 1 reason",
			Resolution: "rule 1 resolution",
			MoreInfo:   "rule 1 more info",
			ErrorKeys:  errorKeys,
			HasReason:  true,
		},
		"rule_2": {
			Summary:    "rule 2 summary",
			Reason:     "",
			Resolution: "rule 2 resolution",
			MoreInfo:   "rule 2 more info",
			ErrorKeys:  errorKeys,
			HasReason:  false,
		},
	}

	clusters := []types.ClusterEntry{
		{
			OrgID:         1,
			AccountNumber: 1,
			ClusterName:   "first_cluster",
			KafkaOffset:   0,
			UpdatedAt:     types.Timestamp(testTimestamp),
		},
		{
			OrgID:         2,
			AccountNumber: 2,
			ClusterName:   "second_cluster",
			KafkaOffset:   100,
			UpdatedAt:     types.Timestamp(testTimestamp),
		},
	}

	storage := mocks.Storage{}
	storage.On("ReadReportForClusterAtTime",
		mock.AnythingOfType("types.OrgID"),
		mock.AnythingOfType("types.ClusterName"),
		mock.AnythingOfType("types.Timestamp")).Return(
		func(orgID types.OrgID, clusterName types.ClusterName, updatedAt types.Timestamp) types.ClusterReport {
			return "{\"analysis_metadata\":{\"metadata\":\"some metadata\"},\"reports\":[{\"rule_id\":\"rule_1|RULE_1\",\"component\":\"ccx_rules_ocp.external.rules.rule_1.report\",\"type\":\"rule\",\"key\":\"RULE_1\",\"details\":\"some details\"}]}"
		},
		func(orgID types.OrgID, clusterName types.ClusterName, updatedAt types.Timestamp) error {
			return nil
		},
	)
	storage.On("WriteNotificationRecordForCluster",
		mock.AnythingOfType("types.ClusterEntry"),
		mock.AnythingOfType("types.NotificationTypeID"),
		mock.AnythingOfType("types.StateID"),
		mock.AnythingOfType("types.ClusterReport"),
		mock.AnythingOfType("types.Timestamp"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("types.EventTarget")).Return(
		func(clusterEntry types.ClusterEntry, notificationTypeID types.NotificationTypeID, stateID types.StateID, report types.ClusterReport, notifiedAt types.Timestamp, errorLog string, eventTarget types.EventTarget) error {
			return nil
		},
	)

	d := differ.Differ{
		Storage:          &storage,
		NotificationType: types.InstantNotif,
		Target:           types.NotificationBackendTarget,
		Thresholds: differ.EventThresholds{
			TotalRisk: differ.DefaultTotalRiskThreshold,
		},
		Filter:   differ.DefaultEventFilter,
		Notifier: &producerMock,
	}
	d.ProcessClusters(&conf.ConfigStruct{Kafka: conf.KafkaConfiguration{Enabled: true}}, ruleContent, clusters)

	executionLog := buf.String()
	assert.Contains(t, executionLog, fmt.Sprintf("{\"level\":\"debug\",\"type\":\"rule\",\"rule\":\"rule_1\",\"error key\":\"RULE_1\",\"likelihood\":4,\"impact\":4,\"totalRisk\":4,\"message\":\"%s\"}\n", differ.ReportWithHighImpactMessage))
	assert.Contains(t, executionLog, "{\"level\":\"debug\",\"cluster\":\"first_cluster\",\"number of events\":1,\"message\":\"Producing instant notification\"}", "processClusters should generate one notification for 'first_cluster' with given data")
	assert.Contains(t, executionLog, "{\"level\":\"debug\",\"cluster\":\"second_cluster\",\"number of events\":1,\"message\":\"Producing instant notification\"}", "processClusters should generate one notification for 'first_cluster' with given data")

}

func TestProcessClustersAllIssuesAlreadyNotifiedCooldownNotPassed(t *testing.T) {
	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.DebugLevel)
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	errorKeys := map[string]utypes.RuleErrorKeyContent{
		"RULE_1": {
			Metadata: utypes.ErrorKeyMetadata{
				Description: "rule 1 error key description",
				Impact: utypes.Impact{
					Name:   "impact_1",
					Impact: 3,
				},
				Likelihood: 4,
			},
			Reason:    "rule 1 reason",
			HasReason: true,
		},
	}

	ruleContent := types.RulesMap{
		"rule_1": {
			Summary:    "rule 1 summary",
			Reason:     "rule 1 reason",
			Resolution: "rule 1 resolution",
			MoreInfo:   "rule 1 more info",
			ErrorKeys:  errorKeys,
			HasReason:  true,
		},
	}

	clusters := []types.ClusterEntry{
		{
			OrgID:         types.OrgID(1),
			AccountNumber: 1,
			ClusterName:   "first_cluster",
			KafkaOffset:   0,
			UpdatedAt:     types.Timestamp(testTimestamp),
		},
		{
			OrgID:         types.OrgID(2),
			AccountNumber: 2,
			ClusterName:   "second_cluster",
			KafkaOffset:   100,
			UpdatedAt:     types.Timestamp(testTimestamp),
		},
	}

	storage := mocks.Storage{}
	storage.On("ReadReportForClusterAtTime",
		mock.AnythingOfType("types.OrgID"),
		mock.AnythingOfType("types.ClusterName"),
		mock.AnythingOfType("types.Timestamp")).Return(
		func(orgID types.OrgID, clusterName types.ClusterName, updatedAt types.Timestamp) types.ClusterReport {
			return "{\"analysis_metadata\":{\"metadata\":\"some metadata\"},\"reports\":[{\"rule_id\":\"rule_1|RULE_1\",\"component\":\"ccx_rules_ocp.external.rules.rule_1.report\",\"type\":\"rule\",\"key\":\"RULE_1\",\"details\":\"some details\"}]}"
		},
		func(orgID types.OrgID, clusterName types.ClusterName, updatedAt types.Timestamp) error {
			return nil
		},
	)

	storage.On("WriteNotificationRecordForCluster",
		mock.AnythingOfType("types.ClusterEntry"),
		mock.AnythingOfType("types.NotificationTypeID"),
		mock.AnythingOfType("types.StateID"),
		mock.AnythingOfType("types.ClusterReport"),
		mock.AnythingOfType("types.Timestamp"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("types.EventTarget")).Return(
		func(clusterEntry types.ClusterEntry, notificationTypeID types.NotificationTypeID, stateID types.StateID, report types.ClusterReport, notifiedAt types.Timestamp, errorLog string, eventTarget types.EventTarget) error {
			return nil
		},
	)

	d := differ.Differ{
		Storage:          &storage,
		NotificationType: types.InstantNotif,
		Target:           types.NotificationBackendTarget,
		Thresholds: differ.EventThresholds{
			TotalRisk: differ.DefaultTotalRiskThreshold,
		},
		Filter: differ.DefaultEventFilter,
	}

	d.PreviouslyReported = make(types.NotifiedRecordsPerCluster, 2)
	d.PreviouslyReported[types.ClusterOrgKey{OrgID: types.OrgID(1), ClusterName: "first_cluster"}] = types.NotificationRecord{
		OrgID:              1,
		AccountNumber:      4,
		ClusterName:        "first_cluster",
		UpdatedAt:          types.Timestamp(testTimestamp),
		NotificationTypeID: 0,
		StateID:            0,
		Report:             "{\"analysis_metadata\":{\"metadata\":\"some metadata\"},\"reports\":[{\"rule_id\":\"rule_1|RULE_1\",\"component\":\"ccx_rules_ocp.external.rules.rule_1.report\",\"type\":\"rule\",\"key\":\"RULE_1\",\"details\":\"some details\"}]}",
		NotifiedAt:         types.Timestamp(testTimestamp.Add(-2)),
		ErrorLog:           "",
	}

	d.PreviouslyReported[types.ClusterOrgKey{OrgID: types.OrgID(2), ClusterName: "second_cluster"}] = types.NotificationRecord{
		OrgID:              2,
		AccountNumber:      4,
		ClusterName:        "second_cluster",
		UpdatedAt:          types.Timestamp(testTimestamp),
		NotificationTypeID: 0,
		StateID:            0,
		Report:             "{\"analysis_metadata\":{\"metadata\":\"some metadata\"},\"reports\":[{\"rule_id\":\"rule_1|RULE_1\",\"component\":\"ccx_rules_ocp.external.rules.rule_1.report\",\"type\":\"rule\",\"key\":\"RULE_1\",\"details\":\"some details\"}]}",
		NotifiedAt:         types.Timestamp(testTimestamp.Add(-2)),
		ErrorLog:           "",
	}
	d.ProcessClusters(&conf.ConfigStruct{Kafka: conf.KafkaConfiguration{Enabled: true}}, ruleContent, clusters)

	executionLog := buf.String()
	assert.Contains(t, executionLog, "{\"level\":\"debug\",\"message\":\"No new issues to notify for cluster first_cluster\"}\n", "Notification already sent for first_cluster's report, but corresponding log not found.")
	assert.Contains(t, executionLog, "{\"level\":\"debug\",\"message\":\"No new issues to notify for cluster second_cluster\"}\n", "Notification already sent for second_cluster's report, but corresponding log not found.")
}

func TestProcessClustersNewIssuesNotPreviouslyNotified(t *testing.T) {
	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.DebugLevel)
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	mockBroker := sarama.NewMockBroker(t, 0)
	defer mockBroker.Close()

	mockBroker.SetHandlerByMap(
		map[string]sarama.MockResponse{
			"MetadataRequest": sarama.NewMockMetadataResponse(t).
				SetBroker(mockBroker.Addr(), mockBroker.BrokerID()).
				SetLeader(brokerCfg.Topic, 0, mockBroker.BrokerID()),
			"OffsetRequest": sarama.NewMockOffsetResponse(t).
				SetOffset(brokerCfg.Topic, 0, -1, 0).
				SetOffset(brokerCfg.Topic, 0, -2, 0),
			"FetchRequest": sarama.NewMockFetchResponse(t, 1),
			"FindCoordinatorRequest": sarama.NewMockFindCoordinatorResponse(t).
				SetCoordinator(sarama.CoordinatorGroup, "", mockBroker),
			"OffsetFetchRequest": sarama.NewMockOffsetFetchResponse(t).
				SetOffset("", brokerCfg.Topic, 0, 0, "", sarama.ErrNoError),
		})

	errorKeys := map[string]utypes.RuleErrorKeyContent{
		"RULE_1": {
			Metadata: utypes.ErrorKeyMetadata{
				Description: "rule 1 error key description",
				Impact: utypes.Impact{
					Name:   "impact_1",
					Impact: 4,
				},
				Likelihood: 4,
			},
			Reason:    "rule 1 reason",
			HasReason: true,
		},
		"RULE_2": {
			Metadata: utypes.ErrorKeyMetadata{
				Description: "rule 2 error key description",
				Impact: utypes.Impact{
					Name:   "impact_2",
					Impact: 4,
				},
				Likelihood: 4,
			},
			HasReason: false,
		},
	}

	ruleContent := types.RulesMap{
		"rule_1": {
			Summary:    "rule 1 summary",
			Reason:     "rule 1 reason",
			Resolution: "rule 1 resolution",
			MoreInfo:   "rule 1 more info",
			ErrorKeys:  errorKeys,
			HasReason:  true,
		},
		"rule_2": {
			Summary:    "rule 2 summary",
			Reason:     "",
			Resolution: "rule 2 resolution",
			MoreInfo:   "rule 2 more info",
			ErrorKeys:  errorKeys,
			HasReason:  false,
		},
	}

	clusters := []types.ClusterEntry{
		{
			OrgID:         1,
			AccountNumber: 1,
			ClusterName:   "first_cluster",
			KafkaOffset:   0,
			UpdatedAt:     types.Timestamp(testTimestamp),
		},
		{
			OrgID:         2,
			AccountNumber: 2,
			ClusterName:   "second_cluster",
			KafkaOffset:   100,
			UpdatedAt:     types.Timestamp(testTimestamp),
		},
	}

	storage := mocks.Storage{}
	storage.On("ReadReportForClusterAtTime",
		mock.AnythingOfType("types.OrgID"),
		mock.AnythingOfType("types.ClusterName"),
		mock.AnythingOfType("types.Timestamp")).Return(
		func(orgID types.OrgID, clusterName types.ClusterName, updatedAt types.Timestamp) types.ClusterReport {
			return "{\"analysis_metadata\":{\"metadata\":\"some metadata\"},\"reports\":[{\"rule_id\":\"rule_1|RULE_1\",\"component\":\"ccx_rules_ocp.external.rules.rule_1.report\",\"type\":\"rule\",\"key\":\"RULE_1\",\"details\":\"some details\"}]}"
		},
		func(orgID types.OrgID, clusterName types.ClusterName, updatedAt types.Timestamp) error {
			return nil
		},
	)

	storage.On("WriteNotificationRecordForCluster",
		mock.AnythingOfType("types.ClusterEntry"),
		mock.AnythingOfType("types.NotificationTypeID"),
		mock.AnythingOfType("types.StateID"),
		mock.AnythingOfType("types.ClusterReport"),
		mock.AnythingOfType("types.Timestamp"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("types.EventTarget")).Return(
		func(clusterEntry types.ClusterEntry, notificationTypeID types.NotificationTypeID, stateID types.StateID, report types.ClusterReport, notifiedAt types.Timestamp, errorLog string, eventTarget types.EventTarget) error {
			return nil
		},
	)

	producerMock := mocks.Producer{}
	producerMock.On("ProduceMessage", mock.AnythingOfType("types.ProducerMessage")).Return(
		func(msg types.ProducerMessage) int32 {
			testPartitionID++
			return int32(testPartitionID)
		},
		func(msg types.ProducerMessage) int64 {
			testOffset++
			return int64(testOffset)
		},
		func(msg types.ProducerMessage) error {
			return nil
		},
	)

	d := differ.Differ{
		Storage:          &storage,
		NotificationType: types.InstantNotif,
		Target:           types.NotificationBackendTarget,
		Thresholds: differ.EventThresholds{
			TotalRisk: differ.DefaultTotalRiskThreshold,
		},
		Filter: differ.DefaultEventFilter,
	}

	d.Notifier = &producerMock

	d.PreviouslyReported = make(types.NotifiedRecordsPerCluster, 1)
	d.PreviouslyReported[types.ClusterOrgKey{OrgID: types.OrgID(3), ClusterName: "a cluster"}] = types.NotificationRecord{
		OrgID:              3,
		AccountNumber:      4,
		ClusterName:        "a cluster",
		UpdatedAt:          types.Timestamp(testTimestamp),
		NotificationTypeID: 0,
		StateID:            1,
		Report:             "{\"analysis_metadata\":{\"metadata\":\"some metadata\"},\"reports\":[{\"rule_id\":\"rule_1|RULE_1\",\"component\":\"ccx_rules_ocp.external.rules.rule_1.report\",\"type\":\"rule\",\"key\":\"RULE_4\",\"details\":\"some details\"}]}",
		NotifiedAt:         types.Timestamp(testTimestamp.Add(-2)),
		ErrorLog:           "",
	}

	d.ProcessClusters(&conf.ConfigStruct{Kafka: conf.KafkaConfiguration{Enabled: true}}, ruleContent, clusters)

	executionLog := buf.String()
	assert.Contains(t, executionLog, fmt.Sprintf("{\"level\":\"debug\",\"type\":\"rule\",\"rule\":\"rule_1\",\"error key\":\"RULE_1\",\"likelihood\":4,\"impact\":4,\"totalRisk\":4,\"message\":\"%s\"}\n", differ.ReportWithHighImpactMessage))
	assert.Contains(t, executionLog, "{\"level\":\"debug\",\"cluster\":\"first_cluster\",\"number of events\":1,\"message\":\"Producing instant notification\"}", "processClusters should generate one notification for 'first_cluster' with given data")
	assert.Contains(t, executionLog, "{\"level\":\"debug\",\"cluster\":\"second_cluster\",\"number of events\":1,\"message\":\"Producing instant notification\"}", "processClusters should generate one notification for 'second_cluster' with given data")
}

func TestRetrievePreviouslyReportedForEventTarget(t *testing.T) {
	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.InfoLevel)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	var (
		now            = time.Now()
		clusters       = "'first cluster','second cluster'"
		orgs           = "'1','2'"
		clusterEntries = []types.ClusterEntry{
			{
				OrgID:         1,
				AccountNumber: 1,
				ClusterName:   "first cluster",
				KafkaOffset:   1,
				UpdatedAt:     types.Timestamp(now),
			},
			{
				OrgID:         2,
				AccountNumber: 2,
				ClusterName:   "second cluster",
				KafkaOffset:   1,
				UpdatedAt:     types.Timestamp(now),
			},
		}
		timeOffset = "1 day"
	)
	producerMock := mocks.Producer{}
	producerMock.On("ProduceMessage", mock.AnythingOfType("types.ProducerMessage")).Return(
		func(msg types.ProducerMessage) int32 {
			testPartitionID++
			return int32(testPartitionID)
		},
		func(msg types.ProducerMessage) int64 {
			testOffset++
			return int64(testOffset)
		},
		func(msg types.ProducerMessage) error {
			return nil
		},
	)
	db, mock := newMock(t)
	defer func() { _ = db.Close() }()

	sut := differ.NewFromConnection(db, types.DBDriverPostgres)
	d := differ.Differ{
		Storage:          sut,
		NotificationType: types.InstantNotif,
		Target:           types.NotificationBackendTarget,
		Thresholds: differ.EventThresholds{
			TotalRisk: differ.DefaultTotalRiskThreshold,
		},
		Filter:   differ.DefaultEventFilter,
		Notifier: &producerMock,
	}
	expectedQuery := fmt.Sprintf(`
	SELECT org_id, cluster, report, notified_at
	FROM (
		SELECT DISTINCT ON (cluster) *
		FROM reported
		WHERE event_type_id = %v AND state = 1 AND org_id IN (%v) AND cluster IN (%v)
		ORDER BY cluster, notified_at DESC) t
	WHERE notified_at > NOW() - $1::INTERVAL ;
	`, types.NotificationBackendTarget, orgs, clusters)

	rows := sqlmock.NewRows(
		[]string{"org_id", "cluster", "report", "notified_at"}).
		AddRow(1, "first cluster", "test", now).
		AddRow(1, "second cluster", "test", now)

	mock.ExpectQuery(regexp.QuoteMeta(expectedQuery)).
		WithArgs(timeOffset).
		WillReturnRows(rows)

	err := d.RetrievePreviouslyReportedForEventTarget(timeOffset, types.NotificationBackendTarget, clusterEntries)
	assert.Nil(t, err)
	executionLog := buf.String()
	assert.Contains(t, executionLog, "{\"level\":\"info\",\"message\":\"Reading previously reported issues for given cluster list...\"}\n{\"level\":\"info\",\"target\":1,\"retrieved\":2,\"message\":\"Done reading previously reported issues still in cool down\"}\n")
}

func TestRetrievePreviouslyReportedForEventTargetEmptyClusterEntries(t *testing.T) {
	clusterEntries := []types.ClusterEntry{}
	timeOffset := "1 day"
	buf := new(bytes.Buffer)

	log.Logger = zerolog.New(buf).Level(zerolog.InfoLevel)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	producerMock := mocks.Producer{}
	producerMock.On("ProduceMessage", mock.AnythingOfType("types.ProducerMessage")).Return(
		func(msg types.ProducerMessage) int32 {
			testPartitionID++
			return int32(testPartitionID)
		},
		func(msg types.ProducerMessage) int64 {
			testOffset++
			return int64(testOffset)
		},
		func(msg types.ProducerMessage) error {
			return nil
		},
	)

	// prepare database mock
	db, _ := newMock(t)
	defer func() { _ = db.Close() }()

	// establish connection to mocked database
	sut := differ.NewFromConnection(db, types.DBDriverPostgres)

	d := differ.Differ{
		Storage:          sut,
		NotificationType: types.InstantNotif,
		Target:           types.NotificationBackendTarget,
		Thresholds: differ.EventThresholds{
			TotalRisk: differ.DefaultTotalRiskThreshold,
		},
		Filter:   differ.DefaultEventFilter,
		Notifier: &producerMock,
	}

	// call tested method
	err := d.RetrievePreviouslyReportedForEventTarget(timeOffset, types.NotificationBackendTarget, clusterEntries)
	assert.Nil(t, err)
	// test returned values
	executionLog := buf.String()
	assert.Contains(t, executionLog, "{\"level\":\"info\",\"message\":\"Reading previously reported issues for given cluster list...\"}\n{\"level\":\"info\",\"target\":1,\"retrieved\":0,\"message\":\"Done reading previously reported issues still in cool down\"}\n")
}

// Test the function closeStorage from differ.go
func TestCloseStorage(t *testing.T) {
	config := conf.StorageConfiguration{
		Driver: "sqlite3",
	}
	storage, err := differ.NewStorage(&config)
	assert.Nil(t, err, "Storage should be created w/o error")

	err = differ.CloseStorage(storage)
	assert.Nil(t, err, "Storage should be closed w/o error")
}

// TestSetupFiltersAndThresholds_Kafka tests proper configuration of filters and thresholds based on config
func TestSetupFiltersAndThresholds_Kafka(t *testing.T) {
	tagsSet := types.MakeSetOfTags([]string{"tag1", "tag2"})

	testConfig := conf.ConfigStruct{
		Kafka: conf.KafkaConfiguration{
			Addresses:           "localhost:9092",
			Topic:               "platform.notifications.ingress",
			Enabled:             true,
			LikelihoodThreshold: 1,
			ImpactThreshold:     2,
			SeverityThreshold:   3,
			TotalRiskThreshold:  4,
			TagFilterEnabled:    true,
			TagsSet:             tagsSet,
		},
	}

	expectedThresholds := differ.EventThresholds{
		Likelihood: 1,
		Impact:     2,
		Severity:   3,
		TotalRisk:  4,
	}

	d := differ.Differ{}
	err := d.SetupFiltersAndThresholds(&testConfig)

	assert.Nil(t, err)
	assert.Equal(t, d.Thresholds, expectedThresholds)
	assert.Equal(t, d.Filter, differ.DefaultEventFilter)
	assert.Equal(t, d.FilterByTag, true)
	assert.Equal(t, d.TagsSet, tagsSet)
}

// TestSetupFiltersAndThresholds_Kafka_NonDefaultFilter tests proper configuration of differ event filter
func TestSetupFiltersAndThresholds_Kafka_NonDefaultFilter(t *testing.T) {
	const customFilter = "totalRisk >= totalRiskThreshold && likelihood+2 > likelihoodThreshold"

	testConfig := conf.ConfigStruct{
		Kafka: conf.KafkaConfiguration{
			Addresses:   "localhost:9092",
			Topic:       "platform.notifications.ingress",
			Enabled:     true,
			EventFilter: customFilter,
		},
	}

	d := differ.Differ{}
	err := d.SetupFiltersAndThresholds(&testConfig)

	assert.Nil(t, err)
	assert.Equal(t, d.Filter, customFilter)
}

// TestSetupFiltersAndThresholds_Servicelog tests proper configuration of filters and thresholds based on config
func TestSetupFiltersAndThresholds_Servicelog(t *testing.T) {
	tagsSet := types.MakeSetOfTags([]string{"tag1", "tag2"})

	testConfig := conf.ConfigStruct{
		ServiceLog: conf.ServiceLogConfiguration{
			URL:                 "localhost:9092",
			Enabled:             true,
			LikelihoodThreshold: 1,
			ImpactThreshold:     2,
			SeverityThreshold:   3,
			TotalRiskThreshold:  4,
			TagFilterEnabled:    true,
			TagsSet:             tagsSet,
		},
	}

	expectedThresholds := differ.EventThresholds{
		Likelihood: 1,
		Impact:     2,
		Severity:   3,
		TotalRisk:  4,
	}

	d := differ.Differ{}
	err := d.SetupFiltersAndThresholds(&testConfig)

	assert.Nil(t, err)
	assert.Equal(t, d.Thresholds, expectedThresholds)
	assert.Equal(t, d.Filter, differ.DefaultEventFilter)
	assert.Equal(t, d.FilterByTag, true)
	assert.Equal(t, d.TagsSet, tagsSet)
}

// TestSetupFiltersAndThresholds_Servicelog_NonDefaultFilter tests proper configuration of differ event filter
func TestSetupFiltersAndThresholds_Servicelog_NonDefaultFilter(t *testing.T) {
	const customFilter = "totalRisk >= totalRiskThreshold && likelihood+2 > likelihoodThreshold"

	testConfig := conf.ConfigStruct{
		ServiceLog: conf.ServiceLogConfiguration{
			URL:         "localhost:9092",
			Enabled:     true,
			EventFilter: customFilter,
		},
	}

	d := differ.Differ{}
	err := d.SetupFiltersAndThresholds(&testConfig)

	assert.Nil(t, err)
	assert.Equal(t, d.Filter, customFilter)
}

func TestRunStatusStorageError(t *testing.T) {
	retval := differ.Run(conf.ConfigStruct{}, types.CliFlags{})
	assert.Equal(t, differ.ExitStatusStorageError, retval)
}

func TestRunStatusCleanerError(t *testing.T) {
	config := conf.ConfigStruct{
		Storage: conf.StorageConfiguration{
			Driver: "sqlite3",
		},
	}
	cliFlags := types.CliFlags{
		PrintNewReportsForCleanup: true,
	}
	retval := differ.Run(config, cliFlags)
	assert.Equal(t, differ.ExitStatusCleanerError, retval)
}

func TestRunStatusCleanerError2(t *testing.T) {
	config := conf.ConfigStruct{
		Storage: conf.StorageConfiguration{
			Driver: "sqlite3",
		},
	}
	cliFlags := types.CliFlags{
		CleanupOnStartup: true,
	}
	retval := differ.Run(config, cliFlags)
	assert.Equal(t, differ.ExitStatusCleanerError, retval)
}

func TestRunDifferNewError(t *testing.T) {
	config := conf.ConfigStruct{
		Storage: conf.StorageConfiguration{
			Driver: "sqlite3",
		},
	}
	retval := differ.Run(config, types.CliFlags{})
	assert.Equal(t, differ.ExitStatusConfiguration, retval)
}
