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

package differ_test

// Documentation in literate-programming-style is available at:
// https://redhatinsights.github.io/ccx-notification-writer/packages/differ/differ_test.html

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RedHatInsights/ccx-notification-service/tests/mocks"
	"github.com/RedHatInsights/insights-results-aggregator-data/testdata"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/mock"

	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/differ"
	"github.com/RedHatInsights/ccx-notification-service/types"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

const (
	reason  = "This reason is more than 255 characters long. This reason is more than 255 characters long. This reason is more than 255 characters long. This reason is more than 255 characters long. This reason is more than 255 characters long. This reason is more than 255 characters long."
	summary = "This summary is more than 255 characters long. This summary is more than 255 characters long. This summary is more than 255 characters long. This summary is more than 255 characters long. This summary is more than 255 characters long. This summary is more than 255 characters long."
)

var reasonTooLong = strings.Repeat(reason, (differ.ServiceLogDescriptionMaxLength+len(reason)+1)/len(reason))

func init() {
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	differ.SetServiceLogSeverityMap()
}

// ---------------------------------------------------------------------------------------
func TestSetServiceLogSeverityMap(t *testing.T) {
	// set by init
	assert.NotEmpty(t, differ.ServiceLogSeverityMap)
	expected := map[int]string{
		differ.TotalRiskLow:       differ.ServiceLogSeverityInfo,
		differ.TotalRiskModerate:  differ.ServiceLogSeverityWarning,
		differ.TotalRiskImportant: differ.ServiceLogSeverityMajor,
		differ.TotalRiskCritical:  differ.ServiceLogSeverityCritical,
	}
	assert.Equal(t, differ.ServiceLogSeverityMap, &expected)
}

// ---------------------------------------------------------------------------------------
func TestCreateServiceLogEntrySummaryAndDescriptionLengthOverMax(t *testing.T) {
	report := types.RenderedReport{
		RuleID:      "a_rule_id",
		ErrorKey:    "an_error_key",
		Resolution:  "a_resolution",
		Reason:      reasonTooLong,
		Description: summary,
	}
	cluster := types.ClusterEntry{
		OrgID:         1,
		AccountNumber: 1,
		ClusterName:   "first_cluster",
		KafkaOffset:   0,
		UpdatedAt:     types.Timestamp(testTimestamp),
	}

	res := differ.CreateServiceLogEntry(&report, cluster, "test_created_by", "test_username", "Info")
	assert.Equal(t, res.CreatedBy, "test_created_by")
	assert.Equal(t, res.Username, "test_username")
	assert.Equal(t, res.ClusterUUID, types.ClusterName("first_cluster"))
	fmt.Println("stoff", len(reasonTooLong))
	assert.Equal(t, res.Summary, summary[0:differ.ServiceLogSummaryMaxLength])
	assert.Equal(t, res.Description, reasonTooLong[0:differ.ServiceLogDescriptionMaxLength])
}

func TestCreateServiceLogEntrySummaryLengthOverMax(t *testing.T) {
	report := types.RenderedReport{
		RuleID:      "a_rule_id",
		ErrorKey:    "an_error_key",
		Resolution:  "a_resolution",
		Reason:      "a short reason",
		Description: summary,
	}
	cluster := types.ClusterEntry{
		OrgID:         1,
		AccountNumber: 1,
		ClusterName:   "first_cluster",
		KafkaOffset:   0,
		UpdatedAt:     types.Timestamp(testTimestamp),
	}

	res := differ.CreateServiceLogEntry(&report, cluster, "test_created_by", "test_username", "Info")
	assert.Equal(t, res.CreatedBy, "test_created_by")
	assert.Equal(t, res.Username, "test_username")
	assert.Equal(t, res.ClusterUUID, types.ClusterName("first_cluster"))
	assert.Equal(t, res.Summary, summary[0:differ.ServiceLogSummaryMaxLength])
	assert.Equal(t, res.Description, "a short reason")
}

