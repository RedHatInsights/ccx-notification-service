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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	utypes "github.com/RedHatInsights/insights-operator-utils/types"

	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/producer"
	"github.com/RedHatInsights/ccx-notification-service/tests/mocks"
	"github.com/RedHatInsights/ccx-notification-service/types"
	"github.com/Shopify/sarama"
	samara_mocks "github.com/Shopify/sarama/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	brokerCfg = conf.KafkaConfiguration{
		Address: "localhost:9092",
		Topic:   "platform.notifications.ingress",
		Timeout: time.Duration(30*10 ^ 9),
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

// Test the checkArgs function when flag for --show-version is set
func TestArgsParsingShowVersion(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=TestArgsParsingShowVersion")
	if os.Getenv("SHOW_VERSION_FLAG") == "1" {
		args := types.CliFlags{
			ShowVersion: true,
		}
		checkArgs(&args)
	}
	cmd.Env = append(os.Environ(), "SHOW_VERSION_FLAG=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		t.Fatalf("checkArgs exited with exit status %d, wanted exit status 0", e.ExitCode())
	}
}

// Test the checkArgs function when flag for --show-authors is set
func TestArgsParsingShowAuthors(t *testing.T) {
	if os.Getenv("SHOW_AUTHORS_FLAG") == "1" {
		args := types.CliFlags{
			ShowAuthors: true,
		}
		checkArgs(&args)
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestArgsParsingShowAuthors")
	cmd.Env = append(os.Environ(), "SHOW_AUTHORS_FLAG=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		t.Fatalf("checkArgs exited with exit status %d, wanted exit status 0", e.ExitCode())
	}
}

// Test the checkArgs function when no flag is set
func TestArgsParsingNoFlags(t *testing.T) {
	if os.Getenv("EXEC_NO_FLAG") == "1" {
		args := types.CliFlags{}
		checkArgs(&args)
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestArgsParsingNoFlags")
	cmd.Env = append(os.Environ(), "EXEC_NO_FLAG=1")
	err := cmd.Run()
	assert.NotNil(t, err)
	if exitError, ok := err.(*exec.ExitError); ok {
		assert.Equal(t, ExitStatusConfiguration, exitError.ExitCode())
		return
	}
	t.Fatalf("checkArgs didn't behave properly. Wanted exit status %d but got error\n %v", ExitStatusConfiguration, err)
}

// Test the checkArgs function when --instant-reports flag is set
func TestArgsParsingInstantReportsFlags(t *testing.T) {
	args := types.CliFlags{
		InstantReports: true,
	}
	checkArgs(&args)
	assert.Equal(t, notificationType, types.InstantNotif)
}

// Test the checkArgs function when --weekly-reports flag is set
func TestArgsParsingWeeklyReportsFlags(t *testing.T) {
	args := types.CliFlags{
		WeeklyReports: true,
	}
	checkArgs(&args)
	assert.Equal(t, notificationType, types.WeeklyDigest)
}

//---------------------------------------------------------------------------------------
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

//---------------------------------------------------------------------------------------
func TestGenerateInstantReportNotificationMessage(t *testing.T) {
	clusterURI := "the_cluster_uri_in_ocm_for_{cluster_id}"
	accountID := "a_stringified_account_id"
	clusterID := "the_displayed_cluster_ID"

	notificationMsg := generateInstantNotificationMessage(&clusterURI, accountID, clusterID)

	assert.NotEmpty(t, notificationMsg, "the generated notification message is empty")
	assert.Empty(t, notificationMsg.Events, "the generated notification message should not have any events")
	assert.Equal(t, types.InstantNotif.String(), notificationMsg.EventType, "the generated notification message should be for instant notifications")
	assert.Equal(t, "{\"display_name\":\"the_displayed_cluster_ID\",\"host_url\":\"the_cluster_uri_in_ocm_for_the_displayed_cluster_ID\"}", notificationMsg.Context, "Notification context is different from expected.")
	assert.Equal(t, notificationBundleName, notificationMsg.Bundle, "Generated notifications should indicate 'openshift' as bundle")
	assert.Equal(t, notificationApplicationName, notificationMsg.Application, "Generated notifications should indicate 'openshift' as application name")
	assert.Equal(t, accountID, notificationMsg.AccountID, "Generated notifications does not have expected account ID")
}

func TestAppendEventsToExistingInstantReportNotificationMsg(t *testing.T) {
	clusterURI := "the_cluster_uri_in_ocm"
	accountID := "a_stringified_account_id"
	clusterID := "the_displayed_cluster_ID"
	notificationMsg := generateInstantNotificationMessage(&clusterURI, accountID, clusterID)

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

//---------------------------------------------------------------------------------------
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
	assert.Equal(t, types.WeeklyDigest.String(), notificationMsg.EventType, "the generated notification message should be for weekly digest")
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

//---------------------------------------------------------------------------------------
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
			Address: brokerAddr,
			Topic:   brokerTopic,
			Timeout: 0,
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

//---------------------------------------------------------------------------------------
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

//---------------------------------------------------------------------------------------
func TestSetupNotificationProducerInvalidBrokerConf(t *testing.T) {
	if os.Getenv("SETUP_PRODUCER") == "1" {
		brokerConfig := conf.KafkaConfiguration{
			Address: "invalid_address",
			Topic:   "",
			Timeout: 0,
		}

		setupNotificationProducer(brokerConfig)
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

	testConfig := conf.KafkaConfiguration{
		Address: mockBroker.Addr(),
		Topic:   brokerCfg.Topic,
		Timeout: brokerCfg.Timeout,
	}

	kafkaProducer := producer.KafkaProducer{
		Configuration: testConfig,
		Producer:      nil,
	}

	setupNotificationProducer(testConfig)

	assert.Equal(t, kafkaProducer.Configuration.Address, notifier.Configuration.Address)
	assert.Equal(t, kafkaProducer.Configuration.Topic, notifier.Configuration.Topic)
	assert.Equal(t, kafkaProducer.Configuration.Timeout, notifier.Configuration.Timeout)
	assert.Nil(t, kafkaProducer.Producer, "Unexpected behavior: Producer was not set up correctly")
	assert.NotNil(t, notifier.Producer, "Unexpected behavior: Producer was not set up correctly")

	err := notifier.Close()
	assert.Nil(t, err, "Unexpected behavior: Producer was not closed successfully")
}

//---------------------------------------------------------------------------------------
func TestProcessClustersInstantNotifsAndTotalRiskInferiorToThreshold(t *testing.T) {
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

	config := conf.ConfigStruct{
		Kafka: conf.KafkaConfiguration{
			Address: mockBroker.Addr(),
			Topic:   brokerCfg.Topic,
			Timeout: brokerCfg.Timeout,
		},
		Notifications: conf.NotificationsConfiguration{
			InsightsAdvisorURL: "an uri",
			ClusterDetailsURI:  "a {cluster} details uri",
			RuleDetailsURI:     "a {rule} details uri",
		},
	}

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
	storage.On("ReadLastNNotificationRecords", mock.AnythingOfType("types.ClusterEntry"), mock.AnythingOfType("int")).Return(
		func(clusterEntry types.ClusterEntry, numberOfRecords int) []types.NotificationRecord {
			return []types.NotificationRecord{
				{
					OrgID:              1,
					AccountNumber:      2,
					ClusterName:        "a cluster",
					UpdatedAt:          types.Timestamp(testTimestamp),
					NotificationTypeID: 0,
					StateID:            0,
					Report:             "{\"analysis_metadata\":{\"metadata\":\"some metadata\"},\"reports\":[{\"rule_id\":\"rule_4|RULE_4\",\"component\":\"ccx_rules_ocp.external.rules.rule_1.report\",\"type\":\"rule\",\"key\":\"RULE_4\",\"details\":\"some details\"},{\"rule_id\":\"rule_4|RULE_4\",\"component\":\"ccx_rules_ocp.external.rules.rule_2.report\",\"type\":\"rule\",\"key\":\"RULE_2\",\"details\":\"some details\"},{\"rule_id\":\"rule_5|RULE_5\",\"component\":\"ccx_rules_ocp.external.rules.rule_5.report\",\"type\":\"rule\",\"key\":\"RULE_3\",\"details\":\"some details\"}]}",
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
		mock.AnythingOfType("string")).Return(
		func(clusterEntry types.ClusterEntry, notificationTypeID types.NotificationTypeID, stateID types.StateID, report types.ClusterReport, notifiedAt types.Timestamp, errorLog string) error {
			return nil
		},
	)

	producerMock := mocks.Producer{}
	producerMock.On("New", mock.AnythingOfType("conf.KafkaConfiguration")).Return(
		func(brokerCfg conf.KafkaConfiguration) *producer.KafkaProducer {
			mockProducer := samara_mocks.NewSyncProducer(t, nil)

			kp := producer.KafkaProducer{
				Configuration: brokerCfg,
				Producer:      mockProducer,
			}
			return &kp
		},
		func(brokerCfg conf.KafkaConfiguration) error {
			return nil
		},
	)

	originalNotifier := notifier
	notifier, _ = producerMock.New(config.Kafka)

	notificationType = types.InstantNotif
	processClusters(ruleContent, &storage, clusters)

	assert.Contains(t, buf.String(), "No new issues to notify for cluster first_cluster", "processClusters shouldn't generate any notification for 'first_cluster' with given data")
	assert.Contains(t, buf.String(), "No new issues to notify for cluster second_cluster", "processClusters shouldn't generate any notification for 'second_cluster' with given data")

	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	notifier = originalNotifier
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

	config := conf.ConfigStruct{
		Kafka: conf.KafkaConfiguration{
			Address: mockBroker.Addr(),
			Topic:   brokerCfg.Topic,
			Timeout: 0,
		},
		Notifications: conf.NotificationsConfiguration{
			InsightsAdvisorURL: "an uri",
			ClusterDetailsURI:  "a {cluster} details uri",
			RuleDetailsURI:     "a {rule} details uri",
		},
	}

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
	storage.On("ReadLastNNotificationRecords", mock.AnythingOfType("types.ClusterEntry"), mock.AnythingOfType("int")).Return(
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
		mock.AnythingOfType("string")).Return(
		func(clusterEntry types.ClusterEntry, notificationTypeID types.NotificationTypeID, stateID types.StateID, report types.ClusterReport, notifiedAt types.Timestamp, errorLog string) error {
			return nil
		},
	)

	producerMock := mocks.Producer{}
	producerMock.On("New", mock.AnythingOfType("conf.KafkaConfiguration")).Return(
		func(brokerCfg conf.KafkaConfiguration) *producer.KafkaProducer {
			mockProducer := samara_mocks.NewSyncProducer(t, nil)
			mockProducer.ExpectSendMessageAndSucceed()
			mockProducer.ExpectSendMessageAndSucceed()

			kp := producer.KafkaProducer{
				Configuration: brokerCfg,
				Producer:      mockProducer,
			}
			return &kp
		},
		func(brokerCfg conf.KafkaConfiguration) error {
			return nil
		},
	)
	producerMock.On("ProduceMessage", mock.AnythingOfType("types.NotificationMessage")).Return(
		func(msg types.NotificationMessage) int32 {
			testPartitionID++
			return int32(testPartitionID)
		},
		func(msg types.NotificationMessage) int64 {
			testOffset++
			return int64(testOffset)
		},
		func(msg types.NotificationMessage) error {
			return nil
		},
	)

	originalNotifier := notifier
	notifier, _ = producerMock.New(config.Kafka)

	notificationType = types.InstantNotif

	processClusters(ruleContent, &storage, clusters)

	assert.Contains(t, buf.String(), "Report with high impact detected", "processClusters should create a notification for 'first_cluster' with given data")
	assert.Contains(t, buf.String(), "Producing instant notification for cluster first_cluster with 1 events", "processClusters should generate one notification for 'first_cluster' with given data")
	assert.Contains(t, buf.String(), "Producing instant notification for cluster second_cluster with 1 events", "processClusters should generate one notification for 'first_cluster' with given data")
	assert.Contains(t, buf.String(), "message sent to partition")

	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	notifier = originalNotifier
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

	config := conf.ConfigStruct{
		Kafka: conf.KafkaConfiguration{
			Address: mockBroker.Addr(),
			Topic:   brokerCfg.Topic,
			Timeout: 0,
		},
		Notifications: conf.NotificationsConfiguration{
			InsightsAdvisorURL: "an uri",
			ClusterDetailsURI:  "a {cluster} details uri",
			RuleDetailsURI:     "a {rule} details uri",
		},
	}

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
	storage.On("ReadLastNNotificationRecords", mock.AnythingOfType("types.ClusterEntry"), mock.AnythingOfType("int")).Return(
		func(clusterEntry types.ClusterEntry, numberOfRecords int) []types.NotificationRecord {
			return []types.NotificationRecord{
				{
					OrgID:              3,
					AccountNumber:      4,
					ClusterName:        "a cluster",
					UpdatedAt:          types.Timestamp(testTimestamp),
					NotificationTypeID: 0,
					StateID:            0,
					Report:             "{\"analysis_metadata\":{\"metadata\":\"some metadata\"},\"reports\":[{\"rule_id\":\"rule_4|RULE_4\",\"component\":\"ccx_rules_ocp.external.rules.rule_4.report\",\"type\":\"rule\",\"key\":\"RULE_4\",\"details\":\"some details\"}]}",
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
		mock.AnythingOfType("string")).Return(
		func(clusterEntry types.ClusterEntry, notificationTypeID types.NotificationTypeID, stateID types.StateID, report types.ClusterReport, notifiedAt types.Timestamp, errorLog string) error {
			return nil
		},
	)

	producerMock := mocks.Producer{}
	producerMock.On("New", mock.AnythingOfType("conf.KafkaConfiguration")).Return(
		func(brokerCfg conf.KafkaConfiguration) *producer.KafkaProducer {
			mockProducer := samara_mocks.NewSyncProducer(t, nil)
			mockProducer.ExpectSendMessageAndSucceed()
			mockProducer.ExpectSendMessageAndSucceed()

			kp := producer.KafkaProducer{
				Configuration: brokerCfg,
				Producer:      mockProducer,
			}
			return &kp
		},
		func(brokerCfg conf.KafkaConfiguration) error {
			return nil
		},
	)
	producerMock.On("ProduceMessage", mock.AnythingOfType("types.NotificationMessage")).Return(
		func(msg types.NotificationMessage) int32 {
			testPartitionID++
			return int32(testPartitionID)
		},
		func(msg types.NotificationMessage) int64 {
			testOffset++
			return int64(testOffset)
		},
		func(msg types.NotificationMessage) error {
			return nil
		},
	)

	originalNotifier := notifier
	notifier, _ = producerMock.New(config.Kafka)

	notificationType = types.InstantNotif

	processClusters(ruleContent, &storage, clusters)

	assert.Contains(t, buf.String(), "{\"level\":\"warn\",\"type\":\"rule\",\"rule\":\"rule_1\",\"error key\":\"RULE_1\",\"likelihood\":4,\"impact\":4,\"totalRisk\":4,\"message\":\"Report with high impact detected\"}\n")
	assert.Contains(t, buf.String(), "{\"level\":\"info\",\"message\":\"Old report does not contain the new issue\"}\n")
	assert.Contains(t, buf.String(), "Producing instant notification for cluster first_cluster with 1 events", "processClusters should generate one notification for 'first_cluster' with given data")
	assert.Contains(t, buf.String(), "Producing instant notification for cluster second_cluster with 1 events", "processClusters should generate one notification for 'first_cluster' with given data")
	assert.Contains(t, buf.String(), "message sent to partition")

	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	notifier = originalNotifier
}

func TestProcessClustersAllIssuesAlreadyNotified(t *testing.T) {
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
	storage.On("ReadLastNNotificationRecords", mock.AnythingOfType("types.ClusterEntry"), mock.AnythingOfType("int")).Return(
		func(clusterEntry types.ClusterEntry, numberOfRecords int) []types.NotificationRecord {
			// Return the same record, therefore notification message should not beproduced
			return []types.NotificationRecord{
				{
					OrgID:              3,
					AccountNumber:      4,
					ClusterName:        "a cluster",
					UpdatedAt:          types.Timestamp(testTimestamp),
					NotificationTypeID: 0,
					StateID:            0,
					Report:             "{\"analysis_metadata\":{\"metadata\":\"some metadata\"},\"reports\":[{\"rule_id\":\"rule_1|RULE_1\",\"component\":\"ccx_rules_ocp.external.rules.rule_1.report\",\"type\":\"rule\",\"key\":\"RULE_1\",\"details\":\"some details\"}]}",
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
		mock.AnythingOfType("string")).Return(
		func(clusterEntry types.ClusterEntry, notificationTypeID types.NotificationTypeID, stateID types.StateID, report types.ClusterReport, notifiedAt types.Timestamp, errorLog string) error {
			return nil
		},
	)

	notificationType = types.InstantNotif

	processClusters(ruleContent, &storage, clusters)

	assert.Contains(t, buf.String(), "{\"level\":\"info\",\"message\":\"No new issues to notify for cluster first_cluster\"}\n", "Notification already sent for first_cluster's report, but corresponding log not found.")
	assert.Contains(t, buf.String(), "{\"level\":\"info\",\"message\":\"No new issues to notify for cluster second_cluster\"}\n", "Notification already sent for second_cluster's report, but corresponding log not found.")

	zerolog.SetGlobalLevel(zerolog.WarnLevel)
}

func TestProcessClustersSomeIssuesAlreadyReported(t *testing.T) {
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

	config := conf.ConfigStruct{
		Kafka: conf.KafkaConfiguration{
			Address: mockBroker.Addr(),
			Topic:   brokerCfg.Topic,
			Timeout: 0,
		},
		Notifications: conf.NotificationsConfiguration{
			InsightsAdvisorURL: "an uri",
			ClusterDetailsURI:  "a {cluster} details uri",
			RuleDetailsURI:     "a {rule} details uri",
		},
	}

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
	storage.On("ReadLastNNotificationRecords", mock.AnythingOfType("types.ClusterEntry"), mock.AnythingOfType("int")).Return(
		func(clusterEntry types.ClusterEntry, numberOfRecords int) []types.NotificationRecord {
			// We return three reports, one of them is in the
			// report, and the others are not -> issues should be
			// notified
			return []types.NotificationRecord{
				{
					OrgID:              3,
					AccountNumber:      4,
					ClusterName:        "a cluster",
					UpdatedAt:          types.Timestamp(testTimestamp),
					NotificationTypeID: 0,
					StateID:            0,
					Report:             "{\"analysis_metadata\":{\"metadata\":\"some metadata\"},\"reports\":[{\"rule_id\":\"rule_1|RULE_1\",\"component\":\"ccx_rules_ocp.external.rules.rule_1.report\",\"type\":\"rule\",\"key\":\"RULE_4\",\"details\":\"some details\"}]}",
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
		mock.AnythingOfType("string")).Return(
		func(clusterEntry types.ClusterEntry, notificationTypeID types.NotificationTypeID, stateID types.StateID, report types.ClusterReport, notifiedAt types.Timestamp, errorLog string) error {
			return nil
		},
	)

	producerMock := mocks.Producer{}
	producerMock.On("New", mock.AnythingOfType("conf.KafkaConfiguration")).Return(
		func(brokerCfg conf.KafkaConfiguration) *producer.KafkaProducer {
			mockProducer := samara_mocks.NewSyncProducer(t, nil)
			mockProducer.ExpectSendMessageAndSucceed()
			mockProducer.ExpectSendMessageAndSucceed()

			kp := producer.KafkaProducer{
				Configuration: brokerCfg,
				Producer:      mockProducer,
			}
			return &kp
		},
		func(brokerCfg conf.KafkaConfiguration) error {
			return nil
		},
	)
	producerMock.On("ProduceMessage", mock.AnythingOfType("types.NotificationMessage")).Return(
		func(msg types.NotificationMessage) int32 {
			testPartitionID++
			return int32(testPartitionID)
		},
		func(msg types.NotificationMessage) int64 {
			testOffset++
			return int64(testOffset)
		},
		func(msg types.NotificationMessage) error {
			return nil
		},
	)

	originalNotifier := notifier
	notifier, _ = producerMock.New(config.Kafka)

	notificationType = types.InstantNotif

	processClusters(ruleContent, &storage, clusters)

	assert.Contains(t, buf.String(), "{\"level\":\"warn\",\"type\":\"rule\",\"rule\":\"rule_1\",\"error key\":\"RULE_1\",\"likelihood\":4,\"impact\":4,\"totalRisk\":4,\"message\":\"Report with high impact detected\"}\n")
	assert.Contains(t, buf.String(), "{\"level\":\"info\",\"message\":\"Old report does not contain the new issue\"}\n")
	assert.Contains(t, buf.String(), "Producing instant notification for cluster first_cluster with 1 events", "processClusters should generate one notification for 'first_cluster' with given data")
	assert.Contains(t, buf.String(), "Producing instant notification for cluster second_cluster with 1 events", "processClusters should generate one notification for 'first_cluster' with given data")
	assert.Contains(t, buf.String(), "message sent to partition")

	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	notifier = originalNotifier
}

//---------------------------------------------------------------------------------------
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

	config := conf.ConfigStruct{
		Kafka: conf.KafkaConfiguration{
			Address: mockBroker.Addr(),
			Topic:   brokerCfg.Topic,
			Timeout: brokerCfg.Timeout,
		},
		Notifications: conf.NotificationsConfiguration{
			InsightsAdvisorURL: "an uri",
			ClusterDetailsURI:  "a {cluster} details uri",
			RuleDetailsURI:     "a {rule} details uri",
		},
	}

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
	producerMock.On("New", mock.AnythingOfType("conf.KafkaConfiguration")).Return(
		func(brokerCfg conf.KafkaConfiguration) *producer.KafkaProducer {
			mockProducer := samara_mocks.NewSyncProducer(t, nil)
			mockProducer.ExpectSendMessageAndSucceed()
			mockProducer.ExpectSendMessageAndSucceed()
			kp := producer.KafkaProducer{
				Configuration: brokerCfg,
				Producer:      mockProducer,
			}
			return &kp
		},
		func(brokerCfg conf.KafkaConfiguration) error {
			return nil
		},
	)

	originalNotifier := notifier
	notifier, _ = producerMock.New(config.Kafka)

	notificationType = types.WeeklyDigest
	processClusters(ruleContent, &storage, clusters)

	print(buf.String())

	assert.Contains(t, buf.String(), "{\"level\":\"info\",\"message\":\"Creating notification digest for account 1\"}")
	assert.Contains(t, buf.String(), "{\"level\":\"info\",\"message\":\"Creating notification digest for account 2\"}")
	assert.Contains(t, buf.String(), "{\"level\":\"info\",\"account number\":1,\"total recommendations\":1,\"clusters affected\":1,\"critical notifications\":0,\"important notifications\":0,\"message\":\"Producing weekly notification for \"}")
	assert.Contains(t, buf.String(), "{\"level\":\"info\",\"account number\":2,\"total recommendations\":1,\"clusters affected\":1,\"critical notifications\":0,\"important notifications\":0,\"message\":\"Producing weekly notification for \"}")
	assert.Contains(t, buf.String(), "message sent to partition")

	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	notifier = originalNotifier
}

func TestPushMetricsGatewayNotFailingWithRetries(t *testing.T) {
	var (
		pushes          int
		expectedPushes  = 1
		timeBetweenPush = 100 * time.Millisecond // 0.1s
		totalTime       = 500 * time.Millisecond // give enough time
	)

	testServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", `text/plain; charset=utf-8`)
			w.WriteHeader(http.StatusOK)
			pushes++
		}),
	)
	defer testServer.Close()

	_, cancel := context.WithTimeout(context.Background(), totalTime)

	metricsConf := conf.MetricsConfiguration{
		Job:        "ccx_notification_service",
		Namespace:  "ccx_notification_service",
		GatewayURL: testServer.URL,
		RetryAfter: timeBetweenPush,
		Retries:    10,
	}

	go pushMetrics(metricsConf)

	time.Sleep(totalTime)
	cancel()

	log.Info().Int("pushes", pushes).Msg("debug")

	assert.Equal(t, expectedPushes, pushes,
		fmt.Sprintf("expected exactly %d retries, but received %d", expectedPushes, pushes))
}

func TestPushMetricsGatewayNotFailingWithRetriesThenOk(t *testing.T) {
	var (
		pushes          int
		expectedPushes  = 6
		timeBetweenPush = 200 * time.Millisecond // 0.5s
		totalTime       = 2 * time.Second        // give enough time
	)

	testServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", `text/plain; charset=utf-8`)
			if pushes >= 5 {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusBadGateway)
			}
			pushes++
		}),
	)
	defer testServer.Close()

	_, cancel := context.WithTimeout(context.Background(), totalTime)

	metricsConf := conf.MetricsConfiguration{
		Job:        "ccx_notification_service",
		Namespace:  "ccx_notification_service",
		GatewayURL: testServer.URL,
		RetryAfter: timeBetweenPush,
		Retries:    10,
	}

	go pushMetrics(metricsConf)

	time.Sleep(totalTime)
	cancel()

	log.Info().Int("pushes", pushes).Msg("debug")

	assert.Equal(t, expectedPushes, pushes,
		fmt.Sprintf("expected exactly %d retries, but received %d", expectedPushes, pushes))
}

func TestPushMetricsGatewayFailing(t *testing.T) {
	if os.Getenv("GATEWAY_502_FAIL_ALL_RETRIES") == "1" {
		var (
			timeBetweenPush = 100 * time.Millisecond // 0.1s
			totalTime       = 1 * time.Second        // give enough time
		)

		testServer := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", `text/plain; charset=utf-8`)
				w.WriteHeader(http.StatusBadGateway)
			}),
		)
		defer testServer.Close()

		_, cancel := context.WithTimeout(context.Background(), totalTime)

		metricsConf := conf.MetricsConfiguration{
			Job:        "ccx_notification_service",
			Namespace:  "ccx_notification_service",
			GatewayURL: testServer.URL,
			RetryAfter: timeBetweenPush,
			Retries:    10,
		}

		go pushMetrics(metricsConf)

		time.Sleep(totalTime)
		cancel()
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestPushMetricsGatewayFailing")
	cmd.Env = append(os.Environ(), "GATEWAY_502_FAIL_ALL_RETRIES=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && e.ExitCode() != ExitStatusMetricsError {
		t.Fatalf(
			"Should exit with status ExitStatusMetricsError(%d). Got status %d",
			ExitStatusMetricsError,
			e.ExitCode())
	}
}
