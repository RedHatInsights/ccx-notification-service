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

// Unit tests for the disabled-rules filtering added to the Service Log
// processing path (CCXDEV-16568). They cover getReportsWithIssuesToNotify,
// where the disabled check has to be the first thing evaluated for every rule
// so that a rule the customer has disabled - either for the single cluster
// (cluster_rule_toggle) or for the whole organization (rule_disable) - never
// reaches the total risk filter or ShouldNotify.

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"

	utypes "github.com/RedHatInsights/insights-results-types"

	"github.com/RedHatInsights/ccx-notification-service/differ"
	"github.com/RedHatInsights/ccx-notification-service/types"
)

const (
	// lowImpactErrorKey identifies a rule variant whose total risk (1) is below
	// the default total risk threshold (2).
	lowImpactErrorKey types.ErrorKey = "TEST_RULE_LOW_IMPACT"

	lowRuleDescription = "low rule description"

	// criticalRuleTotalRisk and importantRuleTotalRisk are the total risks
	// computed from the impact and likelihood in disabledRulesTestRuleContent.
	criticalRuleTotalRisk  = 4
	importantRuleTotalRisk = 3

	// lowRuleTotalRisk is the total risk computed from the impact and the
	// likelihood registered by serviceLogTestRuleContentWithLowRisk.
	lowRuleTotalRisk = 1

	// serviceLogDisabledRulesReportJSON is a report as it is stored in the
	// database, where a rule carries the composite "rule_id|error_key" value
	// next to the fully qualified component and the error key. The aggregator
	// tables, in contrast, store rule_id ("test_rule") and error_key in
	// separate columns. Note that types.ReportItem has no field mapped to the
	// "rule_id" JSON key, so encoding/json discards it: matching runs off
	// "component" (via moduleToRuleName) and "key" instead.
	serviceLogDisabledRulesReportJSON = `{
		"reports": [
			{
				"rule_id": "test_rule|TEST_RULE_CRITICAL_IMPACT",
				"component": "ccx_rules_ocp.external.rules.test_rule.report",
				"type": "rule",
				"key": "TEST_RULE_CRITICAL_IMPACT",
				"details": {},
				"tags": [],
				"links": {}
			},
			{
				"rule_id": "test_rule|TEST_RULE_IMPORTANT_IMPACT",
				"component": "ccx_rules_ocp.external.rules.test_rule.report",
				"type": "rule",
				"key": "TEST_RULE_IMPORTANT_IMPACT",
				"details": {},
				"tags": [],
				"links": {}
			}
		]
	}`
)

// newServiceLogDisabledRulesDiffer returns a Differ using the default event
// filter and total risk threshold, with the given disabled-rule maps loaded
// from the aggregator database.
func newServiceLogDisabledRulesDiffer(
	clusterDisabledRules types.ClusterDisabledRules,
	orgDisabledRules types.OrgDisabledRules) differ.Differ {
	return differ.Differ{
		Filter: differ.DefaultEventFilter,
		Thresholds: differ.EventThresholds{
			TotalRisk: differ.DefaultTotalRiskThreshold,
		},
		ClusterDisabledRules: clusterDisabledRules,
		OrgDisabledRules:     orgDisabledRules,
	}
}

// serviceLogTestRuleContentWithLowRisk extends the shared test rule content
// with an error key whose total risk is below the default threshold.
func serviceLogTestRuleContentWithLowRisk() types.RulesMap {
	ruleContent := disabledRulesTestRuleContent()
	ruleContent[string(disabledRulesTestRuleID)].ErrorKeys[string(lowImpactErrorKey)] = utypes.RuleErrorKeyContent{
		Metadata: utypes.ErrorKeyMetadata{
			Description: lowRuleDescription,
			Impact:      utypes.Impact{Name: "low_impact", Impact: 1},
			Likelihood:  1,
		},
	}
	return ruleContent
}

