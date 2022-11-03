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

package main_test

// Benchmark for config module

import (
	"os"
	"testing"

	conf "github.com/RedHatInsights/ccx-notification-service/conf"
)

// Configuration-related constants
const (
	configFileEnvName = "CCX_NOTIFICATION_SERVICE_CONFIG_FILE"
	configFileName    = "tests/benchmark"
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