func TestCreateServiceLogEntryDescriptionLengthOverMax(t *testing.T) {
	report := types.RenderedReport{
		RuleID:      "a_rule_id",
		ErrorKey:    "an_error_key",
		Resolution:  "a_resolution",
		Reason:      reasonTooLong,
		Description: summary[0:100],
	}
	cluster := types.ClusterEntry{
		OrgID:         1,
		AccountNumber: 1,
		ClusterName:   "first_cluster",
		KafkaOffset:   0,
		UpdatedAt:     types.Timestamp(testTimestamp),
	}

	res := differ.CreateServiceLogEntry(&report, cluster, "test_created_by", "test_username", "Info")
	assert.Equal(t, res.CreatedBy, "test_created_by")
	assert.Equal(t, res.Username, "test_username")
	assert.Equal(t, res.ClusterUUID, types.ClusterName("first_cluster"))
	assert.Equal(t, res.Summary, summary[0:100])
	assert.Equal(t, res.Description, reasonTooLong[0:differ.ServiceLogDescriptionMaxLength])
}

func TestGetServiceLogSeverityValidTotalRisk(t *testing.T) {
	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.WarnLevel)
	assert.Equal(t, differ.GetServiceLogSeverity(differ.TotalRiskLow), differ.ServiceLogSeverityInfo)
	assert.Equal(t, differ.GetServiceLogSeverity(differ.TotalRiskModerate), differ.ServiceLogSeverityWarning)
	assert.Equal(t, differ.GetServiceLogSeverity(differ.TotalRiskImportant), differ.ServiceLogSeverityMajor)
	assert.Equal(t, differ.GetServiceLogSeverity(differ.TotalRiskCritical), differ.ServiceLogSeverityCritical)
	assert.NotContains(t, buf.String(), differ.NoEquivalentSeverityMessage)
}

func TestGetServiceLogSeverityInvalidTotalRiskIsSetToInfo(t *testing.T) {
	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.WarnLevel)

	assert.Equal(t, differ.GetServiceLogSeverity(0), differ.ServiceLogSeverityInfo)
	assert.Contains(t, buf.String(), differ.NoEquivalentSeverityMessage)
	buf.Reset()
	assert.Equal(t, differ.GetServiceLogSeverity(rand.Intn(math.MaxInt64)+differ.TotalRiskMax), differ.ServiceLogSeverityInfo)
	assert.Contains(t, buf.String(), differ.NoEquivalentSeverityMessage)
}

