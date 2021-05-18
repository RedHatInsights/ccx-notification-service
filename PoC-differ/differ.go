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
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/producer"
	"github.com/RedHatInsights/ccx-notification-service/types"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Configuration-related constants
const (
	configFileEnvVariableName = "NOTIFICATION_DIFFER_CONFIG_FILE"
	defaultConfigFileName     = "config"
)

// Exit codes
const (
	// ExitStatusOK means that the tool finished with success
	ExitStatusOK = iota
	// ExitStatusConfiguration is an error code related to program configuration
	ExitStatusConfiguration
	// ExitStatusError is a general error code
	ExitStatusError
	// ExitStatusStorageError is returned in case of any consumer-related error
	ExitStatusStorageError
	// ExitStatusFetchContentError is returned in case content cannot be fetch correctly
	ExitStatusFetchContentError
	// ExitStatusKafkaBrokerError is for kafka broker connection establishment errors
	ExitStatusKafkaBrokerError
	// ExitStatusKafkaProducerError is for kafka event production failures
	ExitStatusKafkaProducerError
	// ExitStatusKafkaConnectionNotClosedError is raised when connection cannot be closed
	ExitStatusKafkaConnectionNotClosedError
)

// Messages
const (
	versionMessage          = "Notification writer version 1.0"
	authorsMessage          = "Pavel Tisnovsky, Red Hat Inc."
	separator               = "------------------------------------------------------------"
	operationFailedMessage  = "Operation failed"
	clusterEntryMessage     = "cluster entry"
	organizationIDAttribute = "org id"
	AccountNumberAttribute  = "account number"
	clusterAttribute        = "cluster"
	totalRiskAttribute      = "totalRisk"
	errorStr                = "Error:"
	invalidJsonContent      = "The provided content cannot be encoded as JSON."
)

// Constants for notification message top level fields
const (
	notificationBundleName       = "openshift"
	notificationApplicationName  = "advisor"
	defaultNotificationAccountID = "6089719" //TODO: Replace this with getting account ID for affected cluster
)

// Constants for notification event expected fields
const (
	//PAYLOAD FIELDS
	notificationPayloadRuleDescription = "rule_description"
	notificationPayloadRuleURL         = "rule_url"
	notificationPayloadTotalRisk       = "total_risk"
	notificationPayloadPublishDate     = "publish_date"
)

// Constants for notification context expected fields
const (
	notificationContextDisplayName = "display_name"
	notificationContextHostURL     = "host_url"
)

var (
	notificationType types.EventType
)

// showVersion function displays version information.
func showVersion() {
	fmt.Println(versionMessage)
}

// showAuthors function displays information about authors.
func showAuthors() {
	fmt.Println(authorsMessage)
}

func calculateTotalRisk(impact, likelihood int) int {
	return (impact + likelihood) / 2
}

// ccx_rules_ocp.external.rules.cluster_wide_proxy_auth_check.report
// ->
// cluster_wide_proxy_auth_check
func moduleToRuleName(module string) string {
	result := strings.TrimSuffix(module, ".report")
	result = strings.TrimPrefix(result, "ccx_rules_ocp.")
	result = strings.TrimPrefix(result, "external.")
	result = strings.TrimPrefix(result, "rules.")
	result = strings.TrimPrefix(result, "bug_rules.")
	result = strings.TrimPrefix(result, "ocs.")

	return result
}

func findRuleByNameAndErrorKey(ruleContent map[string]types.RuleContent, impacts types.Impacts, ruleName string, errorKey string) (int, int, int) {
	rc := ruleContent[ruleName]
	ek := rc.ErrorKeys
	val := ek[errorKey]
	likelihood := val.Metadata.Likelihood
	impact := impacts[val.Metadata.Impact]
	totalRisk := calculateTotalRisk(likelihood, impact)
	return val.Metadata.Likelihood, impact, totalRisk
}

func waitForEnter() {
	fmt.Println("\n... demo mode ... Press 'Enter' to continue...")
	_, err := bufio.NewReader(os.Stdin).ReadBytes('\n')
	if err != nil {
		log.Error().Err(err)
	}
}

