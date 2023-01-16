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

package servicelog

// Documentation in literate-programming-style is available at:
// https://redhatinsights.github.io/ccx-notification-service/packages/producer/servicelog/service_log_producer_test.html

import (
	"encoding/json"
	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/tests/mocks"
	"github.com/RedHatInsights/ccx-notification-service/types"
	"github.com/RedHatInsights/insights-operator-utils/tests/helpers"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func setupMockOCMGateway() *mocks.OCMClient {
	gateway := mocks.OCMClient{}
	gateway.On("GetTokens",
		mock.AnythingOfType("time.Duration")).Return(
		func(delay time.Duration) string {
			return "online_token"
		},
		func(delay time.Duration) string {
			return "refresh_token"
		},
		func(delay time.Duration) error {
			return nil
		},
	)
	return &gateway
}
func TestServiceLogProducerNew(t *testing.T) {
	gateway := setupMockOCMGateway()
	config := conf.ConfigStruct{
		ServiceLog: conf.ServiceLogConfiguration{
			ClientID:     "test_id",
			ClientSecret: "test_secret",
			Enabled:      true,
			URL:          "http://testserver:8000/",
			Timeout:      15 * time.Second,
		},
	}

	producer, err := New(conf.GetServiceLogConfiguration(&config), gateway)
	helpers.FailOnError(t, err)

	assert.Equal(t, producer.Configuration, conf.GetServiceLogConfiguration(&config))
	assert.Equal(t, producer.AccessToken, "online_token")
	assert.Equal(t, producer.TokenRefreshmentStartDelay, time.Second)
	assert.Equal(t, producer.TokenRefreshmentDelay, time.Second)
	assert.Equal(t, producer.TokenRefreshmentThreshold, 30*time.Second)
}

func TestServiceLogProducerSendMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer online_token" {
			t.Errorf("Expected 'Authorization: Bearer token' header, got: '%s'", r.Header.Get("Authorization"))
		}

		// read request body with message sent by our code
		body, err := io.ReadAll(r.Body)
		helpers.FailOnError(t, err)

		// try to unmarshal the message
		var message types.ServiceLogEntry
		err = json.Unmarshal(body, &message)
		helpers.FailOnError(t, err)

		// check message content
		assert.Equal(t, message.ClusterUUID, types.ClusterName("e1a379e4-9ac5-4353-8f82-ad066a734f18"))
		assert.Equal(t, message.ServiceName, "test-service-name")
		assert.Equal(t, message.Description, "test-description")
		assert.Equal(t, message.Summary, "test-summary")
		assert.Equal(t, message.CreatedBy, "test-service-created-by")
		assert.Equal(t, message.Username, "test-service-username")

		w.WriteHeader(http.StatusCreated)
		_, err = w.Write([]byte(`{"id":"2DnciRjDYKGD0gU0pipXq9lFHGD","kind":"ClusterLog","href":"/api/service_logs/v1/cluster_logs/2DnciRjDYKGD0gU0pipXq9lFHGD","timestamp":"2022-08-24T10:53:35.375948253Z","severity":"Info","service_name":"test","cluster_uuid":"e1a379e4-9ac5-4353-8f82-ad066a734f18","summary":"test","description":"test","event_stream_id":"2DnciRaUyJQDY9mggxeiidkSWp0","created_by":"test-service-created-by","username":"test-service-username","created_at":"2022-08-24T10:53:35.375972704Z","email":"test@test.com"}`))
		if err != nil {
			log.Fatal().Msg(err.Error())
		}
	}))
	defer server.Close()

	config := conf.ServiceLogConfiguration{
		Enabled: true,
		URL:     server.URL,
		Timeout: 15 * time.Second,
	}

	producer := Producer{
		Configuration: config,
		AccessToken:   "online_token",
	}

	entry := types.ServiceLogEntry{
		ClusterUUID: "e1a379e4-9ac5-4353-8f82-ad066a734f18",
		Description: "test-description",
		ServiceName: "test-service-name",
		Summary:     "test-summary",
		CreatedBy:   "test-service-created-by",
		Username:    "test-service-username",
	}
	msgBytes, err := json.Marshal(entry)
	helpers.FailOnError(t, err)

	_, _, err = producer.ProduceMessage(msgBytes)
	helpers.FailOnError(t, err)
}

