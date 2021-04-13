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
	versionMessage = "Notification writer version 1.0"
	authorsMessage = "Pavel Tisnovsky, Red Hat Inc."
)

// showVersion function displays version information.
func showVersion() {
	fmt.Println(versionMessage)
}

// showAuthors function displays information about authors.
func showAuthors() {
	fmt.Println(authorsMessage)
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

	log.Info().Msg("Started")

	log.Info().Msg("Parsing rule content")

	// map used to store rule content
	contentMap := make(map[string]RuleContent)

	invalidRules := make([]string, 0)

	err = parseRulesInDirectory(contentDirectory, &contentMap, &invalidRules)
	if err != nil {
		log.Error().Err(err).Msg("parseRulesInDirectory")
		os.Exit(1)
	}
	log.Info().
		Int("parsed rules", len(contentMap)).
		Int("invalid rules", len(invalidRules)).
		Msg("Done")

	if len(invalidRules) > 0 {
		log.Error().Msg("List of invalid rules")
		for i, rule := range invalidRules {
			log.Error().Int("#", i+1).Str("Error", rule).Msg("Invalid rule")
		}
	}

	// prepare the storage
	storageConfiguration := GetStorageConfiguration(config)
	storage, err := NewStorage(storageConfiguration)
	if err != nil {
		log.Err(err).Msg("Operation failed")
		os.Exit(ExitStatusStorageError)
	}

	clusters, err := storage.ReadClusterList()
	if err != nil {
		log.Err(err).Msg("Operation failed")
		os.Exit(ExitStatusStorageError)
	}

	for i, cluster := range clusters {
		log.Info().
			Int("#", i).
			Int("org id", int(cluster.orgID)).
			Str("cluster", string(cluster.clusterName)).
			Msg("cluster entry")
	}

	report, _, err := storage.ReadReportForCluster(1, "5d5892d3-1f74-4ccf-91af-548dfc9767aa")
	if err != nil {
		log.Err(err).Msg("Operation failed")
		os.Exit(ExitStatusStorageError)
	}
	log.Info().Str("report", string(report)).Msg("report")

	log.Info().Msg("Finished")
}
