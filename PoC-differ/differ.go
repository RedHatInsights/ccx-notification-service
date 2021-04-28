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

const contentDirectory = "content"

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
	clusterAttribute        = "cluster"
	totalRiskAttribute      = "totalRisk"
	errorKey                = "Error:"
)

// Notification-related constants
const (
	defaultNotificationBundleName      = "openshift"
	defaultNotificationApplicationName = "advisor"
)

// showVersion function displays version information.
func showVersion() {
	fmt.Println(versionMessage)
}

// showAuthors function displays information about authors.
func showAuthors() {
	fmt.Println(authorsMessage)
}

func readImpact(contentDirectory string) types.GlobalRuleConfig {
	impact, err := parseGlobalContentConfig(contentDirectory + "/config.yaml")
	if err != nil {
		log.Error().Err(err).Msg("parsing impact")
		os.Exit(ExitStatusConfiguration)
	}
	log.Info().
		Int("parsed impact factors", len(impact.Impact)).
		Msg("Read impact: done")

	return impact
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

func findRuleByNameAndErrorKey(ruleContent map[string]types.RuleContent, impacts types.GlobalRuleConfig, ruleName string, errorKey string) (int, int, int) {
	rc := ruleContent[ruleName]
	ek := rc.ErrorKeys
	val := ek[errorKey]
	likelihood := val.Metadata.Likelihood
	impact := impacts.Impact[val.Metadata.Impact]
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

func processClusters(ruleContent map[string]types.RuleContent, impacts types.GlobalRuleConfig, storage *DBStorage, clusters []types.ClusterEntry, kafkaConfig conf.KafkaConfiguration) {
	for i, cluster := range clusters {
		log.Info().
			Int("#", i).
			Int(organizationIDAttribute, int(cluster.OrgID)).
			Str(clusterAttribute, string(cluster.ClusterName)).
			Msg(clusterEntryMessage)

		report, _, err := storage.ReadReportForCluster(cluster.OrgID, cluster.ClusterName)
		if err != nil {
			log.Err(err).Msg(operationFailedMessage)
			os.Exit(ExitStatusStorageError)
		}

		var deserialized types.Report
		err = json.Unmarshal([]byte(report), &deserialized)
		if err != nil {
			log.Err(err).Msg("Deserialization error")
			os.Exit(ExitStatusStorageError)
		}

		log.Info().Msg(separator)
		log.Info().Msg("Preparing Kafka producer")

		notifier := setupNotificationProducer(kafkaConfig)
		log.Info().Msg("Kafka producer ready")

		waitForEnter()

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
			if totalRisk >= 2 {
				log.Warn().Int(totalRiskAttribute, totalRisk).Msg("Report with high impact detected")
				// TODO: Decide actual content of notification message's payload.
				notification := generateNotificationMessage(ruleName, totalRisk, string(cluster.OrgID), types.InstantNotif)
				_, _, err = notifier.ProduceMessage(notification)
				if err != nil {
					log.Error().
						Str(errorKey, err.Error()).
						Msg("Couldn't produce kafka event.")
					os.Exit(ExitStatusKafkaProducerError)
				}
			}
		}

		err = notifier.Close()
		if err != nil {
			// TODO: Can this be handled somehow?
			log.Error().
				Str(errorKey, err.Error()).
				Msg("Couldn't close Kafka connection.")
			os.Exit(ExitStatusKafkaConnectionNotClosedError)
		}
	}
}

func printClusters(clusters []types.ClusterEntry) {
	for i, cluster := range clusters {
		log.Info().
			Int("#", i).
			Int(organizationIDAttribute, int(cluster.OrgID)).
			Str(clusterAttribute, string(cluster.ClusterName)).
			Msg(clusterEntryMessage)
	}
}

func setupNotificationProducer(brokerConfig conf.KafkaConfiguration) (notifier *producer.KafkaProducer) {
	notifier, err := producer.New(brokerConfig)
	if err != nil {
		log.Error().
			Str(errorKey, err.Error()).
			Msg("Couldn't initialize Kafka producer with the provided config.")
		os.Exit(ExitStatusKafkaBrokerError)
	}
	return
}

func generateNotificationMessage(ruleName string, totalRisk int, accountID string, eventType types.EventType) (notification types.NotificationMessage) {
	//TODO: Discuss actual payload content
	events := []types.Event{
		{
			Metadata: nil,
			Payload: map[string]interface{}{
				//TODO: Define payload's keys and add them as constants
				"rule_id":    ruleName,
				"total_risk": totalRisk,
			},
		},
	}
	notification = types.NotificationMessage{
		Bundle:      defaultNotificationBundleName,
		Application: defaultNotificationApplicationName,
		EventType:   eventType.String(),
		Timestamp:   time.Now().UTC().Format(time.RFC3339Nano),
		AccountID:   accountID,
		Events:      events,
		Context:     nil,
	}
	return
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
	log.Info().Msg("Parsing impact from global content configuration")
	impacts := readImpact(contentDirectory)
	waitForEnter()

	log.Info().Msg(separator)

	log.Info().Msg("Getting rule content from content service")

	ruleContent, err := fetchAllRulesContent(conf.GetDependenciesConfiguration(config))

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

	processClusters(ruleContent, impacts, storage, clusters, conf.GetKafkaBrokerConfiguration(config))

	log.Info().Msg("Differ finished")
}
