// Copyright 2022 Red Hat, Inc
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

package differ

import (
	"encoding/json"
	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/types"
	utypes "github.com/RedHatInsights/insights-results-types"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
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

	reports := []types.ReportItem{
		types.ReportItem{
			Type:     "rule",
			Module:   "rule_1.report",
			ErrorKey: "RULE_1",
			Details:  json.RawMessage("{}"),
		},
		types.ReportItem{
			Type:     "rule",
			Module:   "rule_2.report",
			ErrorKey: "RULE_2",
			Details:  json.RawMessage("{}"),
		},
	}

	rendereredReports, err := renderReportsForCluster(config, "e1a379e4-9ac5-4353-8f82-ad066a734f18", reports, ruleContent)
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