func TestServiceLogProducerInvalidMessage(t *testing.T) {
	serviceLogServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer online_token" {
			t.Errorf("Expected 'Authorization: Bearer token' header, got: '%s'", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"id":"21","kind":"Error","href":"/api/service_logs/v1/errors/21","code":"OCM-CA-21","reason":"json: cannot unmarshal string into Go value of type openapi.ClusterLog","operation_id":"2DnejRUk4UfCwsDqhupK7lqxkzF"}`))
		if err != nil {
			log.Fatal().Msg(err.Error())
		}
	}))
	defer serviceLogServer.Close()

	config := conf.ServiceLogConfiguration{
		Enabled: true,
		URL:     serviceLogServer.URL,
		Timeout: 15 * time.Second,
	}

	producer := Producer{
		Configuration: config,
		AccessToken:   "online_token",
	}

	msgBytes, err := json.Marshal("nonsense")
	helpers.FailOnError(t, err)

	_, _, err = producer.ProduceMessage(msgBytes)
	assert.EqualError(t, err, "received unexpected response status code - 400 Bad Request")
}

func TestServiceLogProducerTooLongSummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer online_token" {
			t.Errorf("Expected 'Authorization: Bearer token' header, got: '%s'", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(` {"id":"9","kind":"Error","href":"/api/service_logs/v1/errors/9","code":"OCM-CA-9","reason":"Unable to create ServiceLog: pq: value too long for type character varying(255)","operation_id":"2DnjyKFcq7XD7koOn5AksQ93GQf"}`))
		if err != nil {
			log.Fatal().Msg(err.Error())
		}
	}))
	defer server.Close()

	config := conf.ServiceLogConfiguration{
		Enabled: true,
		URL:     server.URL,
		Timeout: 15 * time.Second,
	}

	producer := Producer{
		Configuration: config,
		AccessToken:   "online_token",
	}

	entry := types.ServiceLogEntry{
		ClusterUUID: "e1a379e4-9ac5-4353-8f82-ad066a734f18",
		Description: "test",
		ServiceName: "test",
		CreatedBy:   "test-service",
		Summary:     "This summary is more than 255 characters long.                                                                                                                                                                                                                   ",
	}
	msgBytes, err := json.Marshal(entry)
	helpers.FailOnError(t, err)

	_, _, err = producer.ProduceMessage(msgBytes)
	assert.EqualError(t, err, "received unexpected response status code - 500 Internal Server Error")
}

func TestServiceLogProducerClose(t *testing.T) {
	producer := Producer{
		Configuration: conf.ServiceLogConfiguration{},
	}

	err := producer.Close()
	helpers.FailOnError(t, err)
}

func TestServiceLogProducerInvalidURL(t *testing.T) {
	config := conf.ServiceLogConfiguration{
		Enabled: true,
		URL:     "http://testserver:8000/",
		Timeout: 15 * time.Second,
	}

	producer := Producer{
		Configuration: config,
		AccessToken:   "online_token",
	}

	entry := types.ServiceLogEntry{
		ClusterUUID: "e1a379e4-9ac5-4353-8f82-ad066a734f18",
		Description: "test",
		ServiceName: "test",
		CreatedBy:   "test-service",
		Summary:     "test",
	}
	msgBytes, err := json.Marshal(entry)
	helpers.FailOnError(t, err)

	_, _, err = producer.ProduceMessage(msgBytes)
	assert.Error(t, err)
}

func TestServiceLogProducerOldToken(t *testing.T) {
	serviceLogServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer online_token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, err := w.Write([]byte(`{"id":"2DnciRjDYKGD0gU0pipXq9lFHGD","kind":"ClusterLog","href":"/api/service_logs/v1/cluster_logs/2DnciRjDYKGD0gU0pipXq9lFHGD","timestamp":"2022-08-24T10:53:35.375948253Z","severity":"Info","service_name":"test","cluster_uuid":"e1a379e4-9ac5-4353-8f82-ad066a734f18","summary":"test","description":"test","event_stream_id":"2DnciRaUyJQDY9mggxeiidkSWp0","created_by":"test-service","created_at":"2022-08-24T10:53:35.375972704Z","email":"test@test.com"}`))
		if err != nil {
			log.Fatal().Msg(err.Error())
		}
	}))
	defer serviceLogServer.Close()

	config := conf.ServiceLogConfiguration{
		Enabled: true,
		URL:     serviceLogServer.URL,
		Timeout: 15 * time.Second,
	}

	gateway := setupMockOCMGateway()
	producer := Producer{
		Configuration: config,
		OCMClient:     gateway,
		AccessToken:   "old_token",
	}

	entry := types.ServiceLogEntry{
		ClusterUUID: "e1a379e4-9ac5-4353-8f82-ad066a734f18",
		Description: "test",
		ServiceName: "test",
		CreatedBy:   "test-service",
		Summary:     "test",
	}
	msgBytes, err := json.Marshal(entry)
	helpers.FailOnError(t, err)

	_, _, err = producer.ProduceMessage(msgBytes)
	assert.NoError(t, err)
}