// TestGetReportsWithIssuesToNotifySkipsClusterDisabledRule verifies that a rule
// disabled for the cluster (cluster_rule_toggle) is excluded from the reports
// to be sent to Service Log, and that it is skipped before the total risk
// filter and ShouldNotify even though its total risk is above the threshold.
func TestGetReportsWithIssuesToNotifySkipsClusterDisabledRule(t *testing.T) {
	buf := setupDisabledRulesLogCapture(t)
	cluster := disabledRulesTestClusterEntry()

	d := newServiceLogDisabledRulesDiffer(
		types.ClusterDisabledRules{
			{ClusterID: cluster.ClusterName, RuleID: disabledRulesTestRuleID, ErrorKey: criticalImpactErrorKey}: {},
		},
		nil)

	reports := types.ReportContent{disabledRulesTestReportItem(criticalImpactErrorKey)}

	reportsWithIssues := differ.GetReportsWithIssuesToNotify(&d, reports, cluster, disabledRulesTestRuleContent())

	assert.Empty(t, reportsWithIssues, "a cluster-disabled rule must not be sent to Service Log")

	executionLog := buf.String()
	assert.Contains(t, executionLog, differ.DisabledRuleSkippedMessage)
	assert.NotContains(t, executionLog, shouldNotifyLogMessage, "disabled rule must not reach ShouldNotify")
	assert.NotContains(t, executionLog, differ.ReportWithHighImpactMessage, "disabled rule must not reach the notify path")
}

// TestGetReportsWithIssuesToNotifySkipsOrgDisabledRule verifies that a rule
// acked for the whole organization (rule_disable) is excluded from the reports
// to be sent to Service Log.
func TestGetReportsWithIssuesToNotifySkipsOrgDisabledRule(t *testing.T) {
	buf := setupDisabledRulesLogCapture(t)
	cluster := disabledRulesTestClusterEntry()

	d := newServiceLogDisabledRulesDiffer(
		nil,
		types.OrgDisabledRules{
			{OrgID: "1", RuleID: disabledRulesTestRuleID, ErrorKey: criticalImpactErrorKey}: {},
		})

	reports := types.ReportContent{disabledRulesTestReportItem(criticalImpactErrorKey)}

	reportsWithIssues := differ.GetReportsWithIssuesToNotify(&d, reports, cluster, disabledRulesTestRuleContent())

	assert.Empty(t, reportsWithIssues, "an org-disabled rule must not be sent to Service Log")

	executionLog := buf.String()
	assert.Contains(t, executionLog, differ.DisabledRuleSkippedMessage)
	assert.NotContains(t, executionLog, shouldNotifyLogMessage, "disabled rule must not reach ShouldNotify")
}

// TestGetReportsWithIssuesToNotifyIncludesRuleNotDisabled verifies that a rule
// that matches neither disabled-rule map is returned with its total risk set.
// The maps contain near misses only: the same rule with another error key for
// this cluster and the very same rule acked for a different organization,
// neither of which may cause the rule to be skipped.
func TestGetReportsWithIssuesToNotifyIncludesRuleNotDisabled(t *testing.T) {
	buf := setupDisabledRulesLogCapture(t)
	cluster := disabledRulesTestClusterEntry()

	d := newServiceLogDisabledRulesDiffer(
		types.ClusterDisabledRules{
			{ClusterID: cluster.ClusterName, RuleID: disabledRulesTestRuleID, ErrorKey: importantImpactErrorKey}: {},
		},
		types.OrgDisabledRules{
			{OrgID: "999", RuleID: disabledRulesTestRuleID, ErrorKey: criticalImpactErrorKey}: {},
		})

	reports := types.ReportContent{disabledRulesTestReportItem(criticalImpactErrorKey)}

	reportsWithIssues := differ.GetReportsWithIssuesToNotify(&d, reports, cluster, disabledRulesTestRuleContent())

	assert.Len(t, reportsWithIssues, 1, "a rule that is not disabled must be sent to Service Log")
	assert.Equal(t, disabledRulesTestModule, reportsWithIssues[0].Module)
	assert.Equal(t, criticalImpactErrorKey, reportsWithIssues[0].ErrorKey)
	assert.Equal(t, criticalRuleTotalRisk, reportsWithIssues[0].TotalRisk)

	executionLog := buf.String()
	assert.NotContains(t, executionLog, differ.DisabledRuleSkippedMessage, "an enabled rule must not be reported as disabled")
	assert.Contains(t, executionLog, differ.ReportWithHighImpactMessage)
}

