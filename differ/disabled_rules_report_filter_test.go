/*
Copyright © 2026 Red Hat, Inc.

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

// Unit tests for the report filtering added in CCXDEV-16569: the report stored
// in the `report` column of the `reported` table must not contain the rule hits
// the customer has disabled. That column is the comparison baseline used by
// ShouldNotify/IssueNotInReport, so omitting a disabled rule from it is what
// makes a later re-enable be recognized as a new issue and prevents a disabled
// rule from getting an artificial cooldown extension when another rule of the
// same cluster triggers a notification (design document section 7, "Cooldown
// extension prevention").

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/differ"
	"github.com/RedHatInsights/ccx-notification-service/tests/mocks"
	"github.com/RedHatInsights/ccx-notification-service/types"
)

const (
	// criticalCompositeRuleID and importantCompositeRuleID are the composite
	// "rule_id|error_key" values carried by the report JSON. The aggregator
	// tables store the rule id and the error key in two separate columns
	// instead, and types.ReportItem has no field mapped to "rule_id", so this
	// value is never read: it only has to survive the filtering untouched.
	criticalCompositeRuleID  = "test_rule|TEST_RULE_CRITICAL_IMPACT"
	importantCompositeRuleID = "test_rule|TEST_RULE_IMPORTANT_IMPACT"

	// otherRuleModule is a fully qualified component of a rule that is not the
	// one registered in the disabled rules maps by these tests.
	otherRuleModule types.ModuleName = "ccx_rules_ocp.external.rules.other_rule.report"

	// reportFilterTestOrgID is the organization of the cluster returned by
	// disabledRulesTestClusterEntry, as the org-wide rule_disable map keys it.
	reportFilterTestOrgID = "1"
)

var (
	// reportFilterCriticalRuleHit and reportFilterImportantRuleHit are the two
	// rule hits of the report used throughout these tests. Both belong to
	// test_rule, which is what moduleToRuleName derives from their component.
	reportFilterCriticalRuleHit  = reportFilterRuleHit(disabledRulesTestModule, criticalCompositeRuleID, criticalImpactErrorKey)
	reportFilterImportantRuleHit = reportFilterRuleHit(disabledRulesTestModule, importantCompositeRuleID, importantImpactErrorKey)
)

// reportFilterRuleHit builds a single rule hit as it is stored in the report
// JSON. Only "type", "component", "key" and "details" are modelled by
// types.ReportItem; the composite "rule_id", "tags" and "links" are not, and
// have to be preserved verbatim by the filtering.
func reportFilterRuleHit(component types.ModuleName, compositeRuleID string, errorKey types.ErrorKey) string {
	return fmt.Sprintf(`{
		"rule_id": %q,
		"component": %q,
		"type": "rule",
		"key": %q,
		"details": {"error_key": %q, "type": "rule"},
		"tags": ["openshift", "service_availability"],
		"links": {"kcs": ["https://access.redhat.com/solutions/1234"]}
	}`, compositeRuleID, component, errorKey, errorKey)
}

// reportFilterDocument assembles a report JSON document out of the given rule
// hits. Next to "reports" the document carries the top level keys a real report
// has, none of which types.Report models: they have to survive the filtering
// untouched as well.
func reportFilterDocument(ruleHits ...string) types.ClusterReport {
	return types.ClusterReport(fmt.Sprintf(`{
		"analysis_metadata": {"start": "2026-01-01T00:00:00Z", "plugin_sets": {"insights-core": "1.0.0"}},
		"fingerprints": [],
		"pass": [{"rule_fqdn": "ccx_rules_ocp.external.rules.passing_rule.report", "key": "PASSING_RULE"}],
		"reports": [%s],
		"skips": [{"rule_fqdn": "ccx_rules_ocp.external.rules.skipped_rule.report", "reason": "MISSING_REQUIREMENTS"}]
	}`, strings.Join(ruleHits, ",")))
}

// clusterDisabledRule returns a cluster_rule_toggle map disabling the given rule
// and error key for the given cluster.
func clusterDisabledRule(clusterName types.ClusterName, ruleID types.RuleID, errorKey types.ErrorKey) types.ClusterDisabledRules {
	return types.ClusterDisabledRules{
		{ClusterID: clusterName, RuleID: ruleID, ErrorKey: errorKey}: {},
	}
}

// orgDisabledRule returns a rule_disable map acking the given rule and error key
// for the given organization.
func orgDisabledRule(orgID string, ruleID types.RuleID, errorKey types.ErrorKey) types.OrgDisabledRules {
	return types.OrgDisabledRules{
		{OrgID: orgID, RuleID: ruleID, ErrorKey: errorKey}: {},
	}
}

// TestFilterDisabledRulesFromReport verifies that the rule hits the customer has
// disabled - through cluster_rule_toggle or through rule_disable - are the only
// ones removed from the stored report, and that everything else in the document
// (the other rule hits with their composite rule_id, tags and links, and the top
// level keys besides "reports") is left as it is. A rule hit is identified the
// same way the Kafka and the Service Log filters identify it: the rule name
// derived from "component" plus the error key taken from "key". Any other
// (cluster, org, rule, error key) combination in the maps is a miss and must
// leave the report unchanged.
func TestFilterDisabledRulesFromReport(t *testing.T) {
	cluster := disabledRulesTestClusterEntry()
	report := reportFilterDocument(reportFilterCriticalRuleHit, reportFilterImportantRuleHit)

	testCases := []struct {
		name           string
		clusterRules   types.ClusterDisabledRules
		orgRules       types.OrgDisabledRules
		expectedReport types.ClusterReport
	}{
		{
			name:           "rule disabled for this cluster is omitted",
			clusterRules:   clusterDisabledRule(cluster.ClusterName, disabledRulesTestRuleID, criticalImpactErrorKey),
			expectedReport: reportFilterDocument(reportFilterImportantRuleHit),
		},
		{
			name:           "rule acked for this organization is omitted",
			orgRules:       orgDisabledRule(reportFilterTestOrgID, disabledRulesTestRuleID, criticalImpactErrorKey),
			expectedReport: reportFilterDocument(reportFilterImportantRuleHit),
		},
		{
			// this is also what --ignore-disabled-rules leads to: both maps are
			// left empty and the original report has to be stored as it is
			name:           "no rule is disabled at all",
			expectedReport: report,
		},
		{
			name:           "the same rule is disabled for another cluster",
			clusterRules:   clusterDisabledRule("00000000-0000-0000-0000-000000000000", disabledRulesTestRuleID, criticalImpactErrorKey),
			expectedReport: report,
		},
		{
			name:           "the same rule is acked for another organization",
			orgRules:       orgDisabledRule("999", disabledRulesTestRuleID, criticalImpactErrorKey),
			expectedReport: report,
		},
		{
			name:           "another error key of the same rule is disabled",
			clusterRules:   clusterDisabledRule(cluster.ClusterName, disabledRulesTestRuleID, "TEST_RULE_OTHER_IMPACT"),
			expectedReport: report,
		},
		{
			name:           "another rule with the same error key is disabled",
			clusterRules:   clusterDisabledRule(cluster.ClusterName, "other_rule", criticalImpactErrorKey),
			expectedReport: report,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf := setupDisabledRulesLogCapture(t)

			d := differ.Differ{
				ClusterDisabledRules: tc.clusterRules,
				OrgDisabledRules:     tc.orgRules,
			}

			filtered := differ.FilterDisabledRulesFromReport(&d, cluster, report)

			nothingDisabled := tc.expectedReport == report
			assert.JSONEq(t, string(tc.expectedReport), string(filtered))
			if nothingDisabled {
				assert.Equal(t, report, filtered, "a report without disabled rule hits must be stored byte-identical")
			}

			executionLog := buf.String()
			assert.Equal(t, !nothingDisabled, strings.Contains(executionLog, differ.DisabledRulesOmittedMessage),
				"unexpected presence or absence of the omitted rule hits log")
			// an unchanged report must be the outcome of a lookup miss and never
			// of a filtering failure
			assert.NotContains(t, executionLog, differ.FilterReportFailedMessage)
		})
	}
}

// TestFilterDisabledRulesFromReportWithEveryRuleDisabled verifies that a report
// whose every rule hit is disabled is stored with an empty "reports" array
// rather than with a null, and that the rest of the document is intact. The two
// rule hits are disabled through a different map each, so a single pass over the
// report consults both of them.
func TestFilterDisabledRulesFromReportWithEveryRuleDisabled(t *testing.T) {
	cluster := disabledRulesTestClusterEntry()
	report := reportFilterDocument(reportFilterCriticalRuleHit, reportFilterImportantRuleHit)

	d := differ.Differ{
		ClusterDisabledRules: clusterDisabledRule(cluster.ClusterName, disabledRulesTestRuleID, criticalImpactErrorKey),
		OrgDisabledRules:     orgDisabledRule(reportFilterTestOrgID, disabledRulesTestRuleID, importantImpactErrorKey),
	}

	filtered := differ.FilterDisabledRulesFromReport(&d, cluster, report)

	assert.JSONEq(t, string(reportFilterDocument()), string(filtered),
		"every rule hit must be gone while the rest of the document stays intact")

	var document map[string]json.RawMessage
	err := json.Unmarshal([]byte(filtered), &document)
	assert.NoError(t, err)
	assert.Equal(t, "[]", string(document[differ.ReportsJSONField]),
		"an emptied report must be serialized as an empty array, not as null")
}

// TestFilterDisabledRulesFromReportIdentifiesRuleHitsByComponentAndKey verifies
// that a rule hit is matched against the aggregator maps through the rule name
// derived from its "component" and the error key taken from its "key", exactly
// like the Kafka and the Service Log filters do it. The composite "rule_id" of
// the report plays no part: types.ReportItem has no field mapped to it, so
// encoding/json discards it.
func TestFilterDisabledRulesFromReportIdentifiesRuleHitsByComponentAndKey(t *testing.T) {
	cluster := disabledRulesTestClusterEntry()

	// test_rule is disabled for this cluster with the critical error key
	d := differ.Differ{
		ClusterDisabledRules: clusterDisabledRule(cluster.ClusterName, disabledRulesTestRuleID, criticalImpactErrorKey),
	}

	t.Run("a composite rule_id contradicting the component is ignored", func(t *testing.T) {
		// the component still says test_rule, so the hit is disabled and has to
		// be omitted no matter what the composite rule_id claims
		hit := reportFilterRuleHit(disabledRulesTestModule, "nope|NOPE", criticalImpactErrorKey)
		report := reportFilterDocument(hit, reportFilterImportantRuleHit)

		filtered := differ.FilterDisabledRulesFromReport(&d, cluster, report)

		assert.JSONEq(t, string(reportFilterDocument(reportFilterImportantRuleHit)), string(filtered))
	})

	t.Run("a composite rule_id of a disabled rule does not disable the hit", func(t *testing.T) {
		// the component says other_rule, which is not disabled, so the hit stays
		// even though its composite rule_id is the disabled one
		hit := reportFilterRuleHit(otherRuleModule, criticalCompositeRuleID, criticalImpactErrorKey)
		report := reportFilterDocument(hit, reportFilterImportantRuleHit)

		filtered := differ.FilterDisabledRulesFromReport(&d, cluster, report)

		assert.Equal(t, report, filtered, "no rule hit of the report is disabled")
	})
}

// TestFilterDisabledRulesFromReportWithUnusableReport verifies that a report
// which cannot be filtered is stored as it is: keeping the original content is
// preferable to losing the record. Every case registers a disabled rule, so a
// well formed report would have been filtered.
func TestFilterDisabledRulesFromReportWithUnusableReport(t *testing.T) {
	cluster := disabledRulesTestClusterEntry()

	testCases := []struct {
		name             string
		report           types.ClusterReport
		expectErrorInLog bool
	}{
		{
			name:             "the document is not valid JSON",
			report:           types.ClusterReport(`{"reports": [`),
			expectErrorInLog: true,
		},
		{
			name:   "the document has no reports key",
			report: types.ClusterReport(`{"analysis_metadata": {}}`),
		},
		{
			name:             "the reports key does not hold an array",
			report:           types.ClusterReport(`{"reports": {"unexpected": "object"}}`),
			expectErrorInLog: true,
		},
		{
			name:             "a rule hit is not an object",
			report:           types.ClusterReport(fmt.Sprintf(`{"reports": [%s, 42]}`, reportFilterCriticalRuleHit)),
			expectErrorInLog: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf := setupDisabledRulesLogCapture(t)

			d := differ.Differ{
				ClusterDisabledRules: clusterDisabledRule(cluster.ClusterName, disabledRulesTestRuleID, criticalImpactErrorKey),
			}

			filtered := differ.FilterDisabledRulesFromReport(&d, cluster, tc.report)

			assert.Equal(t, tc.report, filtered, "an unusable report must be stored unchanged")
			assert.Equal(t, tc.expectErrorInLog, strings.Contains(buf.String(), differ.FilterReportFailedMessage),
				"unexpected presence or absence of the filtering failure log")
			assert.NotContains(t, buf.String(), differ.DisabledRulesOmittedMessage,
				"nothing may be reported as omitted when the report could not be filtered")
		})
	}
}

// TestFilterDisabledRulesFromReportWithoutDisabledRulesSkipsParsing pins the
// short circuit taken when the customer has not disabled any rule at all, which
// is also the case when --ignore-disabled-rules is set and both disabled rules
// maps are therefore empty. A report that cannot be parsed is what tells the
// short circuit apart from a full pass that happens to remove nothing: both
// store the original report, but only the full pass logs a parsing failure.
func TestFilterDisabledRulesFromReportWithoutDisabledRulesSkipsParsing(t *testing.T) {
	cluster := disabledRulesTestClusterEntry()

	// deliberately malformed so that any attempt to parse it is observable
	report := types.ClusterReport(`{"reports": [`)

	t.Run("both disabled rules maps are empty", func(t *testing.T) {
		buf := setupDisabledRulesLogCapture(t)

		d := differ.Differ{}

		filtered := differ.FilterDisabledRulesFromReport(&d, cluster, report)

		assert.Equal(t, report, filtered, "the report must be stored unchanged")
		assert.NotContains(t, buf.String(), differ.FilterReportFailedMessage,
			"the report must not be parsed when no rule is disabled")
	})

	t.Run("a disabled rule is registered", func(t *testing.T) {
		buf := setupDisabledRulesLogCapture(t)

		d := differ.Differ{
			ClusterDisabledRules: clusterDisabledRule(cluster.ClusterName, disabledRulesTestRuleID, criticalImpactErrorKey),
		}

		filtered := differ.FilterDisabledRulesFromReport(&d, cluster, report)

		assert.Equal(t, report, filtered, "the report must be stored unchanged")
		assert.Contains(t, buf.String(), differ.FilterReportFailedMessage,
			"the report must be parsed when a rule is disabled")
	})
}

// capturedNotificationRecord holds the arguments of the
// WriteNotificationRecordForCluster calls made during a processing run.
type capturedNotificationRecord struct {
	calls  int
	state  types.StateID
	report types.ClusterReport
	target types.EventTarget
}

// newCapturingStorageMock returns a Storage mock recording the state, the report
// and the event target of every WriteNotificationRecordForCluster call.
func newCapturingStorageMock(captured *capturedNotificationRecord) *mocks.Storage {
	storage := &mocks.Storage{}
	storage.On("WriteNotificationRecordForCluster",
		mock.AnythingOfType("types.ClusterEntry"),
		mock.AnythingOfType("types.NotificationTypeID"),
		mock.AnythingOfType("types.StateID"),
		mock.AnythingOfType("types.ClusterReport"),
		mock.AnythingOfType("types.Timestamp"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("types.EventTarget")).
		Run(func(args mock.Arguments) {
			captured.calls++
			captured.state = args.Get(2).(types.StateID)
			captured.report = args.Get(3).(types.ClusterReport)
			captured.target = args.Get(6).(types.EventTarget)
		}).
		Return(nil)
	return storage
}

// setupReportFilterStates installs the state IDs of the `states` table so that
// the state a record is written with can be told apart, and restores whatever
// was there when the test finishes.
func setupReportFilterStates(t *testing.T) {
	t.Helper()
	previous := *differ.States
	*differ.States = types.States{
		SentState:       1,
		SameState:       2,
		LowerIssueState: 3,
		ErrorState:      4,
	}
	t.Cleanup(func() { *differ.States = previous })
}

// reportFilterReportItems deserializes the rule hits of the given report the way
// processReportsByCluster does before handing them over to the notification
// targets.
func reportFilterReportItems(t *testing.T, report types.ClusterReport) types.ReportContent {
	t.Helper()
	var deserialized types.Report
	err := json.Unmarshal([]byte(report), &deserialized)
	assert.NoError(t, err)
	return deserialized.Reports
}

// TestProduceEntriesToKafkaStoresReportWithoutDisabledRules verifies that all
// three record writing paths of the Kafka target - no qualifying event ("same"),
// notification delivered ("sent") and delivery failed ("error") - store the
// report without the rule hits the customer has disabled. The critical rule is
// disabled for the cluster while the important one is active, which is the
// situation of the BDD scenario "only the re-enabled rule is notified when other
// rules are in cooldown": the state=1 record written there must not carry the
// disabled rule, otherwise ShouldNotify would still find it once it is
// re-enabled and would suppress the notification.
func TestProduceEntriesToKafkaStoresReportWithoutDisabledRules(t *testing.T) {
	setupReportFilterStates(t)
	cluster := disabledRulesTestClusterEntry()

	report := reportFilterDocument(reportFilterCriticalRuleHit, reportFilterImportantRuleHit)
	expectedStoredReport := reportFilterDocument(reportFilterImportantRuleHit)

	// the important rule was already notified, so nothing qualifies and the
	// "same" state path is taken
	importantRuleInCooldown := types.NotifiedRecordsPerCluster{
		{OrgID: cluster.OrgID, ClusterName: cluster.ClusterName}: {
			OrgID:       cluster.OrgID,
			ClusterName: cluster.ClusterName,
			Report:      reportFilterDocument(reportFilterImportantRuleHit),
		},
	}

	testCases := []struct {
		name               string
		previouslyReported types.NotifiedRecordsPerCluster
		produceError       error
		expectedState      types.StateID
		expectedNotified   int
		expectedError      bool
	}{
		{
			name:               "no event qualifies, the same state is stored",
			previouslyReported: importantRuleInCooldown,
			expectedState:      differ.States.SameState,
		},
		{
			name:             "the notification is delivered, the sent state is stored",
			expectedState:    differ.States.SentState,
			expectedNotified: 1,
		},
		{
			name:             "the delivery fails, the error state is stored",
			produceError:     fmt.Errorf("kafka is not reachable"),
			expectedState:    differ.States.ErrorState,
			expectedNotified: -1,
			expectedError:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			captured := capturedNotificationRecord{}
			storage := newCapturingStorageMock(&captured)

			producerMock := &mocks.Producer{}
			producerMock.On("ProduceMessage", mock.AnythingOfType("types.ProducerMessage")).
				Return(int32(0), int64(1), tc.produceError)

			d := differ.Differ{
				Storage:  storage,
				Notifier: producerMock,
				Filter:   differ.DefaultEventFilter,
				Thresholds: differ.EventThresholds{
					TotalRisk: differ.DefaultTotalRiskThreshold,
				},
				ClusterDisabledRules: clusterDisabledRule(cluster.ClusterName, disabledRulesTestRuleID, criticalImpactErrorKey),
				PreviouslyReported:   tc.previouslyReported,
			}

			notified, err := differ.ProduceEntriesToKafka(&d, cluster,
				disabledRulesTestRuleContent(), reportFilterReportItems(t, report), report)

			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.expectedNotified, notified)

			assert.Equal(t, 1, captured.calls, "exactly one record must be written")
			assert.Equal(t, tc.expectedState, captured.state)
			assert.Equal(t, types.NotificationBackendTarget, captured.target)
			assert.JSONEq(t, string(expectedStoredReport), string(captured.report),
				"the stored report must not contain the disabled rule hit")
		})
	}
}

// TestProcessClustersServiceLogStoresReportWithoutDisabledRules verifies that the
// Service Log target stores the report without the rule hits the customer has
// disabled too. The filtering cannot happen in ProduceEntriesToServiceLog, which
// neither receives nor returns the report, so this exercises the whole
// processReportsByCluster loop: the critical rule is disabled for the cluster,
// the important one is sent to Service Log, and the resulting "sent" record must
// carry the important rule only.
func TestProcessClustersServiceLogStoresReportWithoutDisabledRules(t *testing.T) {
	setupReportFilterStates(t)
	cluster := disabledRulesTestClusterEntry()

	report := reportFilterDocument(reportFilterCriticalRuleHit, reportFilterImportantRuleHit)
	expectedStoredReport := reportFilterDocument(reportFilterImportantRuleHit)

	renderer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := fmt.Fprintf(w,
			`{"clusters":[%q],"reports":{%q:[{"rule_id":%q,"error_key":%q,"resolution":"resolution","reason":"reason","description":"description"}]}}`,
			cluster.ClusterName, cluster.ClusterName, disabledRulesTestRuleID, importantImpactErrorKey)
		assert.NoError(t, err)
	}))
	defer renderer.Close()

	config := conf.ConfigStruct{
		ServiceLog: conf.ServiceLogConfiguration{
			Enabled:            true,
			TotalRiskThreshold: differ.DefaultTotalRiskThreshold,
			EventFilter:        differ.DefaultEventFilter,
		},
		Dependencies: conf.DependenciesConfiguration{
			TemplateRendererServer:   renderer.URL,
			TemplateRendererEndpoint: "/rendered_reports",
			TemplateRendererURL:      renderer.URL + "/rendered_reports",
		},
	}

	captured := capturedNotificationRecord{}
	storage := newCapturingStorageMock(&captured)
	storage.On("ReadReportForClusterAtTime",
		mock.AnythingOfType("types.OrgID"),
		mock.AnythingOfType("types.ClusterName"),
		mock.AnythingOfType("types.Timestamp")).Return(report, nil)

	producerMock := &mocks.Producer{}
	producerMock.On("ProduceMessage", mock.AnythingOfType("types.ProducerMessage")).
		Return(int32(0), int64(1), nil)

	d := differ.Differ{
		Storage:          storage,
		Notifier:         producerMock,
		NotificationType: types.InstantNotif,
		Target:           types.ServiceLogTarget,
		Filter:           differ.DefaultEventFilter,
		Thresholds: differ.EventThresholds{
			TotalRisk: differ.DefaultTotalRiskThreshold,
		},
		ClusterDisabledRules: clusterDisabledRule(cluster.ClusterName, disabledRulesTestRuleID, criticalImpactErrorKey),
	}

	d.ProcessClusters(&config, disabledRulesTestRuleContent(), []types.ClusterEntry{cluster})

	assert.Equal(t, 1, captured.calls, "exactly one record must be written")
	assert.Equal(t, differ.States.SentState, captured.state)
	assert.Equal(t, types.ServiceLogTarget, captured.target)
	assert.JSONEq(t, string(expectedStoredReport), string(captured.report),
		"the stored report must not contain the disabled rule hit")
	producerMock.AssertNumberOfCalls(t, "ProduceMessage", 1)
}
