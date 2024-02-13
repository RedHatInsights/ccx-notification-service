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

package conf_test

// Unit test definitions for functions and methods defined in source file
// config.go
//
// Documentation in literate-programming-style is available at:
// https://redhatinsights.github.io/ccx-notification-service/packages/conf_test/configuration_test.html

import (
	"fmt"
	"os"
	"time"

	"testing"

	"github.com/RedHatInsights/insights-operator-utils/logger"
	"github.com/RedHatInsights/insights-operator-utils/tests/helpers"
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	conf "github.com/RedHatInsights/ccx-notification-service/conf"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
}

// mustLoadConfiguration function loads configuration file or the actual test
// will fail
func mustLoadConfiguration(t *testing.T, envVar string) {
	_, err := conf.LoadConfiguration(envVar, "../tests/config1")
	helpers.FailOnError(t, err)
}

// mustSetEnv function set specified environment variable or the actual test
// will fail
func mustSetEnv(t *testing.T, key, val string) {
	err := os.Setenv(key, val)
	helpers.FailOnError(t, err)
}

// TestLoadDefaultConfiguration test loads a configuration file for testing
// with check that load was correct
func TestLoadDefaultConfiguration(t *testing.T) {
	os.Clearenv()
	mustLoadConfiguration(t, "nonExistingEnvVar")
}

// TestLoadConfigurationFromEnvVariable tests loading the config. file for
// testing from an environment variable
func TestLoadConfigurationFromEnvVariable(t *testing.T) {
	os.Clearenv()

	mustSetEnv(t, "CCX_NOTIFICATION_SERVICE_CONFIG_FILE", "../tests/config2")
	mustLoadConfiguration(t, "CCX_NOTIFICATION_SERVICE_CONFIG_FILE")
}

// TestLoadConfigurationNonEnvVarUnknownConfigFile tests loading an unexisting
// config file when no environment variable is provided
func TestLoadConfigurationNonEnvVarUnknownConfigFile(t *testing.T) {
	_, err := conf.LoadConfiguration("", "foobar")
	assert.Nil(t, err)
}

// TestLoadConfigurationBadConfigFile tests loading an unexisting config file when no environment variable is provided
func TestLoadConfigurationBadConfigFile(t *testing.T) {
	_, err := conf.LoadConfiguration("", "../tests/config3")
	assert.Contains(t, err.Error(), `fatal error config file: While parsing config:`)
}

// TestLoadingConfigurationEnvVariableBadValueNoDefaultConfig tests loading a non-existent configuration file set in environment
func TestLoadingConfigurationEnvVariableBadValueNoDefaultConfig(t *testing.T) {
	os.Clearenv()

	mustSetEnv(t, "CCX_NOTIFICATION_SERVICE_CONFIG_FILE", "non existing file")

	_, err := conf.LoadConfiguration("CCX_NOTIFICATION_SERVICE_CONFIG_FILE", "")
	assert.Contains(t, err.Error(), `fatal error config file: Config File "non existing file" Not Found in`)
}

// TestLoadingConfigurationEnvVariableBadValueNoDefaultConfig tests that if env var is provided, it must point to a valid config file
func TestLoadingConfigurationEnvVariableBadValueDefaultConfigFailure(t *testing.T) {
	os.Clearenv()

	mustSetEnv(t, "CCX_NOTIFICATION_SERVICE_CONFIG_FILE", "non existing file")

	_, err := conf.LoadConfiguration("CCX_NOTIFICATION_SERVICE_CONFIG_FILE", "../tests/config1")
	assert.Contains(t, err.Error(), `fatal error config file: Config File "non existing file" Not Found in`)
}

// TestLoadBrokerConfiguration tests loading the broker configuration sub-tree
func TestLoadBrokerConfiguration(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"
	expectedTimeout, _ := time.ParseDuration("20s")

	mustSetEnv(t, envVar, "../tests/config2")
	config, err := conf.LoadConfiguration(envVar, "")
	assert.Nil(t, err, "Failed loading configuration file from env var!")

	brokerCfg := conf.GetKafkaBrokerConfiguration(&config)

	assert.True(t, brokerCfg.Enabled)
	assert.Equal(t, "localhost:29092", brokerCfg.Addresses)
	assert.Equal(t, "ccx_test_notifications", brokerCfg.Topic)
	assert.Equal(t, expectedTimeout, brokerCfg.Timeout)
	assert.False(t, brokerCfg.TagFilterEnabled)

	assert.Len(t, brokerCfg.Tags, 2)
	assert.Equal(t, "tag1", brokerCfg.Tags[0])
	assert.Equal(t, "tag2", brokerCfg.Tags[1])

	assert.Len(t, brokerCfg.TagsSet, 2)

	_, foundTag1 := brokerCfg.TagsSet["tag1"]
	assert.True(t, foundTag1)
	_, foundTag2 := brokerCfg.TagsSet["tag2"]
	assert.True(t, foundTag2)
}

