/*
Copyright © 2023 Red Hat, Inc.

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
// https://redhatinsights.github.io/ccx-notification-writer/packages/differ/differ_cli_args_test.html

import (
	"testing"

	"github.com/RedHatInsights/ccx-notification-service/differ"

	"github.com/stretchr/testify/assert"

	"github.com/RedHatInsights/ccx-notification-service/types"
)

func TestDeleteOperationSpecified(t *testing.T) {
	testcases := []struct {
		cliFlags types.CliFlags
		want     bool
	}{
		{
			cliFlags: types.CliFlags{
				PrintNewReportsForCleanup: true,
				PerformNewReportsCleanup:  true,
				PrintOldReportsForCleanup: true,
				PerformOldReportsCleanup:  true,
			},
			want: true,
		},
		{
			cliFlags: types.CliFlags{
				PrintNewReportsForCleanup: false,
				PerformNewReportsCleanup:  false,
				PrintOldReportsForCleanup: false,
				PerformOldReportsCleanup:  false,
			},
			want: false,
		},
		{
			cliFlags: types.CliFlags{
				PrintNewReportsForCleanup: true,
				PerformNewReportsCleanup:  false,
				PrintOldReportsForCleanup: false,
				PerformOldReportsCleanup:  false,
			},
			want: true,
		},
		{
			cliFlags: types.CliFlags{
				PrintNewReportsForCleanup: false,
				PerformNewReportsCleanup:  true,
				PrintOldReportsForCleanup: false,
				PerformOldReportsCleanup:  false,
			},
			want: true,
		},
		{
			cliFlags: types.CliFlags{
				PrintNewReportsForCleanup: false,
				PerformNewReportsCleanup:  false,
				PrintOldReportsForCleanup: true,
				PerformOldReportsCleanup:  false,
			},
			want: true,
		},
		{
			cliFlags: types.CliFlags{
				PrintNewReportsForCleanup: false,
				PerformNewReportsCleanup:  false,
				PrintOldReportsForCleanup: false,
				PerformOldReportsCleanup:  true,
			},
			want: true,
		},
	}

	for _, tc := range testcases {
		if tc.want {
			assert.True(t, differ.DeleteOperationSpecified(tc.cliFlags), "a DELETE operation was specified in the CLI flags")
		} else {
			assert.False(t, differ.DeleteOperationSpecified(tc.cliFlags), "a DELETE operation wasn't specified in the CLI flags")
		}
	}
}