func processReportsByCluster(ruleContent map[string]types.RuleContent, impacts types.Impacts, storage *DBStorage, clusters []types.ClusterEntry, notificationConfig conf.NotificationsConfiguration, notifier *producer.KafkaProducer) {
	for i, cluster := range clusters {
		log.Info().
			Int("#", i).
			Int(organizationIDAttribute, int(cluster.OrgID)).
			Int(AccountNumberAttribute, int(cluster.AccountNumber)).
			Str(clusterAttribute, string(cluster.ClusterName)).
			Msg(clusterEntryMessage)

		report, reportedAt, err := storage.ReadReportForCluster(cluster.OrgID, cluster.ClusterName)
		if err != nil {
			log.Err(err).Msg(operationFailedMessage)
			os.Exit(ExitStatusStorageError)
		}

		var deserialized types.Report
		err = json.Unmarshal([]byte(report), &deserialized)
		if err != nil {
			log.Err(err).Msg("Deserialization error - Couldn't create report object")
			os.Exit(ExitStatusStorageError)
		}

		if len(deserialized.Reports) == 0 {
			log.Info().Msgf("No reports in notification database for cluster %s", cluster.ClusterName)
			continue
		}

		notificationMsg := generateNotificationMessage(notificationConfig.ClusterDetailsURI, string(cluster.AccountNumber), string(cluster.ClusterName))

		for i, r := range deserialized.Reports {
			ruleName := moduleToRuleName(string(r.Module))
			errorKey := string(r.ErrorKey)
			likelihood, impact, totalRisk := findRuleByNameAndErrorKey(ruleContent, impacts, ruleName, errorKey)

			log.Info().
				Int("#", i).
				Str("type", r.Type).
				Str("rule", ruleName).
				Str("error key", errorKey).
				Int("likelihood", likelihood).
				Int("impact", impact).
				Int(totalRiskAttribute, totalRisk).
				Msg("Report")
			if totalRisk >= 3 {
				log.Warn().Int(totalRiskAttribute, totalRisk).Msg("Report with high impact detected")
				appendEventToNotificationMessage(notificationConfig.RuleDetailsURI, &notificationMsg, ruleName, totalRisk, time.Time(reportedAt).UTC().Format(time.RFC3339Nano))
			}
		}

		if len(notificationMsg.Events) == 0 {
			log.Info().Msgf("No new issues to notify for cluster %s", string(cluster.ClusterName))
			continue
		}

		log.Info().Msgf("Producing instant notification for cluster %s with %d events", string(cluster.ClusterName), len(notificationMsg.Events))
		_, _, err = notifier.ProduceMessage(notificationMsg)
		if err != nil {
			log.Error().
				Str(errorStr, err.Error()).
				Msg("Couldn't produce kafka event.")
			os.Exit(ExitStatusKafkaProducerError)
		}
	}
}

func getNotificationMessageForCurrentAccount(notificationsByAccount map[types.AccountNumber]types.NotificationMessage, accountNumber types.AccountNumber) (notificationMsg types.NotificationMessage) {
	if _, ok := notificationsByAccount[accountNumber]; !ok {
		log.Info().Msgf("Creating notification message for account ", accountNumber)
		notificationMsg = generateWeeklyNotificationMessage(string(accountNumber))
	} else {
		log.Info().Msgf("Modifying notification message for account ", accountNumber)
		notificationMsg = notificationsByAccount[accountNumber]
	}
	return
}

