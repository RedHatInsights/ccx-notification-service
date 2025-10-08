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

package differ_test

// Documentation in literate-programming-style is available at:
// https://redhatinsights.github.io/ccx-notification-writer/packages/differ/metrics_test.html

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"bytes"

	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"

	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/differ"
)

// TestAddMetricsWithNamespaceAndSubsystem function checks the basic behaviour of function
// AddMetricsWithNamespaceAndSubsystem from `metrics.go`
func TestAddMetricsWithNamespaceAndSubsystem(t *testing.T) {
	// add all metrics into the namespace "foobar"
	differ.AddMetricsWithNamespaceAndSubsystem("foo", "bar")

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
	assert.NotNil(t, differ.NoSeverityTotalRisk)
}

func TestPushMetricsNoNamespaceConfig(t *testing.T) {
	var expectedPushes = 0
	testServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", `text/plain; charset=utf-8`)
			w.WriteHeader(http.StatusOK)
			expectedPushes++
		}),
	)
	defer testServer.Close()

	metricsConf := conf.MetricsConfiguration{
		Job:              "ccx_notification_service",
		GatewayURL:       testServer.URL,
		GatewayAuthToken: "some token",
	}

	err := differ.PushMetrics(&metricsConf)
	assert.Nil(t, err)

	assert.Zero(t, expectedPushes,
		fmt.Sprintf("expected exactly %d pushes", expectedPushes))
}

func TestPushMetricsGatewayNoAuthConfig(t *testing.T) {
	var expectedPushes = 0
	testServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", `text/plain; charset=utf-8`)
			w.WriteHeader(http.StatusOK)
			expectedPushes++
		}),
	)
	defer testServer.Close()

	metricsConf := conf.MetricsConfiguration{
		Job:        "ccx_notification_service",
		Namespace:  "ccx_notification_service",
		GatewayURL: testServer.URL,
	}

	err := differ.PushMetrics(&metricsConf)
	assert.Nil(t, err)

	assert.Zero(t, expectedPushes,
		fmt.Sprintf("expected exactly %d pushes", expectedPushes))
}

func TestPushMetricsGatewayNotFailingWithRetriesThenOk(t *testing.T) {
	var (
		pushes             int
		expectedPushes     = 6
		timeBetweenRetries = 200 * time.Millisecond // 0.5s
		totalTime          = 2 * time.Second        // give enough time
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
		Job:              "ccx_notification_service",
		Namespace:        "ccx_notification_service",
		GatewayURL:       testServer.URL,
		GatewayAuthToken: "some_token",
		RetryAfter:       timeBetweenRetries,
		Retries:          10,
	}

	err := differ.PushMetrics(&metricsConf)
	assert.Nil(t, err)
	cancel()

	log.Info().Int("pushes", pushes).Msg("debug")

	assert.Equal(t, expectedPushes, pushes,
		fmt.Sprintf("expected exactly %d retries, but received %d", expectedPushes, pushes))
}

func TestPushMetricsGatewayNotFailingWithRetries(t *testing.T) {
	var (
		pushes             int
		expectedPushes     = 1
		timeBetweenRetries = 100 * time.Millisecond // 0.1s
		totalTime          = 500 * time.Millisecond // give enough time
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
		Job:              "ccx_notification_service",
		Namespace:        "ccx_notification_service",
		GatewayURL:       testServer.URL,
		GatewayAuthToken: "some_token",
		RetryAfter:       timeBetweenRetries,
		Retries:          10,
	}

	err := differ.PushMetrics(&metricsConf)
	assert.Nil(t, err)
	cancel()

	log.Info().Int("pushes", pushes).Msg("debug")

	assert.Equal(t, expectedPushes, pushes,
		fmt.Sprintf("expected exactly %d retries, but received %d", expectedPushes, pushes))
}

func TestPushMetricsGatewayFailingWarnings(t *testing.T) {
	var timeBetweenRetries = 50 * time.Millisecond

	testCases := []struct {
		name           string
		retries        int
		expectedLogMsg string
	}{
		{
			name:           "Retries zero, single failure warning",
			retries:        0,
			expectedLogMsg: "Metrics push failed, but continuing execution.",
		},
		{
			name:           "Retries non-zero, final failure warning",
			retries:        10,
			expectedLogMsg: "All metric push attempts failed, but continuing execution.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pushes := 0

			testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				pushes++
				w.WriteHeader(http.StatusBadGateway)
			}))
			defer testServer.Close()

			var buf bytes.Buffer
			log.Logger = log.Output(&buf)

			metricsConf := conf.MetricsConfiguration{
				Job:              "ccx_notification_service",
				Namespace:        "ccx_notification_service",
				GatewayURL:       testServer.URL,
				GatewayAuthToken: "some_token",
				RetryAfter:       timeBetweenRetries,
				Retries:          tc.retries,
			}

			err := differ.PushMetrics(&metricsConf)
			assert.Nil(t, err)

			logOutput := buf.String()
			assert.Contains(t, logOutput, tc.expectedLogMsg)
		})
	}
}


func TestPushMetricsInLoop(t *testing.T) {
	// Fake a Pushgateway that responds with 202 to DELETE and with 200 in
	// all other cases and counts the number of pushes received
	var (
		pushes          int
		expectedPushes  = 5 // at least
		timeBetweenPush = 100 * time.Millisecond
		totalTime       = 1 * time.Second // give enough time
	)

	pgwOK := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", `text/plain; charset=utf-8`)
			w.WriteHeader(http.StatusOK)
			pushes++
		}),
	)
	defer pgwOK.Close()

	metricsConf := conf.MetricsConfiguration{
		Job:                    "ccx_notification_service",
		Namespace:              "ccx_notification_service",
		GatewayURL:             pgwOK.URL,
		GatewayAuthToken:       "some_token",
		GatewayTimeBetweenPush: timeBetweenPush,
	}

	ctx, cancel := context.WithTimeout(context.Background(), totalTime)

	go differ.PushMetricsInLoop(ctx, &metricsConf)
	time.Sleep(totalTime)
	cancel()

	assert.GreaterOrEqual(t, pushes, expectedPushes, fmt.Sprintf("expected more than %d pushes but found %d", expectedPushes, pushes))
}
