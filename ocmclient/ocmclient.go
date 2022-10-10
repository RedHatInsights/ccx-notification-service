// Copyright 2021, 2022 Red Hat, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ocmclient

import (
	"fmt"
	gateway "github.com/openshift-online/ocm-sdk-go"
	"github.com/rs/zerolog/log"
	"net/http"
	"time"
)

// OCMGateway is an implementation of the OCMClient interface
type OCMGateway struct {
	connection *gateway.Connection
}

// OCMClient interface defines the methods that interact with the OCM Gateway
type OCMClient interface {
	GetTokens(delay time.Duration) (string, string, error)
}

// NewOCMClient creates a client that can comunicate with the OCM API
func NewOCMClient(clientID, clientSecret, url string, tokenURL string) (OCMClient, error) {
	return NewOCMClientWithTransport(clientID, clientSecret, url, tokenURL, nil)
}

// NewOCMClientWithTransport creates a client that can comunicate with the OCM API, enabling to use a transport wrapper
func NewOCMClientWithTransport(clientID, clientSecret, url string, tokenURL string, transport http.RoundTripper) (OCMClient, error) {
	log.Info().Msg("creating client for the Openshift Cluster Manager (OCM) API...")
	builder := gateway.NewConnectionBuilder().URL(url)
	if tokenURL != "" {
		builder = builder.TokenURL(tokenURL)
	}

	if transport != nil {
		builder.TransportWrapper(func(http.RoundTripper) http.RoundTripper { return transport })
	}

	if clientID == "" || clientSecret == "" {
		err := fmt.Errorf("cannot create the OCM API client with the provided credentials")
		log.Error().Str("client_id", clientID).Str("client_secret", clientSecret).
			Err(err).Msg("")
		return nil, err
	}
	builder = builder.Client(clientID, clientSecret)
	conn, err := builder.Build()

	if err != nil {
		log.Error().Err(err).Msg("unable to build the connection to OCM gateway API")
		return nil, err
	}

	log.Info().Str("url", conn.URL()).Msg("OCM client created successfully")

	return &OCMGateway{
		conn,
	}, nil
}

// GetTokens returns the access and refresh tokens that are currently in use by the connection
func (c *OCMGateway) GetTokens(delay time.Duration) (onlineToken, refreshToken string, err error) {
	return c.connection.Tokens(delay)
}
