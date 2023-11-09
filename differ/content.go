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

package differ

// Generated documentation is available at:
// https://pkg.go.dev/github.com/RedHatInsights/ccx-notification-service/differ
//
// Documentation in literate-programming-style is available at:
// https://redhatinsights.github.io/ccx-notification-service/packages/differ/content.html

import (
	"bytes"
	"encoding/gob"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/types"
	httputils "github.com/RedHatInsights/insights-operator-utils/http"
	utypes "github.com/RedHatInsights/insights-results-types"
)

// fetchAllRulesContent fetches the parsed rules provided by the content-service
func fetchAllRulesContent(config *conf.DependenciesConfiguration) (rules types.RulesMap, err error) {
	contentURL := httputils.SetHTTPPrefix(config.ContentServiceServer + config.ContentServiceEndpoint)

	log.Info().Msgf("Fetching rules content from %s", contentURL)

	req, err := http.NewRequest("GET", contentURL, http.NoBody)
	if err != nil {
		log.Error().Msgf("Got error while setting up HTTP request for content service -  %s", err.Error())
		return nil, err
	}

	body, err := httputils.SendRequest(req, 10*time.Second)
	if err != nil {
		log.Error().Msgf("Got error while processing HTTP request - %s", err.Error())
		return nil, err
	}

	var receivedContent utypes.RuleContentDirectory

	err = gob.NewDecoder(bytes.NewReader(body)).Decode(&receivedContent)
	if err != nil {
		log.Error().Err(err).Msg("Error trying to decode rules content from received answer")
		return nil, err
	}

	rules = receivedContent.Rules

	log.Info().Msgf("Retrieved %d rules from content service", len(rules))

	return
}
