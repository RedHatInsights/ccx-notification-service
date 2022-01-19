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

package differ

// Generated documentation is available at:
// https://pkg.go.dev/github.com/RedHatInsights/ccx-notification-service/differ
//
// Documentation in literate-programming-style is available at:
// https://redhatinsights.github.io/ccx-notification-service/packages/differ/content.html

import (
	"bytes"
	"encoding/gob"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/types"
	utypes "github.com/RedHatInsights/insights-operator-utils/types"
	"github.com/rs/zerolog/log"
)

// fetchAllRulesContent fetches the parsed rules provided by the content-service
func fetchAllRulesContent(config conf.DependenciesConfiguration) (rules types.RulesMap, err error) {
	contentURL := config.ContentServiceServer + config.ContentServiceEndpoint
	if !strings.HasPrefix(config.ContentServiceServer, "http") {
		// if no protocol is specified in given URL, assume it is not
		// needed to use https
		contentURL = "http://" + contentURL
	}
	log.Info().Msgf("Fetching rules content from %s", contentURL)

	client := &http.Client{
		Timeout: time.Second * 10,
	}
	req, err := http.NewRequest("GET", contentURL, http.NoBody)
	if err != nil {
		log.Error().Msgf("Got error while setting up HTTP request -  %s", err.Error())
		return nil, err
	}

	response, err := client.Do(req)
	if err != nil {
		log.Error().Msgf("Got error while making the HTTP request - %s", err.Error())
		return nil, err
	}

	// Read body from response
	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Error().Msgf("Got error while reading the response's body - %s", err.Error())
		return nil, err
	}

	err = response.Body.Close()
	if err != nil {
		log.Error().Msgf("Got error while closing the response body - %s", err.Error())
	}

	var receivedContent utypes.RuleContentDirectory

	err = gob.NewDecoder(bytes.NewReader(body)).Decode(&receivedContent)
	if err != nil {
		log.Error().Err(err).Msg("Error trying to decode rules content from received answer")
		os.Exit(ExitStatusFetchContentError)
	}

	rules = receivedContent.Rules

	log.Info().Msgf("Retrieved %d rules from content service", len(rules))

	return
}