// TestLoadServiceLogConfiguration tests loading the ServiceLog configuration
// sub-tree
func TestLoadServiceLogConfiguration(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"
	expectedTimeout, _ := time.ParseDuration("15s")

	mustSetEnv(t, envVar, "../tests/config2")
	config, err := conf.LoadConfiguration(envVar, "")
	assert.Nil(t, err, "Failed loading configuration file from env var!")

	serviceLogCfg := conf.GetServiceLogConfiguration(&config)

	assert.False(t, serviceLogCfg.Enabled)
	assert.Equal(t, 3, serviceLogCfg.TotalRiskThreshold)
	assert.Equal(t, "test-id", serviceLogCfg.ClientID)
	assert.Equal(t, "test-secret", serviceLogCfg.ClientSecret)
	assert.Equal(t, "token url", serviceLogCfg.TokenURL)
	assert.Equal(t, "localhost:8000/api/service_logs/v1/cluster_logs/", serviceLogCfg.URL)
	assert.Equal(t, expectedTimeout, serviceLogCfg.Timeout)
	assert.Equal(t, "https://console.redhat.com/openshift/insights/advisor/recommendations/{module}|{error_key}", serviceLogCfg.RuleDetailsURI)

	assert.False(t, serviceLogCfg.TagFilterEnabled)
	assert.Len(t, serviceLogCfg.Tags, 2)
	assert.Equal(t, "tag3", serviceLogCfg.Tags[0])
	assert.Equal(t, "tag4", serviceLogCfg.Tags[1])

	assert.Len(t, serviceLogCfg.TagsSet, 2)

	_, foundTag3 := serviceLogCfg.TagsSet["tag3"]
	assert.True(t, foundTag3)
	_, foundTag4 := serviceLogCfg.TagsSet["tag4"]
	assert.True(t, foundTag4)
}

// TestLoadServiceLogConfigurationBadURL tests loading the ServiceLog
// configuration sub-tree with invalid URL
func TestLoadServiceLogConfigurationBadURL(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"

	// this config file contains invalid character in URL so it should not
	// be loaded properly
	mustSetEnv(t, envVar, "../tests/config4")
	_, err := conf.LoadConfiguration(envVar, "")
	assert.Error(t, err)
}

// TestLoadStorageConfiguration tests loading the storage configuration sub-tree
func TestLoadStorageConfiguration(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"
	mustSetEnv(t, envVar, "../tests/config2")
	config, err := conf.LoadConfiguration(envVar, "")
	assert.Nil(t, err, "Failed loading configuration file from env var!")

	storageCfg := conf.GetStorageConfiguration(&config)

	assert.Equal(t, "sqlite3", storageCfg.Driver)
	assert.Equal(t, "user", storageCfg.PGUsername)
	assert.Equal(t, "password", storageCfg.PGPassword)
	assert.Equal(t, "localhost", storageCfg.PGHost)
	assert.Equal(t, 5432, storageCfg.PGPort)
	assert.Equal(t, "notifications", storageCfg.PGDBName)
	assert.Equal(t, "", storageCfg.PGParams)
	assert.Equal(t, true, storageCfg.LogSQLQueries)
}

// TestLoadProcessingConfiguration3AllowedClusters tests loading the processing
// configuration sub-tree
func TestLoadProcessingConfiguration3AllowedClusters(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"

	// configuration file with three clusters in allow list
	mustSetEnv(t, envVar, "../tests/config2")
	config, err := conf.LoadConfiguration(envVar, "")
	assert.Nil(t, err, "Failed loading configuration file from env var!")

	processingCfg := conf.GetProcessingConfiguration(&config)

	assert.True(t, processingCfg.FilterAllowedClusters)
	assert.Len(t, processingCfg.AllowedClusters, 3)
	assert.Equal(t, "aaaaaaaa-0000-0000-0000-000000000000", processingCfg.AllowedClusters[0])
	assert.Equal(t, "aaaaaaaa-1111-1111-1111-111111111111", processingCfg.AllowedClusters[1])
	assert.Equal(t, "aaaaaaaa-2222-2222-2222-222222222222", processingCfg.AllowedClusters[2])
}

