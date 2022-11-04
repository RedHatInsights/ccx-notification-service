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

package conf

// This source file contains definition of data type named ConfigStruct that
// represents configuration of Notification service. This source file
// also contains function named LoadConfiguration that can be used to load
// configuration from provided configuration file and/or from environment
// variables. Additionally several specific functions named
// GetStorageConfiguration, GetLoggingConfiguration,
// GetKafkaBrokerConfiguration, GetNotificationsConfiguration and
// GetMetricsConfiguration are to be used to return specific configuration
// options.

// Generated documentation is available at:
// https://pkg.go.dev/github.com/RedHatInsights/ccx-notification-service/conf
//
// Documentation in literate-programming-style is available at:
// https://redhatinsights.github.io/ccx-notification-service/packages/conf/config.html

// Default name of configuration file is config.toml
// It can be changed via environment variable NOTIFICATION_SERVICE_CONFIG_FILE

// An example of configuration file that can be used in devel environment:
//
// [storage]
// db_driver = "postgres"
// pg_username = "user"
// pg_password = "password"
// pg_host = "localhost"
// pg_port = 5432
// pg_db_name = "notification"
// pg_params = "sslmode=disable"
// log_sql_queries = true
//
// [logging]
// debug = true
// log_level = ""
//
// Environment variables that can be used to override configuration file settings:
// TBD

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// Common constants used for logging and error reporting
const (
	filenameAttribute               = "filename"
	parsingConfigurationFileMessage = "parsing configuration file"
	noKafkaConfig                   = "no Kafka configuration available in Clowder, using default one"
	noBrokerConfig                  = "warning: no broker configurations found in clowder config"
	noSaslConfig                    = "warning: SASL configuration is missing"
	noTopicMapping                  = "warning: no kafka mapping found for topic %s"
	noStorage                       = "warning: no storage section in Clowder config"
)

// ConfigStruct is a structure holding the whole notification service
// configuration
type ConfigStruct struct {
	Logging       LoggingConfiguration       `mapstructure:"logging" toml:"logging"`
	Storage       StorageConfiguration       `mapstructure:"storage" toml:"storage"`
	Kafka         KafkaConfiguration         `mapstructure:"kafka_broker" toml:"kafka_broker"`
	ServiceLog    ServiceLogConfiguration    `mapstructure:"service_log" toml:"service_log"`
	Dependencies  DependenciesConfiguration  `mapstructure:"dependencies" toml:"dependencies"`
	Notifications NotificationsConfiguration `mapstructure:"notifications" toml:"notifications"`
	Metrics       MetricsConfiguration       `mapstructure:"metrics" toml:"metrics"`
	Cleaner       CleanerConfiguration       `mapstructure:"cleaner" toml:"cleaner"`
	Processing    ProcessingConfiguration    `mapstructure:"processing" toml:"processing"`
}

// LoggingConfiguration represents configuration for logging in general
type LoggingConfiguration struct {
	// Debug enables pretty colored logging
	Debug bool `mapstructure:"debug" toml:"debug"`

	// LogLevel sets logging level to show. Possible values are:
	// "debug"
	// "info"
	// "warn", "warning"
	// "error"
	// "fatal"
	//
	// logging level won't be changed if value is not one of listed above
	LogLevel string `mapstructure:"log_level" toml:"log_level"`
}

// StorageConfiguration represents configuration of postgresQSL data storage
type StorageConfiguration struct {
	Driver        string `mapstructure:"db_driver"       toml:"db_driver"`
	PGUsername    string `mapstructure:"pg_username"     toml:"pg_username"`
	PGPassword    string `mapstructure:"pg_password"     toml:"pg_password"`
	PGHost        string `mapstructure:"pg_host"         toml:"pg_host"`
	PGPort        int    `mapstructure:"pg_port"         toml:"pg_port"`
	PGDBName      string `mapstructure:"pg_db_name"      toml:"pg_db_name"`
	PGParams      string `mapstructure:"pg_params"       toml:"pg_params"`
	LogSQLQueries bool   `mapstructure:"log_sql_queries" toml:"log_sql_queries"`
}

