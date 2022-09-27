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

// Documentation in literate-programming-style is available at:
// https://redhatinsights.github.io/ccx-notification-writer/packages/differ/differ_test.html

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/RedHatInsights/ccx-notification-service/producer/kafka"
	"github.com/RedHatInsights/insights-results-aggregator-data/testdata"

	utypes "github.com/RedHatInsights/insights-results-types"

	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/tests/mocks"
	"github.com/RedHatInsights/ccx-notification-service/types"
	"github.com/Shopify/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	brokerCfg = conf.KafkaConfiguration{
		Address:     "localhost:9092",
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

type Payload struct {
	PublishDate     string `json:"publish_date"`
	RuleDescription string `json:"rule_description"`
	RuleURL         string `json:"rule_url"`
	TotalRisk       string `json:"total_risk"`
}

func init() {
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	notificationType = types.InstantNotif
}

// Can't redirect zerolog/log to buffer directly in some tests due to clowder
// init
func captureStdout(f func()) string {
	originalStdOutFile := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	_ = w.Close()
	os.Stdout = originalStdOutFile

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

// ---------------------------------------------------------------------------------------
func TestToJSONEscapedStringValidJSON(t *testing.T) {
	type testStruct struct {
		Name         string
		Items        []string
		ID           int64
		privateField string // An unexported field is not encoded, so our method would not include it in output.
	}

	tested := testStruct{
		Name:         "Random Name",
		Items:        []string{"Apple", "Dell", "Orange", "Piña"},
		ID:           999,
		privateField: "a random field that nobody can see cuz I don't wanna",
	}

	expectedEscapedJSONString := "{\"Name\":\"Random Name\",\"Items\":[\"Apple\",\"Dell\",\"Orange\",\"Piña\"],\"ID\":999}"

	assert.Equal(t, expectedEscapedJSONString, toJSONEscapedString(tested))
}

func TestToJSONEscapedStringInvalidJSON(t *testing.T) {
	type testStruct struct {
		ID           int64
		InvalidField chan int // A value that cannot be represented in JSON
	}

	tested := testStruct{
		ID:           999,
		InvalidField: make(chan int),
	}

	expectedEscapedJSONString := ""

	assert.Equal(t, expectedEscapedJSONString, toJSONEscapedString(tested))
}

// ---------------------------------------------------------------------------------------
func TestGenerateInstantReportNotificationMessage(t *testing.T) {
	clusterURI := "the_cluster_uri_in_ocm_for_{cluster_id}"
	accountID := "a_stringified_account_id"
	orgID := "a_stringified_org_id"
	clusterID := "the_displayed_cluster_ID"

	notificationMsg := generateInstantNotificationMessage(&clusterURI, accountID, orgID, clusterID)

	assert.NotEmpty(t, notificationMsg, "the generated notification message is empty")
	assert.Empty(t, notificationMsg.Events, "the generated notification message should not have any events")
	assert.Equal(t, types.InstantNotif.ToString(), notificationMsg.EventType, "the generated notification message should be for instant notifications")
	assert.Equal(t, "{\"display_name\":\"the_displayed_cluster_ID\",\"host_url\":\"the_cluster_uri_in_ocm_for_the_displayed_cluster_ID\"}", notificationMsg.Context, "Notification context is different from expected.")
	assert.Equal(t, notificationBundleName, notificationMsg.Bundle, "Generated notifications should indicate 'openshift' as bundle")
	assert.Equal(t, notificationApplicationName, notificationMsg.Application, "Generated notifications should indicate 'openshift' as application name")
	assert.Equal(t, accountID, notificationMsg.AccountID, "Generated notifications does not have expected account ID")
	assert.Equal(t, orgID, notificationMsg.OrgID, "Generated notifications does not have expected org ID")
}

func TestAppendEventsToExistingInstantReportNotificationMsg(t *testing.T) {
	clusterURI := "the_cluster_uri_in_ocm"
	accountID := "a_stringified_account_id"
	orgID := "a_stringified_org_id"
	clusterID := "the_displayed_cluster_ID"
	notificationMsg := generateInstantNotificationMessage(&clusterURI, accountID, orgID, clusterID)

	assert.Empty(t, notificationMsg.Events, "the generated notification message should not have any events")

	ruleURI := "a_given_uri/{cluster_id}/{module}/{error_key}"
	ruleDescription := "a_given_rule_name"
	publishDate := "the_date_of_today"
	totalRisk := 1
	module := "a.module"
	errorKey := "an_error_key"

	notificationPayloadURL := generateNotificationPayloadURL(&ruleURI, clusterID, types.ModuleName(module), types.ErrorKey(errorKey))
	appendEventToNotificationMessage(notificationPayloadURL, &notificationMsg, ruleDescription, totalRisk, publishDate)
	assert.Equal(t, len(notificationMsg.Events), 1, "the notification message should have 1 event")
	assert.Equal(t, notificationMsg.Events[0].Metadata, types.EventMetadata{}, "All notification messages should have empty metadata")

	payload := Payload{}
	err := json.Unmarshal([]byte(notificationMsg.Events[0].Payload), &payload)
	assert.Nil(t, err, fmt.Sprintf("Unexpected behavior: Cannot unmarshall notification payload %q", notificationMsg.Events[0].Payload))
	assert.Equal(t, payload.RuleURL, "a_given_uri/the_displayed_cluster_ID/a|module/an_error_key", fmt.Sprintf("The rule URL %s is not correct", payload.RuleURL))

	appendEventToNotificationMessage(notificationPayloadURL, &notificationMsg, ruleDescription, totalRisk, publishDate)
	assert.Equal(t, len(notificationMsg.Events), 2, "the notification message should have 2 events")
	assert.Equal(t, notificationMsg.Events[1].Metadata, types.EventMetadata{}, "All notification messages should have empty metadata")
}

// ---------------------------------------------------------------------------------------
func TestUpdateDigestNotificationCounters(t *testing.T) {
	digest := types.Digest{}
	expectedDigest := types.Digest{}
	updateDigestNotificationCounters(&digest, 1)
	assert.Equal(t, expectedDigest, digest, "No field in digest should be incremented if totalRisk == 1")
	updateDigestNotificationCounters(&digest, 2)
	assert.Equal(t, expectedDigest, digest, "No field in digest should be incremented if totalRisk == 2")
	updateDigestNotificationCounters(&digest, 3)
	expectedDigest.ImportantNotifications++
	assert.Equal(t, expectedDigest, digest, "ImportantNotifications field should be incremented if totalRisk == 3")
	updateDigestNotificationCounters(&digest, 4)
	expectedDigest.CriticalNotifications++
	assert.Equal(t, expectedDigest, digest, "CriticalNotifications field should be incremented if totalRisk == 4")
	updateDigestNotificationCounters(&digest, 5)
	assert.Equal(t, expectedDigest, digest, "No field in digest should be incremented if 3 < totalRisk or totalRisk > 4")
}

func TestGetNotificationDigestForCurrentAccount(t *testing.T) {
	emptyDigest := types.Digest{}
	existingDigest := types.Digest{
		ClustersAffected:       23,
		CriticalNotifications:  2,
		ImportantNotifications: 4,
		Recommendations:        8,
		Incidents:              0,
	}

	notificationsByAccount := map[types.AccountNumber]types.Digest{
		12345: existingDigest,
	}
	assert.Equal(t, emptyDigest, getNotificationDigestForCurrentAccount(notificationsByAccount, 100))
	assert.Equal(t, existingDigest, getNotificationDigestForCurrentAccount(notificationsByAccount, 12345))
}

func TestGenerateWeeklyDigestNotificationMessage(t *testing.T) {
	advisorURI := "the_uri_to_advisor_tab_in_ocm"
	accountID := "a_stringified_account_id"
	digest := types.Digest{}

	notificationMsg := generateWeeklyNotificationMessage(&advisorURI, accountID, digest)

	assert.NotEmpty(t, notificationMsg, "the generated notification message is empty")
	assert.NotEmpty(t, notificationMsg.Events, "the generated notification message should have 1 event with digest's content")
	assert.Equal(t, types.WeeklyDigest.ToString(), notificationMsg.EventType, "the generated notification message should be for weekly digest")
	assert.Equal(t, notificationBundleName, notificationMsg.Bundle, "Generated notifications should indicate 'openshift' as bundle")
	assert.Equal(t, notificationApplicationName, notificationMsg.Application, "Generated notifications should indicate 'openshift' as application name")
	assert.Equal(t, accountID, notificationMsg.AccountID, "Generated notifications does not have expected account ID")
	assert.Equal(t, "{\"advisor_url\":\"the_uri_to_advisor_tab_in_ocm\"}", notificationMsg.Context, "Notification context is different from expected.")

	assert.Equal(t, 1, len(notificationMsg.Events)) // Only one event, always, with the digest's content
	expectedEvent := types.Event{
		Metadata: types.EventMetadata{},
		Payload:  "{\"total_clusters\":\"0\",\"total_critical\":\"0\",\"total_important\":\"0\",\"total_incidents\":\"0\",\"total_recommendations\":\"0\"}",
	}
	assert.Equal(t, expectedEvent, notificationMsg.Events[0])
}

// ---------------------------------------------------------------------------------------
func TestShowVersion(t *testing.T) {
	assert.Contains(t, captureStdout(showVersion), versionMessage, "showVersion function is not displaying the expected content")
}

func TestShowAuthors(t *testing.T) {
	assert.Contains(t, captureStdout(showAuthors), authorsMessage, "showAuthors function is not displaying the expected content")
}

func TestShowConfiguration(t *testing.T) {
	brokerAddr := "localhost:29092"
	brokerTopic := "test_topic"
	db := "db"
	driver := "test_driver"
	advisorURL := "an uri"
	clustersURI := "a {cluster} details uri"
	ruleURI := "a {rule} details uri"
	metricsJob := "ccx"
	metricsNamespace := "notification"
	metricsGateway := "localhost:12345"

	config := conf.ConfigStruct{
		Logging: conf.LoggingConfiguration{
			Debug:    true,
			LogLevel: "info",
		},
		Storage: conf.StorageConfiguration{
			Driver:   driver,
			PGDBName: db,
		},
		Kafka: conf.KafkaConfiguration{
			Address:     brokerAddr,
			Topic:       brokerTopic,
			Timeout:     0,
			Enabled:     true,
			EventFilter: "totalRisk >= totalRiskThreshold",
		},
		Dependencies: conf.DependenciesConfiguration{},
		Notifications: conf.NotificationsConfiguration{
			InsightsAdvisorURL: advisorURL,
			ClusterDetailsURI:  clustersURI,
			RuleDetailsURI:     ruleURI,
		},
		Metrics: conf.MetricsConfiguration{
			Job:        metricsJob,
			Namespace:  metricsNamespace,
			GatewayURL: metricsGateway,
			Retries:    4,
			RetryAfter: 1,
		},
		Cleaner: conf.CleanerConfiguration{
			MaxAge: "70 days",
		},
	}

	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.InfoLevel)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	showConfiguration(config)
	output := buf.String()

	// Assert that at least one item of each struct is shown
	assert.Contains(t, output, brokerAddr)
	assert.Contains(t, output, clustersURI)
	assert.Contains(t, output, db)
	assert.Contains(t, output, driver)
	assert.Contains(t, output, "\"Pretty colored debug logging\":true")
	assert.Contains(t, output, metricsGateway)
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
		assert.Equal(t, item.expectedRisk, calculateTotalRisk(item.impact, item.likelihood))
	}
}

