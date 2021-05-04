// Copyright 2021 Red Hat, Inc
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

package main

import (
	"bytes"
	"encoding/gob"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/types"
	"github.com/go-yaml/yaml"
	"github.com/rs/zerolog/log"
)

// parseGlobalContentConfig reads the configuration file used to store
// metadata used by all rule content, such as impact dictionary.
func parseGlobalContentConfig(configPath string) (types.GlobalRuleConfig, error) {
	configBytes, err := ioutil.ReadFile(filepath.Clean(configPath))
	if err != nil {
		return types.GlobalRuleConfig{}, err
	}

	conf := types.GlobalRuleConfig{}
	err = yaml.Unmarshal(configBytes, &conf)
	return conf, err
}

// fetchAllRulesContent fetches the parsed rules provided by the content-service
func fetchAllRulesContent(config conf.DependenciesConfiguration) (rules types.RulesMap, err error) {
	contentUrl := config.ContentServiceServer + config.ContentServiceEndpoint
	if !strings.HasPrefix(config.ContentServiceServer, "http") {
		//if no protocol is specified in given URL, assume it is not needed to use https
		contentUrl = "http://" + contentUrl
	}
	log.Info().Msgf("Fetching rules content from ", contentUrl)

	client := &http.Client{
		Timeout: time.Second * 10,
	}
	req, err := http.NewRequest("GET", contentUrl, nil)
	if err != nil {
		log.Error().Msgf("Got error while setting up HTTP request -  %s", err.Error())
		return nil, err
	}

	response, err := client.Do(req)
	if err != nil {
		log.Error().Msgf("Got error while making the HTTP request - %s", err.Error())
		return nil, err
	}

	defer response.Body.Close()

	// Read body from response
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Error().Msgf("Got error while reading the response's body - %s", err.Error())
		return nil, err
	}

	var receivedContent types.RuleContentDirectory

	err = gob.NewDecoder(bytes.NewReader(body)).Decode(&receivedContent)
	if err != nil {
		log.Error().Err(err).Msg("Error trying to decode rules content from received answer")
		os.Exit(ExitStatusFetchContentError)
	}

	rules = receivedContent.Rules

	log.Info().Msgf("Got %d rules from content service", len(rules))

	return
}