func processAllReportsFromCurrentWeek(ruleContent map[string]types.RuleContent, impacts types.Impacts, storage *DBStorage, clusters []types.ClusterEntry, notificationConfig conf.NotificationsConfiguration, notifier *producer.KafkaProducer) {
	notificationMsg := types.NotificationMessage{}
	notificationsByAccount := map[types.AccountNumber]types.NotificationMessage{}

	for i, cluster := range clusters {
		log.Info().
			Int("#", i).
			Int(organizationIDAttribute, int(cluster.OrgID)).
			Int(AccountNumberAttribute, int(cluster.AccountNumber)).
			Str(clusterAttribute, string(cluster.ClusterName)).
			Msg(clusterEntryMessage)

		notificationMsg = getNotificationMessageForCurrentAccount(notificationsByAccount, cluster.AccountNumber)

		report, reportedAt, err := storage.ReadReportForCluster(cluster.OrgID, cluster.ClusterName)
		if err != nil {
			log.Err(err).Msg(operationFailedMessage)
			os.Exit(ExitStatusStorageError)
		}

		var deserialized types.Report
		err = json.Unmarshal([]byte(report), &deserialized)
		if err != nil {
			log.Err(err).Msg("Deserialization error - Couldn't create report object")
			os.Exit(ExitStatusStorageError)
		}

		if len(deserialized.Reports) == 0 {
			log.Info().Msgf("No reports in notification database for cluster %s", cluster.ClusterName)
			continue
		}

		for i, r := range deserialized.Reports {
			ruleName := moduleToRuleName(string(r.Module))
			errorKey := string(r.ErrorKey)
			likelihood, impact, totalRisk := findRuleByNameAndErrorKey(ruleContent, impacts, ruleName, errorKey)

			log.Info().
				Int("#", i).
				Str("type", r.Type).
				Str("rule", ruleName).
				Str("error key", errorKey).
				Int("likelihood", likelihood).
				Int("impact", impact).
				Int(totalRiskAttribute, totalRisk).
				Msg("Report")
			if totalRisk >= 3 {
				log.Warn().Int(totalRiskAttribute, totalRisk).Msg("Report with high impact detected. Adding to weekly digest")
				appendEventToNotificationMessage(notificationConfig.RuleDetailsURI, &notificationMsg, ruleName, totalRisk, time.Time(reportedAt).UTC().Format(time.RFC3339Nano))
			}
		}
		notificationsByAccount[cluster.AccountNumber] = notificationMsg
	}

	for account, notificationMsg := range notificationsByAccount {
		if len(notificationMsg.Events) == 0 {
			log.Info().Msgf("No issues to notify to account %d", account)
			continue
		}

		log.Info().Msgf("Producing weekly notification for account %d with %d events.", account, len(notificationMsg.Events))
		_, _, err := notifier.ProduceMessage(notificationMsg)
		if err != nil {
			log.Error().
				Str(errorStr, err.Error()).
				Msg("Couldn't produce kafka event.")
			os.Exit(ExitStatusKafkaProducerError)
		}
	}
}

func processClusters(ruleContent map[string]types.RuleContent, impacts types.Impacts, storage *DBStorage, clusters []types.ClusterEntry, config conf.ConfigStruct) {

	log.Info().Msg(separator)
	log.Info().Msg("Preparing Kafka producer")

	notifier := setupNotificationProducer(conf.GetKafkaBrokerConfiguration(config))
	log.Info().Msg("Kafka producer ready")

	waitForEnter()

	notificationConfig := conf.GetNotificationsConfiguration(config)

	if notificationType == types.InstantNotif {
		processReportsByCluster(ruleContent, impacts, storage, clusters, notificationConfig, notifier)
	} else if notificationType == types.WeeklyDigest {
		processAllReportsFromCurrentWeek(ruleContent, impacts, storage, clusters, notificationConfig, notifier)
	}

	err := notifier.Close()
	if err != nil {
		log.Error().
			Str(errorStr, err.Error()).
			Msg("Couldn't close Kafka connection.")
		os.Exit(ExitStatusKafkaConnectionNotClosedError)
	}
}

func printClusters(clusters []types.ClusterEntry) {
	for i, cluster := range clusters {
		log.Info().
			Int("#", i).
			Int(organizationIDAttribute, int(cluster.OrgID)).
			Int(AccountNumberAttribute, int(cluster.AccountNumber)).
			Str(clusterAttribute, string(cluster.ClusterName)).
			Msg(clusterEntryMessage)
	}
}

func setupNotificationProducer(brokerConfig conf.KafkaConfiguration) (notifier *producer.KafkaProducer) {
	notifier, err := producer.New(brokerConfig)
	if err != nil {
		log.Error().
			Str(errorStr, err.Error()).
			Msg("Couldn't initialize Kafka producer with the provided config.")
		os.Exit(ExitStatusKafkaBrokerError)
	}
	return
}

func generateNotificationMessage(clusterURI string, accountID string, clusterID string) (notification types.NotificationMessage) {
	events := []types.Event{}
	context := types.NotificationContext{
		notificationContextDisplayName: clusterID,
		notificationContextHostURL:     strings.Replace(clusterURI, "{cluster}", clusterID, 1),
	}

	notification = types.NotificationMessage{
		Bundle:      notificationBundleName,
		Application: notificationApplicationName,
		EventType:   notificationType.String(),
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		AccountID:   accountID,
		Events:      events,
		Context:     toJsonEscapedString(context),
	}
	return
}

