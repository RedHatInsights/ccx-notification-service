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
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/types"
	"github.com/go-yaml/yaml"
	"github.com/rs/zerolog/log"
)

// readFilesIntoByteArrayPointers reads the contents of the specified files
// in the base directory and saves them via the specified byte slice pointers.
func readFilesIntoFileContent(baseDir string, filelist []string) (map[string][]byte, error) {
	var filesContent = map[string][]byte{}
	for _, name := range filelist {
		log.Info().Msgf("Parsing %s/%s", baseDir, name)
		var err error
		rawBytes, err := ioutil.ReadFile(filepath.Clean(path.Join(baseDir, name)))
		if err != nil {
			filesContent[name] = nil
			log.Error().Err(err)
		} else {
			filesContent[name] = rawBytes
		}
	}

	return filesContent, nil
}

// createErrorContents takes a mapping of files into contents and perform
// some checks about it
func createErrorContents(contentRead map[string][]byte) (*types.RuleErrorKeyContent, error) {
	errorContent := types.RuleErrorKeyContent{}

	if contentRead["generic.md"] == nil {
		return nil, &types.MissingMandatoryFile{FileName: "generic.md"}
	}

	errorContent.Generic = string(contentRead["generic.md"])

	if contentRead["reason.md"] == nil {
		errorContent.Reason = ""
		errorContent.HasReason = false
	} else {
		errorContent.Reason = string(contentRead["reason.md"])
		errorContent.HasReason = true
	}

	if contentRead["metadata.yaml"] == nil {
		return nil, &types.MissingMandatoryFile{FileName: "metadata.yaml"}
	}

	if err := yaml.Unmarshal(contentRead["metadata.yaml"], &errorContent.Metadata); err != nil {
		return nil, err
	}

	return &errorContent, nil
}

// parseErrorContents function reads the contents of the specified directory
// and parses all subdirectories as error key contents.  This implicitly checks
// that the directory exists, so it is not necessary to ever check that
// elsewhere.
func parseErrorContents(ruleDirPath string) (map[string]types.RuleErrorKeyContent, error) {
	entries, err := ioutil.ReadDir(ruleDirPath)
	if err != nil {
		return nil, err
	}

	errorContents := map[string]types.RuleErrorKeyContent{}

	for _, e := range entries {
		if e.IsDir() {
			name := e.Name()
			contentFiles := []string{
				"generic.md",
				"reason.md",
				"metadata.yaml",
			}

			readContents, err := readFilesIntoFileContent(path.Join(ruleDirPath, name), contentFiles)
			if err != nil {
				return errorContents, err
			}

			errContents, err := createErrorContents(readContents)
			if err != nil {
				return errorContents, err
			}
			errorContents[name] = *errContents
		}
	}

	return errorContents, nil
}

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

// checkRequiredFields function checks if all the required fields in the
// RuleContent are ok and valid. At the moment only checks for Reason field is
// being performed.
func checkRequiredFields(rule types.RuleContent) error {
	if rule.HasReason {
		return nil
	}

	for _, errorKeyContent := range rule.ErrorKeys {
		if !errorKeyContent.HasReason {
			return &types.MissingMandatoryFile{FileName: "reason.md"}
		}
	}

	return nil
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
