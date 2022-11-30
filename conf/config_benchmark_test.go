/*
Copyright © 2022 Red Hat, Inc.

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

// Benchmark for config module

import (
	"os"
	"testing"

	conf "github.com/RedHatInsights/ccx-notification-service/conf"
)

// Configuration-related constants
const (
	configFileEnvName = "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"
	configFileName    = "../tests/benchmark"
)

// loadConfiguration function loads configuration prepared to be used by
// benchmarks
func loadConfiguration() (conf.ConfigStruct, error) {
	os.Clearenv()

	err := os.Setenv(configFileEnvName, configFileName)
	if err != nil {
		return conf.ConfigStruct{}, err
	}

	config, err := conf.LoadConfiguration(configFileEnvName, configFileName)
	if err != nil {
		return conf.ConfigStruct{}, err
	}

	return config, nil
}

func mustLoadBenchmarkConfiguration(b *testing.B) conf.ConfigStruct {
	configuration, err := loadConfiguration()
	if err != nil {
		b.Fatal(err)
	}
	return configuration
}

// BenchmarkGetProcessingConfiguration measures the speed of
// GetProcessingConfiguration function from the conf module.
func BenchmarkGetProcessingConfiguration(b *testing.B) {
	configuration := mustLoadBenchmarkConfiguration(b)

	for i := 0; i < b.N; i++ {
		// call benchmarked function
		m := conf.GetProcessingConfiguration(&configuration)

		b.StopTimer()
		if !m.FilterAllowedClusters {
			b.Fatal("Wrong configuration: filter_allowed_clusters is set to false")
		}
		if !m.FilterBlockedClusters {
			b.Fatal("Wrong configuration: filter_blocked_clusters is set to false")
		}
		b.StartTimer()
	}

}

// BenchmarkGetCleanerConfiguration measures the speed of
// GetCleanerConfiguration function from the conf module.
func BenchmarkGetCleanerConfiguration(b *testing.B) {
	configuration := mustLoadBenchmarkConfiguration(b)

	for i := 0; i < b.N; i++ {
		// call benchmarked function
		m := conf.GetCleanerConfiguration(&configuration)

		b.StopTimer()
		if m.MaxAge != "90 days" {
			b.Fatal("Wrong configuration: max_age = '" + m.MaxAge + "'")
		}
		b.StartTimer()
	}

}

// BenchmarkGetNotificationsConfiguration measures the speed of
// GetNotificationsConfiguration function from the conf module.
func BenchmarkGetNotificationsConfiguration(b *testing.B) {
	configuration := mustLoadBenchmarkConfiguration(b)

	for i := 0; i < b.N; i++ {
		// call benchmarked function
		m := conf.GetNotificationsConfiguration(&configuration)

		b.StopTimer()
		if m.InsightsAdvisorURL != "https://console.redhat.com/openshift/insights/advisor/clusters/{cluster_id}" {
			b.Fatal("Wrong configuration: insights_advisor_url = '" + m.InsightsAdvisorURL + "'")
		}
		if m.ClusterDetailsURI != "https://console.redhat.com/openshift/details/{cluster_id}#insights" {
			b.Fatal("Wrong configuration: cluster_details_uri = '" + m.ClusterDetailsURI + "'")
		}
		if m.RuleDetailsURI != "https://console.redhat.com/openshift/details/{cluster_id}/insights/{module}/{error_key}" {
			b.Fatal("Wrong configuration: rule_details_uri = '" + m.RuleDetailsURI + "'")
		}
		b.StartTimer()
	}

}

// BenchmarkGetDependenciesConfiguration measures the speed of
// GetDependenciesConfiguration function from the conf module.
func BenchmarkGetDependenciesConfiguration(b *testing.B) {
	configuration := mustLoadBenchmarkConfiguration(b)

	for i := 0; i < b.N; i++ {
		// call benchmarked function
		m := conf.GetDependenciesConfiguration(&configuration)

		b.StopTimer()
		if m.ContentServiceServer != "localhost:8082" {
			b.Fatal("Wrong configuration: content service server = '" + m.ContentServiceServer + "'")
		}
		if m.ContentServiceEndpoint != "/api/v1/content" {
			b.Fatal("Wrong configuration: content service endpoint = '" + m.ContentServiceEndpoint + "'")
		}
		b.StartTimer()
	}

}

// BenchmarkGetStorageConfiguration measures the speed of
// GetStorageConfiguration function from the conf module.
func BenchmarkGetStorageConfiguration(b *testing.B) {
	configuration := mustLoadBenchmarkConfiguration(b)

	for i := 0; i < b.N; i++ {
		// call benchmarked function
		m := conf.GetStorageConfiguration(&configuration)

		b.StopTimer()
		if m.Driver != "sqlite3" {
			b.Fatal("Wrong configuration: driver = '" + m.Driver + "'")
		}
		b.StartTimer()
	}

}

// BenchmarkGetLoggingConfiguration measures the speed of
// GetLoggingConfiguration function from the conf module.
func BenchmarkGetLoggingConfiguration(b *testing.B) {
	configuration := mustLoadBenchmarkConfiguration(b)

	for i := 0; i < b.N; i++ {
		// call benchmarked function
		m := conf.GetLoggingConfiguration(&configuration)

		b.StopTimer()
		if !m.Debug {
			b.Fatal("Wrong configuration: debug is set to false")
		}
		if m.LogLevel != "" {
			b.Fatal("Wrong configuration: loglevel = '" + m.LogLevel + "'")
		}
		b.StartTimer()
	}

}

// BenchmarkGetKafkaBrokerConfiguration measures the speed of
// GetKafkaBrokerConfiguration function from the conf module.
func BenchmarkGetKafkaBrokerConfiguration(b *testing.B) {
	configuration := mustLoadBenchmarkConfiguration(b)

	for i := 0; i < b.N; i++ {
		// call benchmarked function
		m := conf.GetKafkaBrokerConfiguration(&configuration)

		b.StopTimer()
		if m.Address != "localhost:9092" {
			b.Fatal("Wrong configuration: address = '" + m.Address + "'")
		}
		if m.Cooldown != "24 hours" {
			b.Fatal("Wrong configuration: cooldown = '" + m.Cooldown + "'")
		}
		b.StartTimer()
	}

}

// BenchmarkGetServiceLogConfiguration measures the speed of
// GetServiceLogConfiguration function from the conf module.
func BenchmarkGetServiceLogConfiguration(b *testing.B) {
	configuration := mustLoadBenchmarkConfiguration(b)

	for i := 0; i < b.N; i++ {
		// call benchmarked function
		m := conf.GetServiceLogConfiguration(&configuration)

		b.StopTimer()
		if m.ClientID != "a-service-id" {
			b.Fatal("Wrong configuration: cliend ID = '" + m.ClientID + "'")
		}
		if m.Enabled {
			b.Fatal("Wrong configuration: service log is enabled")
		}
		if m.Cooldown != "0" {
			b.Fatal("Wrong configuration: cooldown = '" + m.Cooldown + "'")
		}
		b.StartTimer()
	}

}

// BenchmarkGetMetricsConfiguration measures the speed of
// GetMetricsConfiguration function from the conf module.
func BenchmarkGetMetricsConfiguration(b *testing.B) {
	configuration := mustLoadBenchmarkConfiguration(b)

	for i := 0; i < b.N; i++ {
		// call benchmarked function
		m := conf.GetMetricsConfiguration(&configuration)

		b.StopTimer()
		if m.Namespace != "ccx_notification_service" {
			b.Fatal("Wrong configuration: namespace = '" + m.Namespace + "'")
		}
		if m.Subsystem != "to_notification_backend" {
			b.Fatal("Wrong configuration: subsystem = '" + m.Subsystem + "'")
		}
		b.StartTimer()
	}

}