// TestLoadProcessingConfigurationNoAllowedClusters tests loading the
// processing configuration sub-tree
func TestLoadProcessingConfigurationNoAllowedClusters(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"

	// configuration file with no clusters in allow list
	mustSetEnv(t, envVar, "../tests/config5")
	config, err := conf.LoadConfiguration(envVar, "")
	assert.Nil(t, err, "Failed loading configuration file from env var!")

	processingCfg := conf.GetProcessingConfiguration(&config)

	assert.False(t, processingCfg.FilterAllowedClusters)
	assert.Len(t, processingCfg.AllowedClusters, 0)
}

// TestLoadProcessingConfiguration3BlockedClusters tests loading the processing
// configuration sub-tree
func TestLoadProcessingConfiguration3BlockedClusters(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"

	// configuration file with three clusters in block list
	mustSetEnv(t, envVar, "../tests/config2")
	config, err := conf.LoadConfiguration(envVar, "")
	assert.Nil(t, err, "Failed loading configuration file from env var!")

	processingCfg := conf.GetProcessingConfiguration(&config)

	assert.True(t, processingCfg.FilterBlockedClusters)
	assert.Len(t, processingCfg.BlockedClusters, 3)
	assert.Equal(t, "bbbbbbbb-0000-0000-0000-000000000000", processingCfg.BlockedClusters[0])
	assert.Equal(t, "bbbbbbbb-1111-1111-1111-111111111111", processingCfg.BlockedClusters[1])
	assert.Equal(t, "bbbbbbbb-2222-2222-2222-222222222222", processingCfg.BlockedClusters[2])
}

// TestLoadProcessingConfigurationNoBlockedClusters tests loading the
// processing configuration sub-tree
func TestLoadProcessingConfigurationNoBlockedClusters(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"

	// configuration file with no clusters in block list
	mustSetEnv(t, envVar, "../tests/config5")
	config, err := conf.LoadConfiguration(envVar, "")
	assert.Nil(t, err, "Failed loading configuration file from env var!")

	processingCfg := conf.GetProcessingConfiguration(&config)

	assert.False(t, processingCfg.FilterBlockedClusters)
	assert.Len(t, processingCfg.BlockedClusters, 0)
}

// TestLoadLoggingConfiguration tests loading the logging configuration sub-tree
func TestLoadLoggingConfiguration(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"
	mustSetEnv(t, envVar, "../tests/config2")
	config, err := conf.LoadConfiguration(envVar, "")
	assert.Nil(t, err, "Failed loading configuration file from env var!")

	loggingCfg := conf.GetLoggingConfiguration(&config)

	assert.Equal(t, true, loggingCfg.Debug)
	assert.Equal(t, "", loggingCfg.LogLevel)
}

// TestLoadDependenciesConfiguration tests loading the dependencies configuration sub-tree
func TestLoadDependenciesConfiguration(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"
	mustSetEnv(t, envVar, "../tests/config2")
	config, err := conf.LoadConfiguration(envVar, "")
	assert.Nil(t, err, "Failed loading configuration file from env var!")

	depsCfg := conf.GetDependenciesConfiguration(&config)

	assert.Equal(t, ":8081", depsCfg.ContentServiceEndpoint)
}

// TestLoadNotificationsConfiguration tests loading the notifications configuration sub-tree
func TestLoadNotificationsConfiguration(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"

	mustSetEnv(t, envVar, "../tests/config2")
	config, err := conf.LoadConfiguration(envVar, "")
	assert.Nil(t, err, "Failed loading configuration file from env var!")

	configuration := conf.GetNotificationsConfiguration(&config)

	assert.Equal(t, "url_to_specific_rule", configuration.RuleDetailsURI)
	assert.Equal(t, "url_to_advisor", configuration.InsightsAdvisorURL)
	assert.Equal(t, "url_to_specific_cluster", configuration.ClusterDetailsURI)
}

// TestLoadConfigurationFromEnvVariableClowderEnabled tests loading the config.
// file for testing from an environment variable. Clowder config is enabled in
// this case.
func TestLoadConfigurationFromEnvVariableClowderEnabled(t *testing.T) {
	os.Clearenv()

	mustSetEnv(t, "CCX_NOTIFICATION_SERVICE_CONFIG_FILE", "../tests/config2")
	mustSetEnv(t, "ACG_CONFIG", "../tests/clowder_config.json")
	mustLoadConfiguration(t, "CCX_NOTIFICATION_SERVICE_CONFIG_FILE")
}

// TestLoadNotificationsConfiguration tests loading the notifications configuration sub-tree
func TestLoadMetricsConfiguration(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"

	mustSetEnv(t, envVar, "../tests/config2")
	config, err := conf.LoadConfiguration(envVar, "")
	assert.Nil(t, err, "Failed loading configuration file from env var!")

	configuration := conf.GetMetricsConfiguration(&config)

	assert.Equal(t, "ccx_notification_service_namespace", configuration.Namespace)
	assert.Equal(t, ":9091", configuration.GatewayURL)
	assert.Equal(t, "", configuration.GatewayAuthToken)
	assert.Equal(t, 60*time.Second, configuration.GatewayTimeBetweenPush)
}

