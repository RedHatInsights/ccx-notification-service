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

import (
	"github.com/RedHatInsights/ccx-notification-service/types"
	"testing"
)

func BenchmarkFindRenderedReport(b *testing.B) {
	reports := []types.RenderedReport{
		types.RenderedReport{
			RuleID:   "rule_1",
			ErrorKey: "RULE_1",
		},
		types.RenderedReport{
			RuleID:   "rule_2",
			ErrorKey: "RULE_2",
		},
		types.RenderedReport{
			RuleID:   "rule_3",
			ErrorKey: "RULE_3",
		},
		types.RenderedReport{
			RuleID:   "rule_4",
			ErrorKey: "RULE_4",
		},
	}
	ruleName := types.RuleName("rule_4")
	errorKey := types.ErrorKey("RULE_4")
	for i := 0; i < b.N; i++ {
		_, err := findRenderedReport(reports, ruleName, errorKey)
		if err != nil {
			b.Fatal("Given key could not be found in benchmark reports")
		}
	}
}
