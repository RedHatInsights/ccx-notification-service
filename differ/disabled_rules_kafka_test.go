/*
Copyright © 2025, 2026 Red Hat, Inc.

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

// Unit tests for the disabled-rules filtering added to the Kafka processing
// path (CCXDEV-16567). They cover the isRuleDisabled helper directly and its
// use as the first check inside produceEntriesToKafka, so that a rule the
// customer has disabled never reaches the total risk filter or ShouldNotify.

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	utypes "github.com/RedHatInsights/insights-results-types"

	"github.com/RedHatInsights/ccx-notification-service/differ"
	"github.com/RedHatInsights/ccx-notification-service/tests/mocks"
	"github.com/RedHatInsights/ccx-notification-service/types"
)

const (
	// disabledRulesTestModule is the fully qualified component as it appears in
	// the report JSON. moduleToRuleName turns it into the plain rule name
	// "test_rule" that the aggregator tables store as rule_id.
	disabledRulesTestModule types.ModuleName = "ccx_rules_ocp.external.rules.test_rule.report"
	disabledRulesTestRuleID types.RuleID     = "test_rule"

	criticalImpactErrorKey  types.ErrorKey = "TEST_RULE_CRITICAL_IMPACT"
	importantImpactErrorKey types.ErrorKey = "TEST_RULE_IMPORTANT_IMPACT"

	criticalRuleDescription  = "critical rule description"
	importantRuleDescription = "important rule description"

	// shouldNotifyLogMessage is logged by ShouldNotify when a rule reaches the
	// cooldown check and is deemed notifiable. Its absence proves a rule was
	// skipped before ShouldNotify.
	shouldNotifyLogMessage = "Should notify user"
)

// disabledRulesTestClusterEntry returns a cluster in org 1 used across the
// Kafka disabled-rules tests.
func disabledRulesTestClusterEntry() types.ClusterEntry {
	return types.ClusterEntry{
		OrgID:         1,
		AccountNumber: 1,
		ClusterName:   "5d5892d4-2g85-4ccf-02bg-548dfc9767aa",
		UpdatedAt:     types.Timestamp(testTimestamp),
	}
}

// disabledRulesTestRuleContent returns rule content for test_rule with a
// critical (total risk 4) and an important (total risk 3) error key.
func disabledRulesTestRuleContent() types.RulesMap {
	return types.RulesMap{
		string(disabledRulesTestRuleID): {
			ErrorKeys: map[string]utypes.RuleErrorKeyContent{
				string(criticalImpactErrorKey): {
					Metadata: utypes.ErrorKeyMetadata{
						Description: criticalRuleDescription,
						Impact:      utypes.Impact{Name: "critical_impact", Impact: 4},
						Likelihood:  4,
					},
				},
				string(importantImpactErrorKey): {
					Metadata: utypes.ErrorKeyMetadata{
						Description: importantRuleDescription,
						Impact:      utypes.Impact{Name: "important_impact", Impact: 4},
						Likelihood:  2,
					},
				},
			},
		},
	}
}

// disabledRulesTestReportItem builds a single report item for test_rule with
// the given error key, mirroring how the report JSON encodes component and key.
func disabledRulesTestReportItem(errorKey types.ErrorKey) *types.EvaluatedReportItem {
	return &types.EvaluatedReportItem{
		ReportItem: types.ReportItem{
			Type:     "rule",
			Module:   disabledRulesTestModule,
			ErrorKey: errorKey,
		},
	}
}

// newDisabledRulesStorageMock returns a Storage mock that accepts any
// WriteNotificationRecordForCluster call (used to record the per-cluster state).
func newDisabledRulesStorageMock() *mocks.Storage {
	storage := &mocks.Storage{}
	storage.On("WriteNotificationRecordForCluster",
		mock.AnythingOfType("types.ClusterEntry"),
		mock.AnythingOfType("types.NotificationTypeID"),
		mock.AnythingOfType("types.StateID"),
		mock.AnythingOfType("types.ClusterReport"),
		mock.AnythingOfType("types.Timestamp"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("types.EventTarget")).Return(nil)
	return storage
}

// setupDisabledRulesLogCapture redirects zerolog to an in-memory buffer at
// debug level and restores the default level when the test finishes.
func setupDisabledRulesLogCapture(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := new(bytes.Buffer)
	log.Logger = zerolog.New(buf).Level(zerolog.DebugLevel)
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	t.Cleanup(func() { zerolog.SetGlobalLevel(zerolog.WarnLevel) })
	return buf
}

// TestIsRuleDisabled verifies that isRuleDisabled consults both the per-cluster
// (cluster_rule_toggle) and org-wide (rule_disable) maps and matches on every
// component of the composite key: cluster/org id, rule id (derived from the
// module) and error key.
func TestIsRuleDisabled(t *testing.T) {
	cluster := disabledRulesTestClusterEntry()
	const ruleName = types.RuleName("test_rule")
	errorKey := criticalImpactErrorKey

	clusterKey := types.ClusterRuleKey{
		ClusterID: cluster.ClusterName,
		RuleID:    disabledRulesTestRuleID,
		ErrorKey:  errorKey,
	}
	orgKey := types.OrgRuleKey{
		OrgID:    "1",
		RuleID:   disabledRulesTestRuleID,
		ErrorKey: errorKey,
	}

	testCases := []struct {
		name     string
		cluster  types.ClusterDisabledRules
		org      types.OrgDisabledRules
		expected bool
	}{
		{
			name:     "rule disabled at cluster level",
			cluster:  types.ClusterDisabledRules{clusterKey: {}},
			expected: true,
		},
		{
			name:     "rule disabled at org level",
			org:      types.OrgDisabledRules{orgKey: {}},
			expected: true,
		},
		{
			name:     "rule not disabled in either map",
			expected: false,
		},
		{
			name: "cluster map entry has a different error key",
			cluster: types.ClusterDisabledRules{
				{ClusterID: cluster.ClusterName, RuleID: disabledRulesTestRuleID, ErrorKey: "OTHER_ERROR_KEY"}: {},
			},
			expected: false,
		},
		{
			name: "cluster map entry has a different rule id",
			cluster: types.ClusterDisabledRules{
				{ClusterID: cluster.ClusterName, RuleID: "other_rule", ErrorKey: errorKey}: {},
			},
			expected: false,
		},
		{
			name: "cluster map entry is for a different cluster",
			cluster: types.ClusterDisabledRules{
				{ClusterID: "other-cluster", RuleID: disabledRulesTestRuleID, ErrorKey: errorKey}: {},
			},
			expected: false,
		},
		{
			name: "org map entry is for a different org",
			org: types.OrgDisabledRules{
				{OrgID: "999", RuleID: disabledRulesTestRuleID, ErrorKey: errorKey}: {},
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			d := differ.Differ{
				ClusterDisabledRules: tc.cluster,
				OrgDisabledRules:     tc.org,
			}
			result := differ.IsRuleDisabled(&d, cluster, ruleName, errorKey)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestProduceEntriesToKafkaSkipsClusterDisabledRule verifies that a rule
// disabled for the cluster (cluster_rule_toggle) is skipped before the total
// risk filter and ShouldNotify, so no notification event is produced even
// though the rule has a total risk above the threshold and no cooldown record
// exists (which would otherwise notify).
func TestProduceEntriesToKafkaSkipsClusterDisabledRule(t *testing.T) {
	buf := setupDisabledRulesLogCapture(t)
	cluster := disabledRulesTestClusterEntry()

	storage := newDisabledRulesStorageMock()
	producerMock := &mocks.Producer{}
	producerMock.Test(t)

	d := differ.Differ{
		Storage:  storage,
		Notifier: producerMock,
		Filter:   differ.DefaultEventFilter,
		Thresholds: differ.EventThresholds{
			TotalRisk: differ.DefaultTotalRiskThreshold,
		},
		ClusterDisabledRules: types.ClusterDisabledRules{
			{ClusterID: cluster.ClusterName, RuleID: disabledRulesTestRuleID, ErrorKey: criticalImpactErrorKey}: {},
		},
	}

	reportItems := types.ReportContent{disabledRulesTestReportItem(criticalImpactErrorKey)}

	notified, err := differ.ProduceEntriesToKafka(&d, cluster, disabledRulesTestRuleContent(), reportItems, "{}")

	assert.NoError(t, err)
	assert.Equal(t, 0, notified, "a cluster-disabled rule must not produce any notification event")

	executionLog := buf.String()
	assert.Contains(t, executionLog, differ.DisabledRuleSkippedMessage)
	// The disabled rule must never reach the total risk filter or ShouldNotify.
	assert.NotContains(t, executionLog, shouldNotifyLogMessage, "disabled rule must not reach ShouldNotify")
	assert.NotContains(t, executionLog, differ.ReportWithHighImpactMessage, "disabled rule must not reach the notify path")
	producerMock.AssertNotCalled(t, "ProduceMessage", mock.Anything)
}

// TestProduceEntriesToKafkaSkipsOrgDisabledRule verifies that a rule acked
// org-wide (rule_disable) is skipped in produceEntriesToKafka, producing no
// notification event.
func TestProduceEntriesToKafkaSkipsOrgDisabledRule(t *testing.T) {
	buf := setupDisabledRulesLogCapture(t)
	cluster := disabledRulesTestClusterEntry()

	storage := newDisabledRulesStorageMock()
	producerMock := &mocks.Producer{}
	producerMock.Test(t)

	d := differ.Differ{
		Storage:  storage,
		Notifier: producerMock,
		Filter:   differ.DefaultEventFilter,
		Thresholds: differ.EventThresholds{
			TotalRisk: differ.DefaultTotalRiskThreshold,
		},
		OrgDisabledRules: types.OrgDisabledRules{
			{OrgID: "1", RuleID: disabledRulesTestRuleID, ErrorKey: criticalImpactErrorKey}: {},
		},
	}

	reportItems := types.ReportContent{disabledRulesTestReportItem(criticalImpactErrorKey)}

	notified, err := differ.ProduceEntriesToKafka(&d, cluster, disabledRulesTestRuleContent(), reportItems, "{}")

	assert.NoError(t, err)
	assert.Equal(t, 0, notified, "an org-disabled rule must not produce any notification event")

	executionLog := buf.String()
	assert.Contains(t, executionLog, differ.DisabledRuleSkippedMessage)
	producerMock.AssertNotCalled(t, "ProduceMessage", mock.Anything)
}

// TestProduceEntriesToKafkaNotifiesRuleNotDisabled verifies that a rule absent
// from both disabled-rule maps proceeds through the total risk filter and is
// notified.
func TestProduceEntriesToKafkaNotifiesRuleNotDisabled(t *testing.T) {
	buf := setupDisabledRulesLogCapture(t)
	cluster := disabledRulesTestClusterEntry()

	storage := newDisabledRulesStorageMock()
	producerMock := &mocks.Producer{}
	producerMock.On("ProduceMessage", mock.AnythingOfType("types.ProducerMessage")).
		Return(int32(0), int64(1), nil)

	// ClusterDisabledRules and OrgDisabledRules are left nil: no rule is disabled.
	d := differ.Differ{
		Storage:  storage,
		Notifier: producerMock,
		Filter:   differ.DefaultEventFilter,
		Thresholds: differ.EventThresholds{
			TotalRisk: differ.DefaultTotalRiskThreshold,
		},
	}

	reportItems := types.ReportContent{disabledRulesTestReportItem(criticalImpactErrorKey)}

	notified, err := differ.ProduceEntriesToKafka(&d, cluster, disabledRulesTestRuleContent(), reportItems, "{}")

	assert.NoError(t, err)
	assert.Equal(t, 1, notified, "a rule that is not disabled must be notified")

	executionLog := buf.String()
	assert.NotContains(t, executionLog, differ.DisabledRuleSkippedMessage, "an enabled rule must not be reported as disabled")
	assert.Contains(t, executionLog, differ.ReportWithHighImpactMessage)
	producerMock.AssertCalled(t, "ProduceMessage", mock.AnythingOfType("types.ProducerMessage"))
}

// TestProduceEntriesToKafkaSkipsOnlyDisabledRuleAmongMany verifies that when a
// report has several rules and only one is disabled, the disabled rule is
// skipped while the others are still notified. Mirrors the BDD scenario "only
// the re-enabled rule is notified when other rules are in cooldown".
func TestProduceEntriesToKafkaSkipsOnlyDisabledRuleAmongMany(t *testing.T) {
	buf := setupDisabledRulesLogCapture(t)
	cluster := disabledRulesTestClusterEntry()

	storage := newDisabledRulesStorageMock()

	var capturedMessage types.ProducerMessage
	producerMock := &mocks.Producer{}
	producerMock.On("ProduceMessage", mock.AnythingOfType("types.ProducerMessage")).
		Run(func(args mock.Arguments) {
			capturedMessage = args.Get(0).(types.ProducerMessage)
		}).
		Return(int32(0), int64(1), nil)

	d := differ.Differ{
		Storage:  storage,
		Notifier: producerMock,
		Filter:   differ.DefaultEventFilter,
		Thresholds: differ.EventThresholds{
			TotalRisk: differ.DefaultTotalRiskThreshold,
		},
		ClusterDisabledRules: types.ClusterDisabledRules{
			{ClusterID: cluster.ClusterName, RuleID: disabledRulesTestRuleID, ErrorKey: criticalImpactErrorKey}: {},
		},
	}

	reportItems := types.ReportContent{
		disabledRulesTestReportItem(criticalImpactErrorKey),
		disabledRulesTestReportItem(importantImpactErrorKey),
	}

	notified, err := differ.ProduceEntriesToKafka(&d, cluster, disabledRulesTestRuleContent(), reportItems, "{}")

	assert.NoError(t, err)
	assert.Equal(t, 1, notified, "only the rule that is not disabled must be notified")
	assert.Contains(t, buf.String(), differ.DisabledRuleSkippedMessage)

	var message types.NotificationMessage
	err = json.Unmarshal(capturedMessage, &message)
	assert.NoError(t, err)
	assert.Len(t, message.Events, 1, "notification must contain exactly one event")
	assert.Equal(t, importantRuleDescription, message.Events[0].Payload["rule_description"],
		"the only notified rule must be the important (not disabled) rule")
	assert.Equal(t, "3", message.Events[0].Payload["total_risk"],
		"the important rule has total risk 3")
}