// TestLoadNotificationsConfiguration tests loading the notifications configuration sub-tree
func TestLoadCleanerConfiguration(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"

	mustSetEnv(t, envVar, "../tests/config2")
	config, err := conf.LoadConfiguration(envVar, "")
	assert.Nil(t, err, "Failed loading configuration file from env var!")

	configuration := conf.GetCleanerConfiguration(&config)

	assert.Equal(t, "90 days", configuration.MaxAge)
}

// TestLoadClowderConfiguration tests loading a clowder config that should overwrite some
// values of the default loaded config
func TestLoadClowderConfiguration(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"
	os.Clearenv()
	mustSetEnv(t, "ACG_CONFIG", "../tests/clowder_config.json")
	hostname := "kafka"
	port := 9092

	clowder.LoadedConfig = &clowder.AppConfig{
		Kafka: &clowder.KafkaConfig{
			Brokers: []clowder.BrokerConfig{
				{
					Hostname: hostname,
					Port:     &port,
				},
			},
		},
	}

	cfg, err := conf.LoadConfiguration(envVar, "../tests/config1")
	assert.NoError(t, err, "error loading configuration")

	brokerCfg := conf.GetKafkaBrokerConfiguration(&cfg)
	assert.Equal(t, fmt.Sprintf("%s:%d", hostname, port), brokerCfg.Addresses)
}

// TestLoadStorageConfigFromClowder tests loading a clowder config that should overwrite some
// values of the default loaded config
func TestLoadStorageConfigFromClowder(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"
	os.Clearenv()
	mustSetEnv(t, "ACG_CONFIG", "../tests/clowder_config.json")
	dbName := "a_name"
	hostname := "pg_hostname"
	port := 5154
	user := "myusername"
	pass := "secretpassword"

	clowder.LoadedConfig = &clowder.AppConfig{
		Database: &clowder.DatabaseConfig{
			Name:     dbName,
			Hostname: hostname,
			Port:     port,
			Username: user,
			Password: pass,
		},
	}

	cfg, err := conf.LoadConfiguration(envVar, "../tests/config1")
	assert.NoError(t, err, "error loading configuration")

	storageCfg := conf.GetStorageConfiguration(&cfg)
	assert.Equal(t, dbName, storageCfg.PGDBName)
	assert.Equal(t, hostname, storageCfg.PGHost)
	assert.Equal(t, port, storageCfg.PGPort)
	assert.Equal(t, user, storageCfg.PGUsername)
	assert.Equal(t, pass, storageCfg.PGPassword)
}

// TestGetCloudWatchConfiguration checks the function GetCloudWatchConfiguration
func TestGetCloudWatchConfiguration(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"
	os.Clearenv()
	mustSetEnv(t, "ACG_CONFIG", "../tests/clowder_config.json")

	cfg, err := conf.LoadConfiguration(envVar, "../tests/config1")
	assert.NoError(t, err, "error loading configuration")

	assert.Equal(t, logger.CloudWatchConfiguration{
		AWSAccessID:             "",
		AWSSecretKey:            "",
		AWSSessionToken:         "",
		AWSRegion:               "",
		LogGroup:                "",
		StreamName:              "",
		CreateStreamIfNotExists: false,
		Debug:                   false,
	}, conf.GetCloudWatchConfiguration(&cfg))
}

// TestGetSentryLoggingConfiguration checks the function GetSentryLoggingConfiguration
func TestGetSentryLoggingConfiguration(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"
	os.Clearenv()
	mustSetEnv(t, "ACG_CONFIG", "../tests/sentry_logging.json")

	cfg, err := conf.LoadConfiguration(envVar, "../tests/config1")
	assert.NoError(t, err, "error loading configuration")

	assert.Equal(t, logger.SentryLoggingConfiguration{
		SentryDSN:         "",
		SentryEnvironment: "",
	}, conf.GetSentryLoggingConfiguration(&cfg))
}

// TestGetKafkaZerologConfiguration checks the function GetKafkaZeroLogConfiguration
func TestGetKafkaZerologConfiguration(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"
	os.Clearenv()
	mustSetEnv(t, "ACG_CONFIG", "../tests/kafka_zerolog.json")

	cfg, err := conf.LoadConfiguration(envVar, "../tests/config1")
	assert.NoError(t, err, "error loading configuration")

	assert.Equal(t, logger.KafkaZerologConfiguration{
		Broker:   "",
		Topic:    "",
		CertPath: "",
		Level:    "",
	}, conf.GetKafkaZerologConfiguration(&cfg))
}

