/*
Copyright © 2021 Red Hat, Inc.

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
// https://redhatinsights.github.io/ccx-notification-writer/packages/differ/metrics_test.html

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/differ"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
)

// TestAddMetricsWithNamespace function checks the basic behaviour of function
// AddMetricsWithNamespace from `metrics.go`
func TestAddMetricsWithNamespace(t *testing.T) {
	// add all metrics into the namespace "foobar"
	differ.AddMetricsWithNamespace("foobar")

	// check the registration
	assert.NotNil(t, differ.FetchContentErrors)
	assert.NotNil(t, differ.ReadClusterListErrors)
	assert.NotNil(t, differ.ReadReportedErrors)
	assert.NotNil(t, differ.ProducerSetupErrors)
	assert.NotNil(t, differ.StorageSetupErrors)
	assert.NotNil(t, differ.ReadReportForClusterErrors)
	assert.NotNil(t, differ.DeserializeReportErrors)
	assert.NotNil(t, differ.ReportWithHighImpact)
	assert.NotNil(t, differ.NotificationNotSentSameState)
	assert.NotNil(t, differ.NotificationNotSentErrorState)
	assert.NotNil(t, differ.NotificationSent)
}

// TODO: TestPushMetrics

func TestPushMetricsGatewayNotFailingWithRetriesThenOk(t *testing.T) {
	var (
		pushes          int
		expectedPushes  = 6
		timeBetweenPush = 200 * time.Millisecond // 0.5s
		totalTime       = 2 * time.Second        // give enough time
	)

	testServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", `text/plain; charset=utf-8`)
			if pushes >= 5 {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusBadGateway)
			}
			pushes++
		}),
	)
	defer testServer.Close()

	_, cancel := context.WithTimeout(context.Background(), totalTime)

	metricsConf := conf.MetricsConfiguration{
		Job:        "ccx_notification_service",
		Namespace:  "ccx_notification_service",
		GatewayURL: testServer.URL,
		RetryAfter: timeBetweenPush,
		Retries:    10,
	}

	go differ.PushMetrics(metricsConf)

	time.Sleep(totalTime)
	cancel()

	log.Info().Int("pushes", pushes).Msg("debug")

	assert.Equal(t, expectedPushes, pushes,
		fmt.Sprintf("expected exactly %d retries, but received %d", expectedPushes, pushes))
}

func TestPushMetricsGatewayNotFailingWithRetries(t *testing.T) {
	var (
		pushes          int
		expectedPushes  = 1
		timeBetweenPush = 100 * time.Millisecond // 0.1s
		totalTime       = 500 * time.Millisecond // give enough time
	)

	testServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", `text/plain; charset=utf-8`)
			w.WriteHeader(http.StatusOK)
			pushes++
		}),
	)
	defer testServer.Close()

	_, cancel := context.WithTimeout(context.Background(), totalTime)

	metricsConf := conf.MetricsConfiguration{
		Job:        "ccx_notification_service",
		Namespace:  "ccx_notification_service",
		GatewayURL: testServer.URL,
		RetryAfter: timeBetweenPush,
		Retries:    10,
	}

	go differ.PushMetrics(metricsConf)

	time.Sleep(totalTime)
	cancel()

	log.Info().Int("pushes", pushes).Msg("debug")

	assert.Equal(t, expectedPushes, pushes,
		fmt.Sprintf("expected exactly %d retries, but received %d", expectedPushes, pushes))
}

func TestPushMetricsGatewayFailing(t *testing.T) {
	if os.Getenv("GATEWAY_502_FAIL_ALL_RETRIES") == "1" {
		var (
			timeBetweenPush = 100 * time.Millisecond // 0.1s
			totalTime       = 1 * time.Second        // give enough time
		)

		testServer := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", `text/plain; charset=utf-8`)
				w.WriteHeader(http.StatusBadGateway)
			}),
		)
		defer testServer.Close()

		_, cancel := context.WithTimeout(context.Background(), totalTime)

		metricsConf := conf.MetricsConfiguration{
			Job:        "ccx_notification_service",
			Namespace:  "ccx_notification_service",
			GatewayURL: testServer.URL,
			RetryAfter: timeBetweenPush,
			Retries:    10,
		}

		go differ.PushMetrics(metricsConf)

		time.Sleep(totalTime)
		cancel()
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestPushMetricsGatewayFailing")
	cmd.Env = append(os.Environ(), "GATEWAY_502_FAIL_ALL_RETRIES=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && e.ExitCode() != differ.ExitStatusMetricsError {
		t.Fatalf(
			"Should exit with status ExitStatusMetricsError(%d). Got status %d",
			differ.ExitStatusMetricsError,
			e.ExitCode())
	}
}