// TestGetReportsWithIssuesToNotifySkipsOnlyDisabledRuleAmongMany verifies that
// when a report contains several rules and only one of them is disabled, the
// disabled rule is skipped while the remaining ones are still sent. Mirrors the
// BDD scenario "only the re-enabled rule is sent to service log when other
// rules are in cooldown".
func TestGetReportsWithIssuesToNotifySkipsOnlyDisabledRuleAmongMany(t *testing.T) {
	buf := setupDisabledRulesLogCapture(t)
	cluster := disabledRulesTestClusterEntry()

	d := newServiceLogDisabledRulesDiffer(
		types.ClusterDisabledRules{
			{ClusterID: cluster.ClusterName, RuleID: disabledRulesTestRuleID, ErrorKey: criticalImpactErrorKey}: {},
		},
		nil)

	reports := types.ReportContent{
		disabledRulesTestReportItem(criticalImpactErrorKey),
		disabledRulesTestReportItem(importantImpactErrorKey),
	}

	reportsWithIssues := differ.GetReportsWithIssuesToNotify(&d, reports, cluster, disabledRulesTestRuleContent())

	assert.Len(t, reportsWithIssues, 1, "only the rule that is not disabled must be sent to Service Log")
	assert.Equal(t, importantImpactErrorKey, reportsWithIssues[0].ErrorKey,
		"the only remaining rule must be the important (not disabled) one")
	assert.Equal(t, importantRuleTotalRisk, reportsWithIssues[0].TotalRisk)
	assert.Contains(t, buf.String(), differ.DisabledRuleSkippedMessage)
}

// TestGetReportsWithIssuesToNotifyMatchesRulesFromStoredReportJSON verifies
// that rules deserialized from a report as it is stored in the database are
// matched against the aggregator maps, which hold the plain rule id and the
// error key in separate columns. The report's composite "rule_id|error_key"
// field plays no part in this: types.ReportItem has no field mapped to it, so
// encoding/json discards it and matching runs off "component" (via
// moduleToRuleName) and "key".
func TestGetReportsWithIssuesToNotifyMatchesRulesFromStoredReportJSON(t *testing.T) {
	cluster := disabledRulesTestClusterEntry()

	testCases := []struct {
		name       string
		reportJSON string
	}{
		{
			name:       "composite rule_id agreeing with the component",
			reportJSON: serviceLogDisabledRulesReportJSON,
		},
		{
			// A bogus rule_id must not change the outcome, because it is never
			// read. If this case ever fails, something started parsing rule_id
			// and both cases need revisiting.
			name:       "bogus composite rule_id is ignored",
			reportJSON: strings.ReplaceAll(serviceLogDisabledRulesReportJSON, "test_rule|", "nope|"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var report types.Report
			err := json.Unmarshal([]byte(tc.reportJSON), &report)
			assert.NoError(t, err)
			assert.Len(t, report.Reports, 2, "both rules must be deserialized from the report")

			d := newServiceLogDisabledRulesDiffer(
				types.ClusterDisabledRules{
					{ClusterID: cluster.ClusterName, RuleID: disabledRulesTestRuleID, ErrorKey: criticalImpactErrorKey}: {},
				},
				nil)

			reportsWithIssues := differ.GetReportsWithIssuesToNotify(&d, report.Reports, cluster, disabledRulesTestRuleContent())

			if assert.Len(t, reportsWithIssues, 1, "the rule disabled in the aggregator tables must be matched and skipped") {
				assert.Equal(t, importantImpactErrorKey, reportsWithIssues[0].ErrorKey)
			}
		})
	}
}

