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

// Entry point to the notification service.
//
// The purpose of this service is to enable sending automatic email
// notifications and ServiceLog events to users for all serious issues found in
// their OpenShift clusters. The "instant" mode of this service runs as a
// cronjob every fifteen minutes, and it sends a sequence of events to the
// configured Kafka topic so that the notification backend can process them and
// create email notifications based on the provided events.
//
// Additionally ServiceLog events are created, these can be displayed on
// cluster pages. Currently the events are only created for the *important* and
// *critical* issues found in the new_reports table of the configured
// PostgreSQL database. Once the reports are processed, the DB is updated with
// info about sent events by populating the reported table with the
// corresponding information. For more info about initialising the database and
// perform migrations, take a look at the
// https://github.com/RedHatInsights/ccx-notification-writer.
//
// In the instant notification mode, one email will be received for each
// cluster with important or critical issues.
//
// Additionally this service exposes several metrics about consumed and
// processed messages. These metrics can be aggregated by Prometheus and
// displayed by Grafana tools.
package main

// Entry point to the CCX Notification service

// Generated documentation is available at:
// https://pkg.go.dev/github.com/RedHatInsights/ccx-notification-service/
//
// Documentation in literate-programming-style is available at:
// https://redhatinsights.github.io/ccx-notification-service/packages/ccx_notification_service.html

import (
	"os"

	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/differ"
	"github.com/RedHatInsights/insights-operator-utils/logger"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Configuration-related constants
const (
	loadConfigurationMessage = "Load configuration"
)

func main() {
	cliFlags := setupCliFlags()
	checkArgs(&cliFlags)

	// config has exactly the same structure as *.toml file
	config, err := conf.LoadConfiguration(conf.ConfigFileEnvVariableName, conf.DefaultConfigFileName)
	if err != nil {
		log.Err(err).Msg(loadConfigurationMessage)
		os.Exit(ExitStatusConfiguration)
	}

	err = logger.InitZerolog(
		conf.GetLoggingConfiguration(&config),
		conf.GetCloudWatchConfiguration(&config),
		conf.GetSentryLoggingConfiguration(&config),
		conf.GetKafkaZerologConfiguration(&config),
	)
	if err != nil {
		log.Err(err).Msg(loadConfigurationMessage)
		os.Exit(ExitStatusConfiguration)
	}
	log.Error().Msg("TEST ERROR FOR NOTIFICATION SERVICE")
	// configuration is loaded, so it would be possible to display it if
	// asked by user
	if cliFlags.ShowConfiguration {
		showConfiguration(&config)
		os.Exit(ExitStatusOK)
	}

	// override default value by one read from configuration file
	if cliFlags.MaxAge == "" {
		cliFlags.MaxAge = config.Cleaner.MaxAge
	}

	if config.LoggingConf.Debug {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}
	// set log level
	// TODO: refactor utils/logger appropriately
	logLevel := convertLogLevel(config.LoggingConf.LogLevel)
	zerolog.SetGlobalLevel(logLevel)
	log.Info().
		Str("configured", config.LoggingConf.LogLevel).
		Int("internal", int(logLevel)).
		Msg("Log level")

	if cliFlags.Verbose {
		showConfiguration(&config)
	}

	os.Exit(differ.Run(config, cliFlags))
}
