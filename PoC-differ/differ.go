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
	"encoding/json"
	"flag"
	"fmt"
	"os"

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
)

// showVersion function displays version information.
func showVersion() {
	fmt.Println(versionMessage)
}

// showAuthors function displays information about authors.
func showAuthors() {
	fmt.Println(authorsMessage)
}

func readRuleContent(contentDirectory string) (map[string]RuleContent, []string) {
	// map used to store rule content
	contentMap := make(map[string]RuleContent)

	// map used to store invalid rules
	invalidRules := make([]string, 0)

	err := parseRulesInDirectory(contentDirectory, &contentMap, &invalidRules)
	if err != nil {
		log.Error().Err(err).Msg("parseRulesInDirectory")
		os.Exit(1)
	}
	log.Info().
		Int("parsed rules", len(contentMap)).
		Int("invalid rules", len(invalidRules)).
		Msg("Parsing rules: done")

	return contentMap, invalidRules
}

func readImpact(contentDirectory string) GlobalRuleConfig {
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

func printInvalidRules(invalidRules []string) {
	log.Info().Msg(separator)
	log.Error().Msg("List of invalid rules")
	for i, rule := range invalidRules {
		log.Error().Int("#", i+1).Str("Error", rule).Msg("Invalid rule")
	}
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

func findRuleByNameAndErrorKey(ruleContent map[string]RuleContent, impacts GlobalRuleConfig, ruleName string, errorKey string) (int, int, int) {
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
	bufio.NewReader(os.Stdin).ReadBytes('\n')
}

// main function is entry point to the differ
func main() {
	var cliFlags CliFlags

	// define and parse all command line options
	flag.BoolVar(&cliFlags.instantReports, "instant-reports", false, "create instant reports")
	flag.BoolVar(&cliFlags.weeklyReports, "weekly-reports", false, "create weekly reports")
	flag.Parse()

	switch {
	case cliFlags.showVersion:
		showVersion()
		os.Exit(ExitStatusOK)
	case cliFlags.showAuthors:
		showAuthors()
		os.Exit(ExitStatusOK)
	default:
	}

	// check if report type is specified on command line
	if !cliFlags.instantReports && !cliFlags.weeklyReports {
		log.Error().Msg("Type of report needs to be specified on command line")
		os.Exit(ExitStatusConfiguration)
	}

	// config has exactly the same structure as *.toml file
	config, err := LoadConfiguration(configFileEnvVariableName, defaultConfigFileName)
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
	log.Info().Msg("Parsing rule content")
	ruleContent, invalidRules := readRuleContent(contentDirectory)
	waitForEnter()

	if len(invalidRules) > 0 {
		printInvalidRules(invalidRules)
		waitForEnter()
	}

	log.Info().Msg(separator)
	log.Info().Msg("Read cluster list")

	// prepare the storage
	storageConfiguration := GetStorageConfiguration(config)
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

	for i, cluster := range clusters {
		log.Info().
			Int("#", i).
			Int(organizationIDAttribute, int(cluster.orgID)).
			Str(clusterAttribute, string(cluster.clusterName)).
			Msg(clusterEntryMessage)
	}
	log.Info().Int("clusters", len(clusters)).Msg("Read cluster list: done")
	waitForEnter()

	log.Info().Msg(separator)
	log.Info().Msg("Checking new issues for all new reports")
	waitForEnter()

	for i, cluster := range clusters {
		log.Info().
			Int("#", i).
			Int(organizationIDAttribute, int(cluster.orgID)).
			Str(clusterAttribute, string(cluster.clusterName)).
			Msg(clusterEntryMessage)

		report, _, err := storage.ReadReportForCluster(cluster.orgID, cluster.clusterName)
		if err != nil {
			log.Err(err).Msg(operationFailedMessage)
			os.Exit(ExitStatusStorageError)
		}

		var deserialized Report
		err = json.Unmarshal([]byte(report), &deserialized)
		if err != nil {
			log.Err(err).Msg("Deserialization error")
			os.Exit(ExitStatusStorageError)
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
			if totalRisk >= 2 {
				log.Warn().Int(totalRiskAttribute, totalRisk).Msg("Report with high impact detected")
			}
		}
	}

	log.Info().Msg("Differ finished")
}
