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

package differ

// Documentation in literate-programming-style is available at:
// https://redhatinsights.github.io/ccx-notification-writer/packages/differ/renderer_test.html

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"

	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/types"
	utypes "github.com/RedHatInsights/insights-results-types"
)

func TestRenderReportsForCluster(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{"clusters":["e1a379e4-9ac5-4353-8f82-ad066a734f18"],"reports":{"e1a379e4-9ac5-4353-8f82-ad066a734f18":[{"rule_id":"rule_1","error_key":"RULE_1","resolution":"rule 1 resolution","reason":"rule 1 reason","description":"rule 1 error key description"},{"rule_id":"rule_2","error_key":"RULE_2","resolution":"rule 2 resolution","reason":"","description":"rule 2 error key description"}]}}`))
		if err != nil {
			log.Fatal().Msg(err.Error())
		}
	}))
	defer server.Close()

	config := conf.DependenciesConfiguration{
		TemplateRendererServer:   server.URL,
		TemplateRendererEndpoint: "",
		TemplateRendererURL:      server.URL,
	}

	errorKeys := map[string]utypes.RuleErrorKeyContent{
		"RULE_1": {
			Metadata: utypes.ErrorKeyMetadata{
				Description: "rule 1 error key description",
				Impact: utypes.Impact{
					Name:   "impact_1",
					Impact: 3,
				},
				Likelihood: 2,
			},
			Reason:    "rule 1 reason",
			HasReason: true,
		},
		"RULE_2": {
			Metadata: utypes.ErrorKeyMetadata{
				Description: "rule 2 error key description",
				Impact: utypes.Impact{
					Name:   "impact_2",
					Impact: 2,
				},
				Likelihood: 3,
			},
			HasReason: false,
		},
	}
	ruleContent := types.RulesMap{
		"rule_1": {
			Plugin: utypes.RulePluginInfo{
				PythonModule: "rule_1",
			},
			Summary:    "rule 1 summary",
			Reason:     "rule 1 reason",
			Resolution: "rule 1 resolution",
			MoreInfo:   "rule 1 more info",
			ErrorKeys:  errorKeys,
			HasReason:  true,
		},
		"rule_2": {
			Plugin: utypes.RulePluginInfo{
				PythonModule: "rule_2",
			},
			Summary:    "rule 2 summary",
			Reason:     "",
			Resolution: "rule 2 resolution",
			MoreInfo:   "rule 2 more info",
			ErrorKeys:  errorKeys,
			HasReason:  false,
		},
	}

	rules := getAllContentFromMap(ruleContent)

	reports := types.ReportContent{
		{
			ReportItem: types.ReportItem{
				Type:     "rule",
				Module:   "rule_1.report",
				ErrorKey: "RULE_1",
				Details:  json.RawMessage("{}"),
			},
		},
		{
			ReportItem: types.ReportItem{
				Type:     "rule",
				Module:   "rule_2.report",
				ErrorKey: "RULE_2",
				Details:  json.RawMessage("{}"),
			},
		},
	}
	rendereredReports, err := renderReportsForCluster(&config, "e1a379e4-9ac5-4353-8f82-ad066a734f18", reports, rules)
	v, _ := json.Marshal(rendereredReports)
	log.Info().Msg(string(v))
	assert.NoError(t, err)
	assert.Equal(t, 2, len(rendereredReports))
	assert.Contains(t, rendereredReports, types.RenderedReport{
		RuleID:      "rule_1",
		ErrorKey:    "RULE_1",
		Resolution:  "rule 1 resolution",
		Reason:      "rule 1 reason",
		Description: "rule 1 error key description",
	})
}

func TestRenderReportsForClusterInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`not a JSON`))
		if err != nil {
			log.Fatal().Msg(err.Error())
		}
	}))
	defer server.Close()

	config := conf.DependenciesConfiguration{
		TemplateRendererServer:   server.URL,
		TemplateRendererEndpoint: "",
		TemplateRendererURL:      server.URL,
	}

	rendereredReports, err := renderReportsForCluster(&config, "e1a379e4-9ac5-4353-8f82-ad066a734f18", types.ReportContent{}, []utypes.RuleContent{})
	v, _ := json.Marshal(rendereredReports)
	log.Info().Msg(string(v))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid character")
}

func TestRenderReportsForClusterInvalidURL(t *testing.T) {
	config := conf.DependenciesConfiguration{
		TemplateRendererServer:   "not an url",
		TemplateRendererEndpoint: "",
		TemplateRendererURL:      "not an url",
	}

	rendereredReports, err := renderReportsForCluster(&config, "e1a379e4-9ac5-4353-8f82-ad066a734f18", types.ReportContent{}, []utypes.RuleContent{})
	v, _ := json.Marshal(rendereredReports)
	log.Info().Msg(string(v))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported protocol")
}

// TestGetAllContentFromMapEmptyCase tests the function getAllContentFromMap
// for empty input
func TestGetAllContentFromMapEmptyCase(t *testing.T) {
	// empty input
	var ruleContent types.RulesMap

	allContent := getAllContentFromMap(ruleContent)

	// result should be empty as well
	assert.Empty(t, allContent)
}

// TestGetAllContentFromMapOneItem tests the function getAllContentFromMap
// for input with one item
func TestGetAllContentFromMapOneItem(t *testing.T) {
	var errorKeys map[string]utypes.RuleErrorKeyContent

	// input with one item
	ruleContent := types.RulesMap{
		"rule_1": {
			Plugin: utypes.RulePluginInfo{
				PythonModule: "rule_1",
			},
			Summary:    "rule 1 summary",
			Reason:     "rule 1 reason",
			Resolution: "rule 1 resolution",
			MoreInfo:   "rule 1 more info",
			ErrorKeys:  errorKeys,
			HasReason:  true,
		},
	}

	allContent := getAllContentFromMap(ruleContent)

	// result should contain exactly one item
	assert.NotEmpty(t, allContent)
	assert.Len(t, allContent, 1)

	// check the content
	rule1 := allContent[0]
	assert.Equal(t, rule1, ruleContent["rule_1"])
}

// TestGetAllContentFromMapTwoItems tests the function getAllContentFromMap
// for input with two items
func TestGetAllContentFromMapTwoItems(t *testing.T) {
	var errorKeys map[string]utypes.RuleErrorKeyContent

	// input with one item
	ruleContent := types.RulesMap{
		"rule_1": {
			Plugin: utypes.RulePluginInfo{
				PythonModule: "rule_1",
			},
			Summary:    "rule 1 summary",
			Reason:     "rule 1 reason",
			Resolution: "rule 1 resolution",
			MoreInfo:   "rule 1 more info",
			ErrorKeys:  errorKeys,
			HasReason:  true,
		},
		"rule_2": {
			Plugin: utypes.RulePluginInfo{
				PythonModule: "rule_2",
			},
			Summary:    "rule 2 summary",
			Reason:     "",
			Resolution: "rule 2 resolution",
			MoreInfo:   "rule 2 more info",
			ErrorKeys:  errorKeys,
			HasReason:  false,
		},
	}

	allContent := getAllContentFromMap(ruleContent)

	// result should contain exactly two items
	assert.NotEmpty(t, allContent)

	// we are not sure about their order
	assert.Len(t, allContent, 2)

	// check the content
	rule1 := allContent[0]
	rule2 := allContent[1]

	// we are not sure about order
	if rule1.Summary == "rule 1 summary" {
		// order seems to be rule1->rule2
		assert.Equal(t, rule1, ruleContent["rule_1"])
		assert.Equal(t, rule2, ruleContent["rule_2"])
	} else {
		// order seems to be rule2->rule1
		assert.Equal(t, rule1, ruleContent["rule_2"])
		assert.Equal(t, rule2, ruleContent["rule_1"])
	}
}

func TestAddDetailedInfoURLToRenderedReport(t *testing.T) {
	detailsURI := "theUri/{module}|{error_key}"
	renderedReport := types.RenderedReport{
		RuleID:      "a.rule.id",
		ErrorKey:    "THE_ERROR_KEY",
		Resolution:  "this issue cannot be resolved",
		Reason:      "there is no real reason",
		Description: "this is just a test issue with a bunch of words in it",
	}

	addDetailedInfoURLToRenderedReport(&renderedReport, &detailsURI)
	assert.Equal(t, renderedReport.Reason, "there is no real reason\n\n[More details](theUri/a.rule.id|THE_ERROR_KEY).")
}
