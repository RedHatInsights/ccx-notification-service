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

// Package servicelog contains an implementation of Producer interface that can
// be used to produce (that is send) messages to Service Log.
package servicelog

// Generated documentation is available at:
// https://pkg.go.dev/github.com/RedHatInsights/ccx-notification-service/producer
//
// Documentation in literate-programming-style is available at:
// https://redhatinsights.github.io/ccx-notification-service/packages/producer/servicelog/service_log_producer.html

import (
	"bytes"
	"fmt"
	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/ocmclient"
	"github.com/RedHatInsights/ccx-notification-service/types"
	httputils "github.com/RedHatInsights/insights-operator-utils/http"
	"github.com/rs/zerolog/log"
	"net/http"
	"time"
)

// Producer is an implementation of Producer interface for Service Log
type Producer struct {
	Configuration              conf.ServiceLogConfiguration
	OCMClient                  ocmclient.OCMClient
	AccessToken                string
	TokenRefreshmentCounter    int
	TokenRefreshmentStartDelay time.Duration
	TokenRefreshmentDelay      time.Duration
	TokenRefreshmentThreshold  time.Duration
}

// New constructs a new instance of Producer implementation
func New(config *conf.ServiceLogConfiguration, ocmClient ocmclient.OCMClient) (*Producer, error) {
	prod := &Producer{
		Configuration:              *config,
		TokenRefreshmentStartDelay: time.Second,
		TokenRefreshmentDelay:      time.Second,
		TokenRefreshmentThreshold:  30 * time.Second,
	}
	prod.OCMClient = ocmClient
	err := prod.refreshToken()
	if err != nil {
		return prod, err
	}
	return prod, nil
}

// ProduceMessage sends the given message to Service Log
func (producer *Producer) ProduceMessage(msg types.ProducerMessage) (partitionID int32, offset int64, err error) {
	serviceLogURL := httputils.SetHTTPPrefix(producer.Configuration.URL)

	client := &http.Client{
		Timeout: time.Second * producer.Configuration.Timeout,
	}

	req, err := http.NewRequest(http.MethodPost, serviceLogURL, bytes.NewBuffer(msg))
	req.Header.Add("Authorization", "Bearer "+producer.AccessToken)
	if err != nil {
		log.Error().Err(err).Str("url", serviceLogURL).Msg("Error setting up HTTP POST request")
		return -1, -1, err
	}

	response, err := client.Do(req)
	if err != nil {
		log.Error().Err(err).Msgf("Error making the HTTP request")
		return -1, -1, err
	}

	switch response.StatusCode {
	case http.StatusUnauthorized:
		err = producer.refreshToken()
		for {
			if err == nil {
				break
			}
			if producer.TokenRefreshmentDelay >= producer.TokenRefreshmentThreshold {
				log.Error().Err(err).Msg("Access token could not be refreshed")
				return -1, -1, err
			}
			log.Error().
				Err(err).
				Dur("delay", producer.TokenRefreshmentDelay).
				Msgf("Could not receive a new access token. Retrying...")
			time.Sleep(producer.TokenRefreshmentDelay)
			err = producer.refreshToken()
			producer.TokenRefreshmentDelay = 2 * producer.TokenRefreshmentDelay
		}
		producer.TokenRefreshmentDelay = producer.TokenRefreshmentStartDelay
		return producer.ProduceMessage(msg)
	case http.StatusCreated:
		return 0, 0, nil
	default:
		err = fmt.Errorf("received unexpected response status code - %s", response.Status)
		log.Error().Err(err).Msgf("Got unexpected response status code")
		return -1, -1, err
	}
}

// Close closes Producer (in case of Service Log implementation, it does not do anything)
func (producer *Producer) Close() error {
	return nil
}

func (producer *Producer) refreshToken() error {
	log.Info().Msg("refreshing access token...")
	accessToken, _, err := producer.OCMClient.GetTokens(producer.TokenRefreshmentDelay)
	if err != nil {
		return err
	}
	producer.AccessToken = accessToken
	log.Info().Msg("access token refreshed successfully")
	return nil
}
