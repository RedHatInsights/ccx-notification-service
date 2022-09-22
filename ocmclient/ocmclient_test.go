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

package ocmclient_test

import (
	"testing"

	"github.com/RedHatInsights/ccx-notification-service/ocmclient"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
}

// TestClientCreation tests if creating a client works as expected
func TestClientCreationError(t *testing.T) {
	clientID := ""
	clientSecret := ""
	url := ""
	_, err := ocmclient.NewOCMClient(clientID, clientSecret, url)
	assert.NotEqual(t, nil, err)
}

// TestClientCreationWithTransportError tests if creating a client works as expected
func TestClientCreationWithTransportError(t *testing.T) {
	clientID := ""
	clientSecret := ""
	url := ""
	_, err := ocmclient.NewOCMClientWithTransport(clientID, clientSecret, url, nil)
	assert.NotEqual(t, nil, err)
}