// DependenciesConfiguration represents configuration of external services and other dependencies
type DependenciesConfiguration struct {
	ContentServiceServer     string `mapstructure:"content_server" toml:"content_server"`
	ContentServiceEndpoint   string `mapstructure:"content_endpoint" toml:"content_endpoint"`
	TemplateRendererServer   string `mapstructure:"template_renderer_server" toml:"template_renderer_server"`
	TemplateRendererEndpoint string `mapstructure:"template_renderer_endpoint" toml:"template_renderer_endpoint"`
	TemplateRendererURL      string
}

// CleanerConfiguration represents configuration for the main cleaner
type CleanerConfiguration struct {
	// MaxAge is specification of max age for records to be cleaned
	MaxAge string `mapstructure:"max_age" toml:"max_age"`
}

// KafkaConfiguration represents configuration of Kafka brokers and topics
type KafkaConfiguration struct {
	Enabled             bool          `mapstructure:"enabled" toml:"enabled"`
	Address             string        `mapstructure:"address" toml:"address"`
	SecurityProtocol    string        `mapstructure:"security_protocol" toml:"security_protocol"`
	CertPath            string        `mapstructure:"cert_path" toml:"cert_path"`
	SaslMechanism       string        `mapstructure:"sasl_mechanism" toml:"sasl_mechanism"`
	SaslUsername        string        `mapstructure:"sasl_username" toml:"sasl_username"`
	SaslPassword        string        `mapstructure:"sasl_password" toml:"sasl_password"`
	Topic               string        `mapstructure:"topic"   toml:"topic"`
	Timeout             time.Duration `mapstructure:"timeout" toml:"timeout"`
	LikelihoodThreshold int           `mapstructure:"likelihood_threshold" toml:"likelihood_threshold"`
	ImpactThreshold     int           `mapstructure:"impact_threshold" toml:"impact_threshold"`
	SeverityThreshold   int           `mapstructure:"severity_threshold" toml:"severity_threshold"`
	TotalRiskThreshold  int           `mapstructure:"total_risk_threshold" toml:"total_risk_threshold"`
	EventFilter         string        `mapstructure:"event_filter" toml:"event_filter"`
}

// ServiceLogConfiguration represents configuration of ServiceLog REST API
type ServiceLogConfiguration struct {
	Enabled             bool          `mapstructure:"enabled" toml:"enabled"`
	ClientID            string        `mapstructure:"client_id" toml:"client_id"`
	ClientSecret        string        `mapstructure:"client_secret" toml:"client_secret"`
	TokenURL            string        `mapstructure:"token_url" toml:"token_url"`
	URL                 string        `mapstructure:"url" toml:"url"`
	Timeout             time.Duration `mapstructure:"timeout" toml:"timeout"`
	LikelihoodThreshold int           `mapstructure:"likelihood_threshold" toml:"likelihood_threshold"`
	ImpactThreshold     int           `mapstructure:"impact_threshold" toml:"impact_threshold"`
	SeverityThreshold   int           `mapstructure:"severity_threshold" toml:"severity_threshold"`
	TotalRiskThreshold  int           `mapstructure:"total_risk_threshold" toml:"total_risk_threshold"`
	EventFilter         string        `mapstructure:"event_filter" toml:"event_filter"`
	RuleDetailsURI      string        `mapstructure:"rule_details_uri" toml:"rule_details_uri"`
}

// NotificationsConfiguration represents the configuration specific to the content of notifications
type NotificationsConfiguration struct {
	InsightsAdvisorURL string `mapstructure:"insights_advisor_url" toml:"insights_advisor_url"`
	ClusterDetailsURI  string `mapstructure:"cluster_details_uri" toml:"cluster_details_uri"`
	RuleDetailsURI     string `mapstructure:"rule_details_uri"    toml:"rule_details_uri"`
	Cooldown           string `mapstructure:"cooldown" toml:"cooldown"`
}