func TestModuleNameToRuleNameValidRuleName(t *testing.T) {
	moduleName := types.ModuleName("ccx_rules_ocp.external.rules.cluster_wide_proxy_auth_check.report")
	ruleName := types.RuleName("cluster_wide_proxy_auth_check")
	assert.Equal(t, ruleName, moduleToRuleName(moduleName))
}

// ---------------------------------------------------------------------------------------
func TestSetupNotificationProducerInvalidBrokerConf(t *testing.T) {
	if os.Getenv("SETUP_PRODUCER") == "1" {
		testConfig := conf.ConfigStruct{
			Kafka: conf.KafkaConfiguration{
				Address:     "invalid_address",
				Topic:       "",
				Timeout:     0,
				Enabled:     true,
				EventFilter: "totalRisk >= totalRiskThreshold",
			},
		}

		setupKafkaProducer(testConfig)
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestSetupNotificationProducerInvalidBrokerConf")
	cmd.Env = append(os.Environ(), "SETUP_PRODUCER=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && e.ExitCode() != ExitStatusKafkaBrokerError {
		t.Fatalf(
			"Should exit with status ExitStatusKafkaBrokerError(%d). Got status %d",
			ExitStatusKafkaBrokerError,
			e.ExitCode())
	}
}

func TestSetupNotificationProducerValidBrokerConf(t *testing.T) {
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
			Address: mockBroker.Addr(),
			Topic:   brokerCfg.Topic,
			Timeout: brokerCfg.Timeout,
			Enabled: brokerCfg.Enabled,
		},
	}

	kafkaProducer := kafka.Producer{
		Configuration: conf.GetKafkaBrokerConfiguration(testConfig),
		Producer:      nil,
	}

	setupKafkaProducer(testConfig)

	prod := kafkaNotifier.(*kafka.Producer)

	assert.Equal(t, kafkaProducer.Configuration.Address, prod.Configuration.Address)
	assert.Equal(t, kafkaProducer.Configuration.Topic, prod.Configuration.Topic)
	assert.Equal(t, kafkaProducer.Configuration.Timeout, prod.Configuration.Timeout)
	assert.Nil(t, kafkaProducer.Producer, "Unexpected behavior: Producer was not set up correctly")
	assert.NotNil(t, prod.Producer, "Unexpected behavior: Producer was not set up correctly")

	err := kafkaNotifier.Close()
	assert.Nil(t, err, "Unexpected behavior: Producer was not closed successfully")
}

