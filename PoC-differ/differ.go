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
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
)

const contentDirectory = "content"

// Configuration-related constants
const (
	configFileEnvVariableName = "NOTIFICATION_DIFFER_CONFIG_FILE"
	defaultConfigFileName     = "config"
)

// main function is entry point to the differ
func main() {
	// config has exactly the same structure as *.toml file
	config, err := LoadConfiguration(configFileEnvVariableName, defaultConfigFileName)
	if err != nil {
		log.Err(err).Msg("Load configuration")
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
	log.Info().Msg("Finished")
}