func generateWeeklyNotificationMessage(accountID string) (notification types.NotificationMessage) {
	events := []types.Event{}
	context := types.NotificationContext{}

	notification = types.NotificationMessage{
		Bundle:      notificationBundleName,
		Application: notificationApplicationName,
		EventType:   notificationType.String(),
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		AccountID:   accountID,
		Events:      events,
		Context:     toJsonEscapedString(context),
	}
	return
}

func toJsonEscapedString(i interface{}) string {
	b, err := json.Marshal(i)
	if err != nil {
		log.Err(err).Msg(invalidJsonContent)
	}
	s := string(b)
	return s
}

func appendEventToNotificationMessage(ruleURI string, notification *types.NotificationMessage, ruleName string, totalRisk int, publishDate string) {
	payload := types.EventPayload{
		notificationPayloadRuleDescription: ruleName,
		notificationPayloadRuleURL:         strings.Replace(ruleURI, "{rule}", ruleName, 1),
		notificationPayloadTotalRisk:       string(totalRisk),
		notificationPayloadPublishDate:     publishDate,
	}
	event := types.Event{
		//The insights Notifications backend expects this field to be an empty object in the received JSON
		Metadata: types.EventMetadata{},
		//The insights Notifications backend expects to receive the payload as a string with all its fields as escaped strings
		Payload: toJsonEscapedString(payload),
	}
	notification.Events = append(notification.Events, event)
}

func checkArgs(args *types.CliFlags) {
	switch {
	case args.ShowVersion:
		showVersion()
		os.Exit(ExitStatusOK)
	case args.ShowAuthors:
		showAuthors()
		os.Exit(ExitStatusOK)
	default:
	}

	// check if report type is specified on command line
	if !args.InstantReports && !args.WeeklyReports {
		log.Error().Msg("Type of report needs to be specified on command line")
		os.Exit(ExitStatusConfiguration)
	}

	if args.InstantReports {
		notificationType = types.InstantNotif
	} else {
		notificationType = types.WeeklyDigest
	}
}

// main function is entry point to the differ
func main() {
	var cliFlags types.CliFlags

	// define and parse all command line options
	flag.BoolVar(&cliFlags.InstantReports, "instant-reports", false, "create instant reports")
	flag.BoolVar(&cliFlags.WeeklyReports, "weekly-reports", false, "create weekly reports")
	flag.Parse()
	checkArgs(&cliFlags)

	// config has exactly the same structure as *.toml file
	config, err := conf.LoadConfiguration(configFileEnvVariableName, defaultConfigFileName)
	if err != nil {
		log.Err(err).Msg("Load configuration")
		os.Exit(ExitStatusConfiguration)
	}

	if config.Logging.Debug {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	log.Info().Msg("Differ started")
	waitForEnter()

	log.Info().Msg(separator)

	log.Info().Msg("Getting rule content and impacts from content service")

	ruleContent, impacts, err := fetchAllRulesContent(conf.GetDependenciesConfiguration(config))
	if err != nil {
		os.Exit(ExitStatusFetchContentError)
	}

	waitForEnter()

	log.Info().Msg(separator)
	log.Info().Msg("Read cluster list")

	// prepare the storage
	storageConfiguration := conf.GetStorageConfiguration(config)
	storage, err := NewStorage(storageConfiguration)
	if err != nil {
		log.Err(err).Msg(operationFailedMessage)
		os.Exit(ExitStatusStorageError)
	}

	clusters, err := storage.ReadClusterList()
	if err != nil {
		log.Err(err).Msg(operationFailedMessage)
		os.Exit(ExitStatusStorageError)
	}

	printClusters(clusters)
	log.Info().Int("clusters", len(clusters)).Msg("Read cluster list: done")
	waitForEnter()

	log.Info().Msg(separator)
	log.Info().Msg("Checking new issues for all new reports")
	waitForEnter()

	processClusters(ruleContent, impacts, storage, clusters, config)

	log.Info().Msg("Differ finished")
}