func TestStartDifferNoDestination(t *testing.T) {
	if os.Getenv("SETUP_PRODUCER") == "1" {
		testConfig := conf.ConfigStruct{}

		startDiffer(testConfig, nil, false)
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestStartDifferNoDestination")
	cmd.Env = append(os.Environ(), "SETUP_PRODUCER=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && e.ExitCode() != ExitStatusConfiguration {
		t.Fatalf(
			"Should exit with status ExitStatusConfiguration(%d). Got status %d",
			ExitStatusConfiguration,
			e.ExitCode())
	}
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
					Impact: 3,
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

	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.InfoLevel)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	processClusters(conf.ConfigStruct{Kafka: conf.KafkaConfiguration{Enabled: true}}, ruleContent, &storage, clusters)

	executionLog := buf.String()
	assert.Contains(t, executionLog, "no rows in result set", "No report should be retrieved for the first cluster")
	assert.Contains(t, executionLog, "No new issues to notify for cluster second_cluster", "the processReportsByCluster loop did not continue as extpected")
	assert.Contains(t, executionLog, "Number of entries not processed: 1", "the first cluster should have been skipped")

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
					Impact: 3,
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

	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.InfoLevel)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	processClusters(conf.ConfigStruct{Kafka: conf.KafkaConfiguration{Enabled: true}}, ruleContent, &storage, clusters)

	executionLog := buf.String()
	assert.Contains(t, executionLog, "cannot unmarshal object into Go struct field Report.reports of type []types.ReportItem", "The string retrieved is not a list of reports. It should not deserialize correctly")
	assert.Contains(t, executionLog, "No new issues to notify for cluster second_cluster", "the processReportsByCluster loop did not continue as extpected")
	assert.Contains(t, executionLog, "Number of entries not processed: 1", "the first cluster should have been skipped")

	zerolog.SetGlobalLevel(zerolog.WarnLevel)
}

