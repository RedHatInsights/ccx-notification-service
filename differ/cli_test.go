/*
Copyright © 2023 Red Hat, Inc.

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

package differ_test

// Documentation in literate-programming-style is available at:
// https://redhatinsights.github.io/ccx-notification-writer/packages/differ/differ_cli_args_test.html

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/differ"
	"github.com/RedHatInsights/insights-operator-utils/logger"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/stretchr/testify/assert"

	"github.com/RedHatInsights/ccx-notification-service/types"
)

// Can't redirect zerolog/log to buffer directly in some tests due to clowder
// init
func captureStdout(f func()) string {
	originalStdOutFile := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	_ = w.Close()
	os.Stdout = originalStdOutFile

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

// Test the checkArgs function when flag for --show-version is set
func TestArgsParsingShowVersion(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=TestArgsParsingShowVersion")
	if os.Getenv("SHOW_VERSION_FLAG") == "1" {
		args := types.CliFlags{
			ShowVersion: true,
		}
		differ.CheckArgs(&args)
	}
	cmd.Env = append(os.Environ(), "SHOW_VERSION_FLAG=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		t.Fatalf("checkArgs exited with exit status %d, wanted exit status 0", e.ExitCode())
	}
}

// Test the checkArgs function when flag for --show-authors is set
func TestArgsParsingShowAuthors(t *testing.T) {
	if os.Getenv("SHOW_AUTHORS_FLAG") == "1" {
		args := types.CliFlags{
			ShowAuthors: true,
		}
		differ.CheckArgs(&args)
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestArgsParsingShowAuthors")
	cmd.Env = append(os.Environ(), "SHOW_AUTHORS_FLAG=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		t.Fatalf("checkArgs exited with exit status %d, wanted exit status 0", e.ExitCode())
	}
}

// Test the checkArgs function when no flag is set
func TestArgsParsingNoFlags(t *testing.T) {
	if os.Getenv("EXEC_NO_FLAG") == "1" {
		args := types.CliFlags{}
		differ.CheckArgs(&args)
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestArgsParsingNoFlags")
	cmd.Env = append(os.Environ(), "EXEC_NO_FLAG=1")
	err := cmd.Run()
	assert.NotNil(t, err)
	if exitError, ok := err.(*exec.ExitError); ok {
		assert.Equal(t, differ.ExitStatusConfiguration, exitError.ExitCode())
		return
	}
	t.Fatalf("checkArgs didn't behave properly. Wanted exit status %d but got error\n %v", differ.ExitStatusConfiguration, err)
}

// Test the checkArgs function when --instant-reports flag is set
func TestArgsParsingInstantReportsFlags(t *testing.T) {
	args := types.CliFlags{
		InstantReports: true,
	}
	differ.CheckArgs(&args)
	assert.Equal(t, differ.NotificationType, types.InstantNotif)
}

func TestShowVersion(t *testing.T) {
	assert.Contains(t, captureStdout(differ.ShowVersion), differ.VersionMessage, "ShowVersion function is not displaying the expected content")
}

func TestShowAuthors(t *testing.T) {
	assert.Contains(t, captureStdout(differ.ShowAuthors), differ.AuthorsMessage, "ShowAuthors function is not displaying the expected content")
}

func TestShowConfiguration(t *testing.T) {
	brokerAddr := "localhost:29092"
	brokerTopic := "test_topic"
	contentServer := "content server"
	contentEndpoint := "content endpoint"
	templateRendererServer := "template renderer server"
	templateRendererEndpoint := "template renderer endpoint"
	db := "db"
	driver := "test_driver"
	advisorURL := "an uri"
	clustersURI := "a {cluster} details uri"
	ruleURI := "a {rule} details uri"
	metricsJob := "ccx"
	metricsNamespace := "notification"
	metricsGateway := "localhost:12345"

	config := conf.ConfigStruct{
		LoggingConf: logger.LoggingConfiguration{
			Debug:    true,
			LogLevel: "info",
		},
		Storage: conf.StorageConfiguration{
			Driver:   driver,
			PGDBName: db,
		},
		Dependencies: conf.DependenciesConfiguration{
			ContentServiceServer:     contentServer,
			ContentServiceEndpoint:   contentEndpoint,
			TemplateRendererServer:   templateRendererServer,
			TemplateRendererEndpoint: templateRendererEndpoint,
		},
		Kafka: conf.KafkaConfiguration{
			Address:     brokerAddr,
			Topic:       brokerTopic,
			Timeout:     0,
			Enabled:     true,
			EventFilter: "totalRisk >= totalRiskThreshold",
		},
		Notifications: conf.NotificationsConfiguration{
			InsightsAdvisorURL: advisorURL,
			ClusterDetailsURI:  clustersURI,
			RuleDetailsURI:     ruleURI,
		},
		Metrics: conf.MetricsConfiguration{
			Job:        metricsJob,
			Namespace:  metricsNamespace,
			GatewayURL: metricsGateway,
			Retries:    4,
			RetryAfter: 1,
		},
		Cleaner: conf.CleanerConfiguration{
			MaxAge: "70 days",
		},
	}

	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.InfoLevel)
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	differ.ShowConfiguration(&config)
	output := buf.String()

	// Assert that at least one item of each struct is shown
	assert.Contains(t, output, brokerAddr)
	assert.Contains(t, output, clustersURI)
	assert.Contains(t, output, db)
	assert.Contains(t, output, templateRendererServer)
	assert.Contains(t, output, driver)
	assert.Contains(t, output, "\"Pretty colored debug logging\":true")
	assert.Contains(t, output, metricsGateway)
}

func TestDeleteOperationSpecified(t *testing.T) {
	testcases := []struct {
		cliFlags types.CliFlags
		want     bool
	}{
		{
			cliFlags: types.CliFlags{
				PrintNewReportsForCleanup: true,
				PerformNewReportsCleanup:  true,
				PrintOldReportsForCleanup: true,
				PerformOldReportsCleanup:  true,
			},
			want: true,
		},
		{
			cliFlags: types.CliFlags{
				PrintNewReportsForCleanup: false,
				PerformNewReportsCleanup:  false,
				PrintOldReportsForCleanup: false,
				PerformOldReportsCleanup:  false,
			},
			want: false,
		},
		{
			cliFlags: types.CliFlags{
				PrintNewReportsForCleanup: true,
				PerformNewReportsCleanup:  false,
				PrintOldReportsForCleanup: false,
				PerformOldReportsCleanup:  false,
			},
			want: true,
		},
		{
			cliFlags: types.CliFlags{
				PrintNewReportsForCleanup: false,
				PerformNewReportsCleanup:  true,
				PrintOldReportsForCleanup: false,
				PerformOldReportsCleanup:  false,
			},
			want: true,
		},
		{
			cliFlags: types.CliFlags{
				PrintNewReportsForCleanup: false,
				PerformNewReportsCleanup:  false,
				PrintOldReportsForCleanup: true,
				PerformOldReportsCleanup:  false,
			},
			want: true,
		},
		{
			cliFlags: types.CliFlags{
				PrintNewReportsForCleanup: false,
				PerformNewReportsCleanup:  false,
				PrintOldReportsForCleanup: false,
				PerformOldReportsCleanup:  true,
			},
			want: true,
		},
	}

	for _, tc := range testcases {
		if tc.want {
			assert.True(t, differ.DeleteOperationSpecified(tc.cliFlags), "a DELETE operation was specified in the CLI flags")
		} else {
			assert.False(t, differ.DeleteOperationSpecified(tc.cliFlags), "a DELETE operation wasn't specified in the CLI flags")
		}
	}
}