// MetricsConfiguration holds metrics related configuration
type MetricsConfiguration struct {
	Job              string        `mapstructure:"job_name" toml:"job_name"`
	Namespace        string        `mapstructure:"namespace" toml:"namespace"`
	Subsystem        string        `mapstructure:"subsystem" toml:"subsystem"`
	GatewayURL       string        `mapstructure:"gateway_url" toml:"gateway_url"`
	GatewayAuthToken string        `mapstructure:"gateway_auth_token" toml:"gateway_auth_token"`
	Retries          int           `mapstructure:"retries" toml:"retries"`
	RetryAfter       time.Duration `mapstructure:"retry_after" toml:"retry_after"`
}

// ProcessingConfiguration represents configuration for processing subsystem
type ProcessingConfiguration struct {
	FilterAllowedClusters bool     `mapstructure:"filter_allowed_clusters" toml:"filter_allowed_clusters"`
	AllowedClusters       []string `mapstructure:"allowed_clusters" toml:"allowed_clusters"`
	FilterBlockedClusters bool     `mapstructure:"filter_blocked_clusters" toml:"filter_blocked_clusters"`
	BlockedClusters       []string `mapstructure:"blocked_clusters" toml:"blocked_clusters"`
}

// LoadConfiguration loads configuration from defaultConfigFile, file set in
// configFileEnvVariableName or from env
func LoadConfiguration(configFileEnvVariableName, defaultConfigFile string) (ConfigStruct, error) {
	var configuration ConfigStruct

	// env. variable holding name of configuration file
	configFile, specified := os.LookupEnv(configFileEnvVariableName)
	if specified {
		log.Info().Str(filenameAttribute, configFile).Msg(parsingConfigurationFileMessage)
		// we need to separate the directory name and filename without
		// extension
		directory, basename := filepath.Split(configFile)
		file := strings.TrimSuffix(basename, filepath.Ext(basename))
		// parse the configuration
		viper.SetConfigName(file)
		viper.AddConfigPath(directory)
	} else {
		log.Info().Str(filenameAttribute, defaultConfigFile).Msg(parsingConfigurationFileMessage)
		// parse the configuration
		viper.SetConfigName(defaultConfigFile)
		viper.AddConfigPath(".")
	}

	// try to read the whole configuration
	err := viper.ReadInConfig()
	if _, isNotFoundError := err.(viper.ConfigFileNotFoundError); !specified && isNotFoundError {
		// If config file is not present (which might be correct in
		// some environment) we need to read configuration from
		// environment variables. The problem is that Viper is not
		// smart enough to understand the structure of config by
		// itself, so we need to read fake config file
		fakeTomlConfigWriter := new(bytes.Buffer)

		err := toml.NewEncoder(fakeTomlConfigWriter).Encode(configuration)
		if err != nil {
			return configuration, err
		}

		fakeTomlConfig := fakeTomlConfigWriter.String()

		viper.SetConfigType("toml")

		err = viper.ReadConfig(strings.NewReader(fakeTomlConfig))
		if err != nil {
			return configuration, err
		}
	} else if err != nil {
		// error is processed on caller side
		return configuration, fmt.Errorf("fatal error config file: %s", err)
	}

	// override configuration from env if there's variable in env

	const envPrefix = "CCX_NOTIFICATION_SERVICE_"

	viper.AutomaticEnv()
	viper.SetEnvPrefix(envPrefix)
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "__"))

	err = viper.Unmarshal(&configuration)
	if err != nil {
		return configuration, err
	}

	updateConfigFromClowder(&configuration)

	configuration.Dependencies.TemplateRendererURL, err = createURL(
		configuration.Dependencies.TemplateRendererServer,
		configuration.Dependencies.TemplateRendererEndpoint)
	if err != nil {
		fmt.Println("error creating content template renderer URL")
		return configuration, err
	}

	// everything's should be ok
	return configuration, nil
}

func createURL(server, endpoint string) (string, error) {
	u, err := url.Parse(server)
	if err != nil {
		return "", err
	}
	u.Path = path.Join(u.Path, endpoint)
	return u.String(), nil
}

// GetStorageConfiguration returns storage configuration
func GetStorageConfiguration(configuration *ConfigStruct) StorageConfiguration {
	return configuration.Storage
}