func TestProcessClustersInstantNotifsAndTotalRiskInferiorToThreshold(t *testing.T) {
	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.InfoLevel)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	errorKeys := map[string]utypes.RuleErrorKeyContent{
		"RULE_1": {
			Metadata: utypes.ErrorKeyMetadata{
				Description: "rule 1 error key description",
				Impact: utypes.Impact{
					Name:   "impact_1",
					Impact: 3,
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

	processClusters(conf.ConfigStruct{Kafka: conf.KafkaConfiguration{Enabled: true}}, ruleContent, &storage, clusters)

	executionLog := buf.String()
	assert.Contains(t, executionLog, "No new issues to notify for cluster first_cluster", "processClusters shouldn't generate any notification for 'first_cluster' with given data")
	assert.Contains(t, executionLog, "No new issues to notify for cluster second_cluster", "processClusters shouldn't generate any notification for 'second_cluster' with given data")

	zerolog.SetGlobalLevel(zerolog.WarnLevel)
}

func TestProcessClustersInstantNotifsAndTotalRiskImportant(t *testing.T) {
	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.InfoLevel)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

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

	originalNotifier := kafkaNotifier
	kafkaNotifier = &producerMock

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

	processClusters(conf.ConfigStruct{Kafka: conf.KafkaConfiguration{Enabled: true}}, ruleContent, &storage, clusters)

	executionLog := buf.String()
	assert.Contains(t, executionLog, "Report with high impact detected", "processClusters should create a notification for 'first_cluster' with given data")
	assert.Contains(t, executionLog, "{\"level\":\"info\",\"cluster\":\"first_cluster\",\"number of events\":1,\"message\":\"Producing instant notification\"}", "processClusters should generate one notification for 'first_cluster' with given data")
	assert.Contains(t, executionLog, "{\"level\":\"info\",\"cluster\":\"second_cluster\",\"number of events\":1,\"message\":\"Producing instant notification\"}", "processClusters should generate one notification for 'first_cluster' with given data")

	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	kafkaNotifier = originalNotifier
}

func TestProcessClustersInstantNotifsAndTotalRiskCritical(t *testing.T) {
	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.InfoLevel)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

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

	originalNotifier := kafkaNotifier
	kafkaNotifier = &producerMock

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

	processClusters(conf.ConfigStruct{Kafka: conf.KafkaConfiguration{Enabled: true}}, ruleContent, &storage, clusters)

	executionLog := buf.String()
	assert.Contains(t, executionLog, "{\"level\":\"warn\",\"type\":\"rule\",\"rule\":\"rule_1\",\"error key\":\"RULE_1\",\"likelihood\":4,\"impact\":4,\"totalRisk\":4,\"message\":\"Report with high impact detected\"}\n")
	assert.Contains(t, executionLog, "{\"level\":\"info\",\"cluster\":\"first_cluster\",\"number of events\":1,\"message\":\"Producing instant notification\"}", "processClusters should generate one notification for 'first_cluster' with given data")
	assert.Contains(t, executionLog, "{\"level\":\"info\",\"cluster\":\"second_cluster\",\"number of events\":1,\"message\":\"Producing instant notification\"}", "processClusters should generate one notification for 'first_cluster' with given data")

	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	kafkaNotifier = originalNotifier
}

func TestProcessClustersAllIssuesAlreadyNotifiedCooldownNotPassed(t *testing.T) {
	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.InfoLevel)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

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

	previouslyReported[types.NotificationBackendTarget][types.ClusterOrgKey{OrgID: types.OrgID(1), ClusterName: "first_cluster"}] = types.NotificationRecord{
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

	previouslyReported[types.NotificationBackendTarget][types.ClusterOrgKey{OrgID: types.OrgID(2), ClusterName: "second_cluster"}] = types.NotificationRecord{
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
	processClusters(conf.ConfigStruct{Kafka: conf.KafkaConfiguration{Enabled: true}}, ruleContent, &storage, clusters)

	executionLog := buf.String()
	assert.Contains(t, executionLog, "{\"level\":\"info\",\"message\":\"No new issues to notify for cluster first_cluster\"}\n", "Notification already sent for first_cluster's report, but corresponding log not found.")
	assert.Contains(t, executionLog, "{\"level\":\"info\",\"message\":\"No new issues to notify for cluster second_cluster\"}\n", "Notification already sent for second_cluster's report, but corresponding log not found.")

	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	previouslyReported = types.NotifiedRecordsPerClusterByTarget{
		types.NotificationBackendTarget: types.NotifiedRecordsPerCluster{},
		types.ServiceLogTarget:          types.NotifiedRecordsPerCluster{},
	}
}

func TestProcessClustersNewIssuesNotPreviouslyNotified(t *testing.T) {
	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.InfoLevel)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

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

	originalNotifier := kafkaNotifier
	kafkaNotifier = &producerMock

	previouslyReported[types.NotificationBackendTarget][types.ClusterOrgKey{OrgID: types.OrgID(3), ClusterName: "a cluster"}] = types.NotificationRecord{
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

	processClusters(conf.ConfigStruct{Kafka: conf.KafkaConfiguration{Enabled: true}}, ruleContent, &storage, clusters)

	executionLog := buf.String()
	assert.Contains(t, executionLog, "{\"level\":\"warn\",\"type\":\"rule\",\"rule\":\"rule_1\",\"error key\":\"RULE_1\",\"likelihood\":4,\"impact\":4,\"totalRisk\":4,\"message\":\"Report with high impact detected\"}\n")
	assert.Contains(t, executionLog, "{\"level\":\"info\",\"cluster\":\"first_cluster\",\"number of events\":1,\"message\":\"Producing instant notification\"}", "processClusters should generate one notification for 'first_cluster' with given data")
	assert.Contains(t, executionLog, "{\"level\":\"info\",\"cluster\":\"second_cluster\",\"number of events\":1,\"message\":\"Producing instant notification\"}", "processClusters should generate one notification for 'second_cluster' with given data")

	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	kafkaNotifier = originalNotifier
	previouslyReported = types.NotifiedRecordsPerClusterByTarget{
		types.NotificationBackendTarget: types.NotifiedRecordsPerCluster{},
		types.ServiceLogTarget:          types.NotifiedRecordsPerCluster{},
	}
}

// ---------------------------------------------------------------------------------------
func TestProcessClustersWeeklyDigest(t *testing.T) {
	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.InfoLevel)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

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
					Impact: 3,
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
	storage.On("ReadReportForCluster",
		mock.AnythingOfType("types.OrgID"),
		mock.AnythingOfType("types.ClusterName")).Return(
		func(orgID types.OrgID, clusterName types.ClusterName) types.ClusterReport {
			return "{\"analysis_metadata\":{\"metadata\":\"some metadata\"},\"reports\":[{\"rule_id\":\"rule_1|RULE_1\",\"component\":\"ccx_rules_ocp.external.rules.rule_1.report\",\"type\":\"rule\",\"key\":\"RULE_1\",\"details\":\"some details\"}]}"
		},
		func(orgID types.OrgID, clusterName types.ClusterName) types.Timestamp {
			return types.Timestamp(testTimestamp)
		},
		func(orgID types.OrgID, clusterName types.ClusterName) error {
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

	originalNotifier := kafkaNotifier
	kafkaNotifier = &producerMock

	notificationType = types.WeeklyDigest
	processClusters(conf.ConfigStruct{Kafka: conf.KafkaConfiguration{Enabled: true}}, ruleContent, &storage, clusters)

	print(buf.String())

	assert.Contains(t, buf.String(), "{\"level\":\"info\",\"message\":\"Creating notification digest for account 1\"}")
	assert.Contains(t, buf.String(), "{\"level\":\"info\",\"message\":\"Creating notification digest for account 2\"}")
	assert.Contains(t, buf.String(), "{\"level\":\"info\",\"account number\":1,\"total recommendations\":1,\"clusters affected\":1,\"critical notifications\":0,\"important notifications\":0,\"message\":\"Producing weekly notification for \"}")
	assert.Contains(t, buf.String(), "{\"level\":\"info\",\"account number\":2,\"total recommendations\":1,\"clusters affected\":1,\"critical notifications\":0,\"important notifications\":0,\"message\":\"Producing weekly notification for \"}")

	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	kafkaNotifier = originalNotifier
}

// ---------------------------------------------------------------------------------------
func TestProduceEntriesToServiceLog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rendered_reports" {
			t.Errorf("Expected to request '/render_reports', got: %s", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type: application/json header, got: %s", r.Header.Get("Accept"))
		}
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"clusters":["84f7eedc-0dd8-49cd-9d4d-f6646df3a5bc"],"reports":{"84f7eedc-0dd8-49cd-9d4d-f6646df3a5bc":[{"rule_id":"node_installer_degraded","error_key":"ek1","resolution":"rule 1 resolution","reason":"This reason is more than 255 characters long. This reason is more than 255 characters long. This reason is more than 255 characters long. This reason is more than 255 characters long. This reason is more than 255 characters long. This reason is more than 255 characters long.","description":"This reason is more than 255 characters long. This summary is more than 255 characters long. This summary is more than 255 characters long. This summary is more than 255 characters long. This summary is more than 255 characters long. This summary is more than 255 characters long."},{"rule_id":"rule2","error_key":"ek2","resolution":"rule 2 resolution","reason":"rule 2 reason","description":"rule 2 error key description"}, {"rule_id":"rule3","error_key":"ek3","resolution":"rule 3 resolution","reason":"rule 3 reason","description":"rule 3 error key description"}]}}`))
		if err != nil {
			log.Fatal().Msg(err.Error())
		}
	}))
	defer server.Close()

	config := conf.ConfigStruct{
		ServiceLog: conf.ServiceLogConfiguration{
			Enabled:            true,
			TotalRiskThreshold: 1,
			EventFilter:        "totalRisk > totalRiskThreshold",
		},
		Dependencies: conf.DependenciesConfiguration{
			TemplateRendererServer:   server.URL,
			TemplateRendererEndpoint: "/rendered_reports",
			TemplateRendererURL:      server.URL + "/rendered_reports",
		},
	}
	serviceLogEventThresholds.TotalRisk = 1
	serviceLogEventFilter = "totalRisk > totalRiskThreshold"

	ruleContent := types.RulesMap{
		"node_installer_degraded": testdata.RuleContent1,
		"rule2":                   testdata.RuleContent2,
		"rule3":                   testdata.RuleContent3,
	}

	cluster := types.ClusterEntry{
		OrgID:         1,
		AccountNumber: 1,
		ClusterName:   types.ClusterName(testdata.ClusterName),
		KafkaOffset:   0,
		UpdatedAt:     types.Timestamp(testTimestamp),
	}

	var deserialized types.Report
	err := json.Unmarshal([]byte(testdata.ClusterReport3Rules), &deserialized)
	if err != nil {
		log.Error().Msg(err.Error())
		return
	}
	reports := deserialized.Reports

	producerMock := mocks.Producer{}
	producerMock.On("ProduceMessage", mock.AnythingOfType("types.ProducerMessage")).Return(
		func(msg types.ProducerMessage) int32 {
			return 0
		},
		func(msg types.ProducerMessage) int64 {
			return 0
		},
		func(msg types.ProducerMessage) error {
			return nil
		},
	)

	originalNotifier := serviceLogNotifier
	serviceLogNotifier = &producerMock

	produceEntriesToServiceLog(config, cluster, ruleContent, reports)
	producerMock.AssertNumberOfCalls(t, "ProduceMessage", 2)

	serviceLogNotifier = originalNotifier
}