// TestGetReportsWithIssuesToNotifyKeepsFilteringLowTotalRisk verifies that the
// disabled check did not replace the total risk filter: a rule that is not
// disabled but whose total risk is below the threshold is still excluded, and
// it is not reported as disabled.
func TestGetReportsWithIssuesToNotifyKeepsFilteringLowTotalRisk(t *testing.T) {
	cluster := disabledRulesTestClusterEntry()

	testCases := []struct {
		name          string
		totalRisk     int
		expectedCount int
	}{
		{
			name:          "total risk below the threshold is filtered out",
			totalRisk:     differ.DefaultTotalRiskThreshold,
			expectedCount: 0,
		},
		{
			// Lowering the threshold to the rule's own total risk must let it
			// through. This is what tells "filtered out because the risk is
			// below the threshold" apart from "filtered out because the error
			// key was never registered in the rule content" - the latter
			// resolves to total risk 0 and would stay excluded here too.
			name:          "total risk at the threshold is kept",
			totalRisk:     lowRuleTotalRisk,
			expectedCount: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf := setupDisabledRulesLogCapture(t)

			d := newServiceLogDisabledRulesDiffer(nil, nil)
			d.Thresholds.TotalRisk = tc.totalRisk

			reports := types.ReportContent{disabledRulesTestReportItem(lowImpactErrorKey)}

			reportsWithIssues := differ.GetReportsWithIssuesToNotify(&d, reports, cluster, serviceLogTestRuleContentWithLowRisk())

			if assert.Len(t, reportsWithIssues, tc.expectedCount) && tc.expectedCount > 0 {
				assert.Equal(t, lowRuleTotalRisk, reportsWithIssues[0].TotalRisk)
			}
			assert.NotContains(t, buf.String(), differ.DisabledRuleSkippedMessage,
				"a rule filtered out by the total risk threshold must not be reported as disabled")
		})
	}
}

// TestGetReportsWithIssuesToNotifyDisabledRuleSkippedBeforeCooldownCheck
// verifies that a disabled rule is skipped before ShouldNotify is consulted.
// The cluster has a previously notified report containing the rule, so if the
// rule reached ShouldNotify it would be counted as "not sent, same state". The
// enabled case shows the counter does increase when ShouldNotify is reached,
// which makes the missing increment in the disabled case meaningful.
func TestGetReportsWithIssuesToNotifyDisabledRuleSkippedBeforeCooldownCheck(t *testing.T) {
	cluster := disabledRulesTestClusterEntry()

	previouslyReported := types.NotifiedRecordsPerCluster{
		{OrgID: cluster.OrgID, ClusterName: cluster.ClusterName}: {
			OrgID:       cluster.OrgID,
			ClusterName: cluster.ClusterName,
			Report:      types.ClusterReport(serviceLogDisabledRulesReportJSON),
		},
	}

	testCases := []struct {
		name                  string
		clusterDisabledRules  types.ClusterDisabledRules
		expectedSameStateDiff int
		expectDisabledLog     bool
	}{
		{
			name: "disabled rule is skipped before the cooldown check",
			clusterDisabledRules: types.ClusterDisabledRules{
				{ClusterID: cluster.ClusterName, RuleID: disabledRulesTestRuleID, ErrorKey: criticalImpactErrorKey}: {},
			},
			expectedSameStateDiff: 0,
			expectDisabledLog:     true,
		},
		{
			name:                  "enabled rule reaches the cooldown check",
			expectedSameStateDiff: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			buf := setupDisabledRulesLogCapture(t)

			d := newServiceLogDisabledRulesDiffer(tc.clusterDisabledRules, nil)
			d.PreviouslyReported = previouslyReported

			reports := types.ReportContent{disabledRulesTestReportItem(criticalImpactErrorKey)}

			sameStateBefore := int(testutil.ToFloat64(differ.NotificationNotSentSameState))
			reportsWithIssues := differ.GetReportsWithIssuesToNotify(&d, reports, cluster, disabledRulesTestRuleContent())
			sameStateDiff := int(testutil.ToFloat64(differ.NotificationNotSentSameState)) - sameStateBefore

			assert.Empty(t, reportsWithIssues, "the rule must not be sent to Service Log")
			assert.Equal(t, tc.expectedSameStateDiff, sameStateDiff,
				"unexpected NotificationNotSentSameState metric increment")
			assert.Equal(t, tc.expectDisabledLog,
				strings.Contains(buf.String(), differ.DisabledRuleSkippedMessage),
				"unexpected presence or absence of the disabled-rule skip log")
		})
	}
}