// GetLoggingConfiguration returns logging configuration
func GetLoggingConfiguration(configuration *ConfigStruct) LoggingConfiguration {
	return configuration.Logging
}

// GetKafkaBrokerConfiguration returns kafka broker configuration
func GetKafkaBrokerConfiguration(configuration *ConfigStruct) KafkaConfiguration {
	return configuration.Kafka
}

// GetServiceLogConfiguration returns ServiceLog configuration
func GetServiceLogConfiguration(configuration *ConfigStruct) ServiceLogConfiguration {
	return configuration.ServiceLog
}

// GetDependenciesConfiguration returns dependencies configuration
func GetDependenciesConfiguration(configuration ConfigStruct) DependenciesConfiguration {
	return configuration.Dependencies
}

// GetNotificationsConfiguration returns configuration related with notification content
func GetNotificationsConfiguration(configuration ConfigStruct) NotificationsConfiguration {
	return configuration.Notifications
}

// GetMetricsConfiguration returns metrics configuration
func GetMetricsConfiguration(configuration *ConfigStruct) MetricsConfiguration {
	return configuration.Metrics
}

// GetCleanerConfiguration returns cleaner configuration
func GetCleanerConfiguration(configuration ConfigStruct) CleanerConfiguration {
	return configuration.Cleaner
}

// GetProcessingConfiguration returns processing configuration
func GetProcessingConfiguration(configuration ConfigStruct) ProcessingConfiguration {
	return configuration.Processing
}

// updateConfigFromClowder updates the current config with the values defined in clowder
func updateConfigFromClowder(configuration *ConfigStruct) {
	if !clowder.IsClowderEnabled() || clowder.LoadedConfig == nil {
		fmt.Println("Clowder is disabled")
		return
	}

	fmt.Println("Clowder is enabled")
	if clowder.LoadedConfig.Kafka == nil {
		fmt.Println(noKafkaConfig)
	} else {
		// make sure broker(s) are configured in Clowder
		if len(clowder.LoadedConfig.Kafka.Brokers) > 0 {
			broker := clowder.LoadedConfig.Kafka.Brokers[0]
			// port can be empty in clowder, so taking it into account
			if broker.Port != nil {
				configuration.Kafka.Address = fmt.Sprintf("%s:%d", broker.Hostname, *broker.Port)
			} else {
				configuration.Kafka.Address = broker.Hostname
			}

			// SSL config
			if broker.Authtype != nil {
				fmt.Println("kafka is configured to use authentication")
				if broker.Sasl != nil {
					configuration.Kafka.SaslUsername = *broker.Sasl.Username
					configuration.Kafka.SaslPassword = *broker.Sasl.Password
					configuration.Kafka.SaslMechanism = *broker.Sasl.SaslMechanism
					configuration.Kafka.SecurityProtocol = *broker.Sasl.SecurityProtocol
					if caPath, err := clowder.LoadedConfig.KafkaCa(broker); err == nil {
						configuration.Kafka.CertPath = caPath
					}
				} else {
					fmt.Println(noSaslConfig)
				}
			}

		} else {
			fmt.Println(noBrokerConfig)
		}

		updateTopicsMapping(configuration)
	}

	if clowder.LoadedConfig.Database != nil {
		// get DB configuration from clowder
		configuration.Storage.PGDBName = clowder.LoadedConfig.Database.Name
		configuration.Storage.PGHost = clowder.LoadedConfig.Database.Hostname
		configuration.Storage.PGPort = clowder.LoadedConfig.Database.Port
		configuration.Storage.PGUsername = clowder.LoadedConfig.Database.Username
		configuration.Storage.PGPassword = clowder.LoadedConfig.Database.Password
	} else {
		fmt.Println(noStorage)
	}
}

func updateTopicsMapping(configuration *ConfigStruct) {
	// Updating topics from clowder mapping if available
	if topicCfg, ok := clowder.KafkaTopics[configuration.Kafka.Topic]; ok {
		configuration.Kafka.Topic = topicCfg.Name
	} else {
		fmt.Printf(noTopicMapping, configuration.Kafka.Topic)
	}
}