// ---------------------------------------------------------------------------------------
func TestProduceEntriesToServiceLog(t *testing.T) {
	testCases := []struct {
		name                                 string
		produceMessageError                  error
		expectedMessages                     int
		expectedProduceMessageCalls          int
		expectedNotificationSentDiff         int
		expectedNotificationNotSentErrorDiff int
	}{
		{
			name:                                 "Success - all messages sent",
			produceMessageError:                  nil,
			expectedMessages:                     3,
			expectedProduceMessageCalls:          3,
			expectedNotificationSentDiff:         3,
			expectedNotificationNotSentErrorDiff: 0,
		},
		{
			name:                                 "Failure - ProduceMessage returns error",
			produceMessageError:                  fmt.Errorf("failed to produce message"),
			expectedMessages:                     0,
			expectedProduceMessageCalls:          3,
			expectedNotificationSentDiff:         0,
			expectedNotificationNotSentErrorDiff: 3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/rendered_reports" {
					t.Errorf("Expected to request '/render_reports', got: %s", r.URL.Path)
				}
				if r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("Expected Content-Type: application/json header, got: %s", r.Header.Get("Accept"))
				}
				w.WriteHeader(http.StatusOK)
				_, err := fmt.Fprintf(w, `{"clusters":["84f7eedc-0dd8-49cd-9d4d-f6646df3a5bc"],"reports":{"84f7eedc-0dd8-49cd-9d4d-f6646df3a5bc":[{"rule_id":"node_installer_degraded","error_key":"ek1","resolution":"rule 1 resolution","reason":"%s","description":"%s"},{"rule_id":"rule2","error_key":"ek2","resolution":"rule 2 resolution","reason":"rule 2 reason","description":"rule 2 error key description"}, {"rule_id":"rule3","error_key":"ek3","resolution":"rule 3 resolution","reason":"rule 3 reason","description":"rule 3 error key description"}]}}`, reasonTooLong, summary)
				if err != nil {
					log.Fatal().Msg(err.Error())
				}
			}))
			defer server.Close()

			config := conf.ConfigStruct{
				ServiceLog: conf.ServiceLogConfiguration{
					Enabled:            true,
					TotalRiskThreshold: 1,
					EventFilter:        "totalRisk > totalRiskThreshold",
				},
				Dependencies: conf.DependenciesConfiguration{
					TemplateRendererServer:   server.URL,
					TemplateRendererEndpoint: "/rendered_reports",
					TemplateRendererURL:      server.URL + "/rendered_reports",
				},
			}

			producerMock := mocks.Producer{}
			producerMock.On("ProduceMessage", mock.AnythingOfType("types.ProducerMessage")).Return(
				func(msg types.ProducerMessage) int32 {
					return 0
				},
				func(msg types.ProducerMessage) int64 {
					return 0
				},
				func(msg types.ProducerMessage) error {
					return tc.produceMessageError
				},
			)

			d := differ.Differ{
				NotificationType: types.InstantNotif,
				Target:           types.NotificationBackendTarget,
				Filter:           differ.DefaultEventFilter,
				Notifier:         &producerMock,
			}

			ruleContent := types.RulesMap{
				"node_installer_degraded": testdata.RuleContent1,
				"rule2":                   testdata.RuleContent2,
				"rule3":                   testdata.RuleContent3,
			}

			rules := differ.GetAllContentFromMap(ruleContent)

			cluster := types.ClusterEntry{
				OrgID:         1,
				AccountNumber: 1,
				ClusterName:   types.ClusterName(testdata.ClusterName),
				KafkaOffset:   0,
				UpdatedAt:     types.Timestamp(testTimestamp),
			}

			var deserialized types.Report
			err := json.Unmarshal([]byte(testdata.ClusterReport3Rules), &deserialized)
			if err != nil {
				log.Error().Msg(err.Error())
				return
			}
			reports := deserialized.Reports

			var notificationSentBefore = int(testutil.ToFloat64(differ.NotificationSent))
			var notificationNotSentErrorStateBefore = int(testutil.ToFloat64(differ.NotificationNotSentErrorState))
			var notificationNotSentSameStateBefore = int(testutil.ToFloat64(differ.NotificationNotSentSameState))

			messages, err := d.ProduceEntriesToServiceLog(&config, cluster, rules, ruleContent, reports)
			producerMock.AssertNumberOfCalls(t, "ProduceMessage", tc.expectedProduceMessageCalls)
			assert.Nil(t, err)
			assert.Equal(t, tc.expectedMessages, messages)

			notificationSentDiff := int(testutil.ToFloat64(differ.NotificationSent)) - notificationSentBefore
			notificationNotSentErrorStateDiff := int(testutil.ToFloat64(differ.NotificationNotSentErrorState)) - notificationNotSentErrorStateBefore
			notificationNotSentSameStateDiff := int(testutil.ToFloat64(differ.NotificationNotSentSameState)) - notificationNotSentSameStateBefore

			assert.Equal(t, tc.expectedNotificationSentDiff, notificationSentDiff,
				"Unexpected NotificationSent metric value")
			assert.Equal(t, tc.expectedNotificationNotSentErrorDiff, notificationNotSentErrorStateDiff,
				"Unexpected NotificationNotSentErrorState metric value")
			assert.Equal(t, 0, notificationNotSentSameStateDiff,
				"Expected metric value to be 0 as the reports weren't already notified")
		})
	}
}