// TestLoadConfigurationNoKafkaBroker test if number of configured brokers are
// tested properly when no broker config exists
func TestLoadConfigurationNoKafkaBroker(t *testing.T) {
	var testDB = "test_db"
	os.Clearenv()

	// explicit database and broker configuration
	clowder.LoadedConfig = &clowder.AppConfig{
		Database: &clowder.DatabaseConfig{
			Name: testDB,
		},
		Kafka: &clowder.KafkaConfig{}, // no brokers in configuration
	}
	mustSetEnv(t, "CCX_NOTIFICATION_SERVICE_CONFIG_FILE", "../tests/config2")
	mustSetEnv(t, "ACG_CONFIG", "../tests/clowder_config.json")

	// load configuration using Clowder config
	config, err := conf.LoadConfiguration("CCX_NOTIFICATION_SERVICE_CONFIG_FILE", "../tests/config2")
	assert.NoError(t, err, "Failed loading configuration file")

	// retrieve broker configuration that was just loaded
	brokerCfg := conf.GetKafkaBrokerConfiguration(&config)

	// check broker configuration
	assert.Equal(t, "localhost:29092", brokerCfg.Addresses)
	assert.Equal(t, "ccx_test_notifications", brokerCfg.Topic)
	assert.True(t, brokerCfg.Enabled)
}

// TestLoadConfigurationKafkaBrokerEmptyConfig test if empty broker config is
// loaded via Clowder
func TestLoadConfigurationKafkaBrokerEmptyConfig(t *testing.T) {
	var testDB = "test_db"
	os.Clearenv()

	// just one empty broker configuration
	var brokersConfig []clowder.BrokerConfig = []clowder.BrokerConfig{
		{},
	}

	// explicit database and broker configuration
	clowder.LoadedConfig = &clowder.AppConfig{
		Database: &clowder.DatabaseConfig{
			Name: testDB,
		},
		Kafka: &clowder.KafkaConfig{
			Brokers: brokersConfig},
	}
	mustSetEnv(t, "CCX_NOTIFICATION_SERVICE_CONFIG_FILE", "../tests/config2")
	mustSetEnv(t, "ACG_CONFIG", "../tests/clowder_config.json")

	// load configuration using Clowder config
	config, err := conf.LoadConfiguration("CCX_NOTIFICATION_SERVICE_CONFIG_FILE", "../tests/config2")
	assert.NoError(t, err, "Failed loading configuration file")

	// retrieve broker configuration that was just loaded
	brokerCfg := conf.GetKafkaBrokerConfiguration(&config)

	// check broker configuration
	assert.Equal(t, "", brokerCfg.Addresses)
	assert.Equal(t, "ccx_test_notifications", brokerCfg.Topic)
	assert.True(t, brokerCfg.Enabled)
}

// TestLoadConfigurationKafkaBrokerNoPort test loading broker configuration w/o port
func TestLoadConfigurationKafkaBrokerNoPort(t *testing.T) {
	var testDB = "test_db"
	os.Clearenv()

	// just one non-empty broker configuration
	var brokersConfig []clowder.BrokerConfig = []clowder.BrokerConfig{
		{
			Hostname: "test",
			Port:     nil}, // port is not set
	}

	// explicit database and broker configuration
	clowder.LoadedConfig = &clowder.AppConfig{
		Database: &clowder.DatabaseConfig{
			Name: testDB,
		},
		Kafka: &clowder.KafkaConfig{
			Brokers: brokersConfig},
	}
	mustSetEnv(t, "CCX_NOTIFICATION_SERVICE_CONFIG_FILE", "../tests/config2")
	mustSetEnv(t, "ACG_CONFIG", "../tests/clowder_config.json")

	// load configuration using Clowder config
	config, err := conf.LoadConfiguration("CCX_NOTIFICATION_SERVICE_CONFIG_FILE", "../tests/config2")
	assert.NoError(t, err, "Failed loading configuration file")

	// retrieve broker configuration that was just loaded
	brokerCfg := conf.GetKafkaBrokerConfiguration(&config)

	// check broker configuration
	// no port should be set
	assert.Equal(t, "test", brokerCfg.Addresses)
	assert.Equal(t, "ccx_test_notifications", brokerCfg.Topic)
	assert.True(t, brokerCfg.Enabled)
}

