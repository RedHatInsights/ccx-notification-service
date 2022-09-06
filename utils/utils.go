/*
Copyright © 2022 Red Hat, Inc.

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

package utils

import (
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"strings"
	"time"
)

// SetHTTPPrefix adds HTTP prefix if it is not already present in the given string
func SetHTTPPrefix(url string) string {
	if !strings.HasPrefix(url, "http") {
		// if no protocol is specified in given URL, assume it is not
		// needed to use https
		url = "http://" + url
	}
	return url
}

// SendRequest sends the given request, reads the body and handles related errors
func SendRequest(req *http.Request, timeout time.Duration) ([]byte, error) {
	client := &http.Client{
		Timeout: timeout,
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
		return nil, err
	}
	return body, nil
}
