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

package conf_test

import (
	"os"
	"time"

	"testing"

	"github.com/RedHatInsights/insights-operator-utils/tests/helpers"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	conf "github.com/RedHatInsights/ccx-notification-service/conf"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
}

func mustLoadConfiguration(envVar string) {
	_, err := conf.LoadConfiguration(envVar, "../tests/config1")
	if err != nil {
		panic(err)
	}
}

func mustSetEnv(t *testing.T, key, val string) {
	err := os.Setenv(key, val)
	helpers.FailOnError(t, err)
}

// TestLoadDefaultConfiguration loads a configuration file for testing
func TestLoadDefaultConfiguration(t *testing.T) {
	os.Clearenv()
	mustLoadConfiguration("nonExistingEnvVar")
}

// TestLoadConfigurationFromEnvVariable tests loading the config. file for testing from an environment variable
func TestLoadConfigurationFromEnvVariable(t *testing.T) {
	os.Clearenv()

	mustSetEnv(t, "CCX_NOTIFICATION_SERVICE_CONFIG_FILE", "../tests/config2")
	mustLoadConfiguration("CCX_NOTIFICATION_SERVICE_CONFIG_FILE")
}

// TestLoadConfigurationNonEnvVarUnknownConfigFile tests loading an unexisting config file when no environment variable is provided
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

	brokerCfg := conf.GetKafkaBrokerConfiguration(config)

	assert.True(t, brokerCfg.Enabled)
	assert.Equal(t, "localhost:29092", brokerCfg.Address)
	assert.Equal(t, "ccx_test_notifications", brokerCfg.Topic)
	assert.Equal(t, expectedTimeout, brokerCfg.Timeout)
}

// TestLoadServiceLogConfiguration tests loading the ServiceLog configuration
// sub-tree
func TestLoadServiceLogConfiguration(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"

	mustSetEnv(t, envVar, "../tests/config2")
	config, err := conf.LoadConfiguration(envVar, "")
	assert.Nil(t, err, "Failed loading configuration file from env var!")

	serviceLogCfg := conf.GetServiceLogConfiguration(config)

	assert.False(t, serviceLogCfg.Enabled)
	assert.Equal(t, 3, serviceLogCfg.TotalRiskThreshold)
}

// TestLoadStorageConfiguration tests loading the storage configuration sub-tree
func TestLoadStorageConfiguration(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"
	mustSetEnv(t, envVar, "../tests/config2")
	config, err := conf.LoadConfiguration(envVar, "")
	assert.Nil(t, err, "Failed loading configuration file from env var!")

	storageCfg := conf.GetStorageConfiguration(config)

	assert.Equal(t, "sqlite3", storageCfg.Driver)
	assert.Equal(t, "user", storageCfg.PGUsername)
	assert.Equal(t, "password", storageCfg.PGPassword)
	assert.Equal(t, "localhost", storageCfg.PGHost)
	assert.Equal(t, 5432, storageCfg.PGPort)
	assert.Equal(t, "notifications", storageCfg.PGDBName)
	assert.Equal(t, "", storageCfg.PGParams)
	assert.Equal(t, true, storageCfg.LogSQLQueries)
}

// TestLoadLoggingConfiguration tests loading the logging configuration sub-tree
func TestLoadLoggingConfiguration(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"
	mustSetEnv(t, envVar, "../tests/config2")
	config, err := conf.LoadConfiguration(envVar, "")
	assert.Nil(t, err, "Failed loading configuration file from env var!")

	loggingCfg := conf.GetLoggingConfiguration(config)

	assert.Equal(t, true, loggingCfg.Debug)
	assert.Equal(t, "", loggingCfg.LogLevel)
}

// TestLoadDependenciesConfiguration tests loading the dependencies configuration sub-tree
func TestLoadDependenciesConfiguration(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"
	mustSetEnv(t, envVar, "../tests/config2")
	config, err := conf.LoadConfiguration(envVar, "")
	assert.Nil(t, err, "Failed loading configuration file from env var!")

	depsCfg := conf.GetDependenciesConfiguration(config)

	assert.Equal(t, ":8081", depsCfg.ContentServiceEndpoint)
}

// TestLoadNotificationsConfiguration tests loading the notifications configuration sub-tree
func TestLoadNotificationsConfiguration(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"

	mustSetEnv(t, envVar, "../tests/config2")
	config, err := conf.LoadConfiguration(envVar, "")
	assert.Nil(t, err, "Failed loading configuration file from env var!")

	conf := conf.GetNotificationsConfiguration(config)

	assert.Equal(t, "url_to_specific_rule", conf.RuleDetailsURI)
	assert.Equal(t, "url_to_advisor", conf.InsightsAdvisorURL)
	assert.Equal(t, "url_to_specific_cluster", conf.ClusterDetailsURI)
}

// TestLoadConfigurationFromEnvVariableClowderEnabled tests loading the config.
// file for testing from an environment variable. Clowder config is enabled in
// this case.
func TestLoadConfigurationFromEnvVariableClowderEnabled(t *testing.T) {
	os.Clearenv()

	mustSetEnv(t, "CCX_NOTIFICATION_SERVICE_CONFIG_FILE", "tests/config2")
	mustSetEnv(t, "ACG_CONFIG", "tests/clowder_config.json")
	mustLoadConfiguration("CCX_NOTIFICATION_SERVICE_CONFIG_FILE")
}

// TestLoadNotificationsConfiguration tests loading the notifications configuration sub-tree
func TestLoadMetricsConfiguration(t *testing.T) {
	envVar := "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"

	mustSetEnv(t, envVar, "../tests/config2")
	config, err := conf.LoadConfiguration(envVar, "")
	assert.Nil(t, err, "Failed loading configuration file from env var!")

	conf := conf.GetMetricsConfiguration(config)

	assert.Equal(t, "ccx_notification_service_namespace", conf.Namespace)
	assert.Equal(t, ":9091", conf.GatewayURL)
	assert.Equal(t, "", conf.GatewayAuthToken)
}