// TestLoadConfigurationKafkaBrokerPort test loading broker port
func TestLoadConfigurationKafkaBrokerPort(t *testing.T) {
	var testDB = "test_db"
	os.Clearenv()

	var port = 1234

	// just one non-empty broker configuration
	var brokersConfig []clowder.BrokerConfig = []clowder.BrokerConfig{
		{
			Hostname: "test",
			Port:     &port}, // port is set
	}

	// explicit database and broker configuration
	clowder.LoadedConfig = &clowder.AppConfig{
		Database: &clowder.DatabaseConfig{
			Name: testDB,
		},
		Kafka: &clowder.KafkaConfig{
			Brokers: brokersConfig},
	}
	mustSetEnv(t, "CCX_NOTIFICATION_SERVICE_CONFIG_FILE", "../tests/config2")
	mustSetEnv(t, "ACG_CONFIG", "../tests/clowder_config.json")

	// load configuration using Clowder config
	config, err := conf.LoadConfiguration("CCX_NOTIFICATION_SERVICE_CONFIG_FILE", "../tests/config2")
	assert.NoError(t, err, "Failed loading configuration file")

	// retrieve broker configuration that was just loaded
	brokerCfg := conf.GetKafkaBrokerConfiguration(&config)

	// check broker configuration
	assert.Equal(t, "test:1234", brokerCfg.Addresses)
	assert.Equal(t, "ccx_test_notifications", brokerCfg.Topic)
	assert.True(t, brokerCfg.Enabled)
}

// TestLoadConfigurationKafkaBrokerAuthConfigMissingSASL test loading broker auth. config
// is correct but missing SASL configuration
func TestLoadConfigurationKafkaBrokerAuthConfigMissingSASL(t *testing.T) {
	var testDB = "test_db"
	os.Clearenv()

	var port = 1234
	var authType clowder.BrokerConfigAuthtype

	var brokersConfig []clowder.BrokerConfig = []clowder.BrokerConfig{
		{
			Hostname: "test",
			Port:     &port,
			Authtype: &authType,
		},
	}

	// explicit database and broker configuration
	clowder.LoadedConfig = &clowder.AppConfig{
		Database: &clowder.DatabaseConfig{
			Name: testDB,
		},
		Kafka: &clowder.KafkaConfig{
			Brokers: brokersConfig,
		},
	}
	mustSetEnv(t, "CCX_NOTIFICATION_SERVICE_CONFIG_FILE", "../tests/config2")
	mustSetEnv(t, "ACG_CONFIG", "../tests/clowder_config.json")

	// load configuration using Clowder config
	config, err := conf.LoadConfiguration("CCX_NOTIFICATION_SERVICE_CONFIG_FILE", "../tests/config2")
	assert.NoError(t, err, "Failed loading configuration file")

	// retrieve broker configuration that was just loaded
	brokerCfg := conf.GetKafkaBrokerConfiguration(&config)

	// check broker configuration
	assert.Equal(t, "test:1234", brokerCfg.Addresses)
	assert.Equal(t, "ccx_test_notifications", brokerCfg.Topic)
	assert.True(t, brokerCfg.Enabled)

	// additionally SASL config should be set too
	assert.Equal(t, "", brokerCfg.SaslUsername)
	assert.Equal(t, "", brokerCfg.SaslPassword)
	assert.Equal(t, "", brokerCfg.SaslMechanism)
	assert.Equal(t, "", brokerCfg.SecurityProtocol)
}

// TestLoadConfigurationKafkaBrokerAuthConfig test loading broker auth. config
// is correct
func TestLoadConfigurationKafkaBrokerAuthConfig(t *testing.T) {
	var testDB = "test_db"
	os.Clearenv()

	var port = 1234
	var authType clowder.BrokerConfigAuthtype

	var username = "username"
	var password = "password"
	var saslMechanism = "mechanism"
	var securityProtocol = "security_protocol"

	var brokersConfig []clowder.BrokerConfig = []clowder.BrokerConfig{
		{
			Hostname: "test",
			Port:     &port,
			Authtype: &authType,
			// proper SASL configuration
			Sasl: &clowder.KafkaSASLConfig{
				Username:         &username,
				Password:         &password,
				SaslMechanism:    &saslMechanism,
				SecurityProtocol: &securityProtocol},
		},
	}

	// explicit database and broker configuration
	clowder.LoadedConfig = &clowder.AppConfig{
		Database: &clowder.DatabaseConfig{
			Name: testDB,
		},
		Kafka: &clowder.KafkaConfig{
			Brokers: brokersConfig,
		},
	}
	mustSetEnv(t, "CCX_NOTIFICATION_SERVICE_CONFIG_FILE", "../tests/config2")
	mustSetEnv(t, "ACG_CONFIG", "../tests/clowder_config.json")

	// load configuration using Clowder config
	config, err := conf.LoadConfiguration("CCX_NOTIFICATION_SERVICE_CONFIG_FILE", "../tests/config2")
	assert.NoError(t, err, "Failed loading configuration file")

	// retrieve broker configuration that was just loaded
	brokerCfg := conf.GetKafkaBrokerConfiguration(&config)

	// check broker configuration
	assert.Equal(t, "test:1234", brokerCfg.Addresses)
	assert.Equal(t, "ccx_test_notifications", brokerCfg.Topic)
	assert.True(t, brokerCfg.Enabled)

	// additionally SASL config should be set too
	assert.Equal(t, username, brokerCfg.SaslUsername)
	assert.Equal(t, password, brokerCfg.SaslPassword)
	assert.Equal(t, saslMechanism, brokerCfg.SaslMechanism)
	assert.Equal(t, securityProtocol, brokerCfg.SecurityProtocol)
}

