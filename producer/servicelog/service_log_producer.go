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

// Package servicelog contains functions that can be used to produce (that is
// send) messages to Service Log.
package servicelog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/types"
	httputils "github.com/RedHatInsights/insights-operator-utils/http"
	"github.com/rs/zerolog/log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Producer is an implementation of Producer interface for Service Log
type Producer struct {
	Configuration              conf.ServiceLogConfiguration
	AccessToken                string
	TokenRefreshmentCounter    int
	TokenRefreshmentStartDelay time.Duration
	TokenRefreshmentDelay      time.Duration
	TokenRefreshmentThreshold  time.Duration
}

// New constructs a new instance of Producer implementation
func New(config conf.ConfigStruct) (*Producer, error) {
	prod := &Producer{
		Configuration:              conf.GetServiceLogConfiguration(config),
		TokenRefreshmentStartDelay: time.Second,
		TokenRefreshmentDelay:      time.Second,
		TokenRefreshmentThreshold:  30 * time.Second,
	}
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
		log.Error().Err(err).Msg("Got error while setting up HTTP request")
		return -1, -1, err
	}

	response, err := client.Do(req)
	if err != nil {
		log.Error().Err(err).Msgf("Got error while making the HTTP request")
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
		log.Error().Err(err).Msgf("Got unexpected response status code from token refreshment API")
		return -1, -1, err
	}
}

// Close closes Producer (in case of Service Log implementation, it does not do anything)
func (producer *Producer) Close() error {
	return nil
}

func (producer *Producer) refreshToken() error {
	params := url.Values{}
	params.Add("grant_type", `refresh_token`)
	params.Add("client_id", `rhsm-api`)
	params.Add("refresh_token", producer.Configuration.OfflineToken)
	body := strings.NewReader(params.Encode())

	tokenRefreshmentURL := httputils.SetHTTPPrefix(producer.Configuration.TokenRefreshmentURL)
	req, err := http.NewRequest(http.MethodPost, tokenRefreshmentURL, body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err != nil {
		log.Error().Err(err).Msg("Got error while setting up HTTP request for refreshing access token")
		return err
	}

	response, err := httputils.SendRequest(req, 15*time.Second)
	if err != nil {
		return err
	}

	var receivedToken *types.AccessTokenOutput
	err = json.Unmarshal(response, &receivedToken)
	if err != nil {
		log.Error().Err(err).Msg("Error trying to decode template renderer output from received answer")
		return err
	}
	producer.AccessToken = receivedToken.AccessToken
	return nil
}