// TestLoadConfigurationKafkaTopicUpdatedFromClowder tests that when applying the config,
// if the Clowder config is enabled, the Kafka topics are replaced by the ones defined in
// LoadedConfig.Kafka.Topics if found, and used as-is if not.
func TestLoadConfigurationKafkaTopicUpdatedFromClowder(t *testing.T) {
	os.Clearenv()
	hostname := "kafka"
	port := 9092
	topicName := "ccx_test_notifications"
	newTopicName := "the.clowder.kafka.topic"

	// explicit database and broker config
	clowder.LoadedConfig = &clowder.AppConfig{
		Kafka: &clowder.KafkaConfig{
			Brokers: []clowder.BrokerConfig{
				{
					Hostname: hostname,
					Port:     &port,
				},
			},
		},
	}

	clowder.KafkaTopics = make(map[string]clowder.TopicConfig)
	clowder.KafkaTopics[topicName] = clowder.TopicConfig{
		Name:          newTopicName,
		RequestedName: topicName,
	}

	mustSetEnv(t, "ACG_CONFIG", "../tests/clowder_config.json")

	config, err := conf.LoadConfiguration("CCX_NOTIFICATION_SERVICE_CONFIG_FILE", "../tests/config2")
	assert.NoError(t, err, "Failed loading configuration file")

	brokerCfg := conf.GetKafkaBrokerConfiguration(&config)
	assert.Equal(t, fmt.Sprintf("%s:%d", hostname, port), brokerCfg.Addresses)
	assert.Equal(t, newTopicName, brokerCfg.Topic)

	// config with different broker configuration, broker's hostname taken from clowder, but no topic to map to
	topicName = "test_notification_topic"

	config, err = conf.LoadConfiguration("CCX_NOTIFICATION_SERVICE_CONFIG_FILE", "../tests/config1")
	assert.NoError(t, err, "Failed loading configuration file")

	brokerCfg = conf.GetKafkaBrokerConfiguration(&config)
	assert.Equal(t, fmt.Sprintf("%s:%d", hostname, port), brokerCfg.Addresses)
	assert.Equal(t, topicName, brokerCfg.Topic)
}

// TestGetStorageConfigurationIsImmutable checks if function
// GetStorageConfiguration is not mutable
func TestGetStorageConfigurationIsImmutable(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"
	mustSetEnv(t, envVar, "tests/config2")

	// load configuration with check if loading was ok
	config, err := conf.LoadConfiguration(envVar, "")
	assert.Nil(t, err, "Failed loading configuration file from env var!")

	// clone the configuration
	origConfig := config

	// call the tested function
	conf.GetStorageConfiguration(&config)

	// and compare original configuration with possibly mutated one
	assert.Equal(t, config, origConfig, "GetStorageConfiguration must not be mutable")
}

// TestGetLoggingConfigurationIsImmutable checks if function
// GetLoggingConfiguration is not mutable
func TestGetLoggingConfigurationIsImmutable(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"
	mustSetEnv(t, envVar, "tests/config2")

	// load configuration with check if loading was ok
	config, err := conf.LoadConfiguration(envVar, "")
	assert.Nil(t, err, "Failed loading configuration file from env var!")

	// clone the configuration
	origConfig := config

	// call the tested function
	conf.GetLoggingConfiguration(&config)

	// and compare original configuration with possibly mutated one
	assert.Equal(t, config, origConfig, "GetLoggingConfiguration must not be mutable")
}

// TestGetKafkaBrokerConfigurationIsImmutable checks if function
// GetKafkaBrokerConfiguration is not mutable
func TestGetKafkaBrokerConfigurationIsImmutable(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"
	mustSetEnv(t, envVar, "tests/config2")

	// load configuration with check if loading was ok
	config, err := conf.LoadConfiguration(envVar, "")
	assert.Nil(t, err, "Failed loading configuration file from env var!")

	// clone the configuration
	origConfig := config

	// call the tested function
	conf.GetKafkaBrokerConfiguration(&config)

	// and compare original configuration with possibly mutated one
	assert.Equal(t, config, origConfig, "GetKafkaBrokerConfiguration must not be mutable")
}

// TestGetMetricsConfigurationIsImmutable checks if function
// GetMetricsConfiguration is not mutable
func TestGetMetricsConfigurationIsImmutable(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"
	mustSetEnv(t, envVar, "tests/config2")

	// load configuration with check if loading was ok
	config, err := conf.LoadConfiguration(envVar, "")
	assert.Nil(t, err, "Failed loading configuration file from env var!")

	// clone the configuration
	origConfig := config

	// call the tested function
	conf.GetMetricsConfiguration(&config)

	// and compare original configuration with possibly mutated one
	assert.Equal(t, config, origConfig, "GetMetricsConfiguration must not be mutable")
}

// TestGetServiceLogConfigurationIsImmutable checks if function
// GetServiceLogConfiguration is not mutable
func TestGetServiceLogConfigurationIsImmutable(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"
	mustSetEnv(t, envVar, "tests/config2")

	// load configuration with check if loading was ok
	config, err := conf.LoadConfiguration(envVar, "")
	assert.Nil(t, err, "Failed loading configuration file from env var!")

	// clone the configuration
	origConfig := config

	// call the tested function
	conf.GetServiceLogConfiguration(&config)

	// and compare original configuration with possibly mutated one
	assert.Equal(t, config, origConfig, "GetServiceLogConfiguration must not be mutable")
}

// TestGetDependenciesConfigurationIsImmutable checks if function
// GetDependenciesConfiguration is not mutable
func TestGetDependenciesConfigurationIsImmutable(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"
	mustSetEnv(t, envVar, "tests/config2")

	// load configuration with check if loading was ok
	config, err := conf.LoadConfiguration(envVar, "")
	assert.Nil(t, err, "Failed loading configuration file from env var!")

	// clone the configuration
	origConfig := config

	// call the tested function
	conf.GetDependenciesConfiguration(&config)

	// and compare original configuration with possibly mutated one
	assert.Equal(t, config, origConfig, "GetDependenciesConfiguration must not be mutable")
}

// TestGetNotificationsConfigurationIsImmutable checks if function
// GetNotificationsConfiguration is not mutable
func TestGetNotificationsConfigurationIsImmutable(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"
	mustSetEnv(t, envVar, "tests/config2")

	// load configuration with check if loading was ok
	config, err := conf.LoadConfiguration(envVar, "")
	assert.Nil(t, err, "Failed loading configuration file from env var!")

	// clone the configuration
	origConfig := config

	// call the tested function
	conf.GetNotificationsConfiguration(&config)

	// and compare original configuration with possibly mutated one
	assert.Equal(t, config, origConfig, "GetNotificationsConfiguration must not be mutable")
}

// TestGetCleanerConfigurationIsImmutable checks if function
// GetCleanerConfiguration is not mutable
func TestGetCleanerConfigurationIsImmutable(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"
	mustSetEnv(t, envVar, "tests/config2")

	// load configuration with check if loading was ok
	config, err := conf.LoadConfiguration(envVar, "")
	assert.Nil(t, err, "Failed loading configuration file from env var!")

	// clone the configuration
	origConfig := config

	// call the tested function
	conf.GetCleanerConfiguration(&config)

	// and compare original configuration with possibly mutated one
	assert.Equal(t, config, origConfig, "GetCleanerConfiguration must not be mutable")
}

// TestGetProcessingConfigurationIsImmutable checks if function
// GetProcessingConfiguration is not mutable
func TestGetProcessingConfigurationIsImmutable(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"
	mustSetEnv(t, envVar, "tests/config2")

	// load configuration with check if loading was ok
	config, err := conf.LoadConfiguration(envVar, "")
	assert.Nil(t, err, "Failed loading configuration file from env var!")

	// clone the configuration
	origConfig := config

	// call the tested function
	conf.GetProcessingConfiguration(&config)

	// and compare original configuration with possibly mutated one
	assert.Equal(t, config, origConfig, "GetProcessingConfiguration must not be mutable")
}

// TestCreateURLPositiveCase checks the function createURL when correct data to
// create valid URL are used
func TestCreateURLPositiveCase(t *testing.T) {
	url, err := conf.CreateURL("http://foo", "bar")
	assert.Nil(t, err)
	assert.Equal(t, "http://foo/bar", url, "Unexpected URL returned")
}

// TestCreateURLNegativeCase checks the function createURL when wrong server is used
func TestCreateURLNegativeCase(t *testing.T) {
	_, err := conf.CreateURL(":\\", "\\")
	assert.NotNil(t, err)
}
