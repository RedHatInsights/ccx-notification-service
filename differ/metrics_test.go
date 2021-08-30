/*
Copyright © 2021 Red Hat, Inc.

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
// https://redhatinsights.github.io/ccx-notification-writer/packages/differ/metrics_test.html

import (
	"testing"

	"github.com/RedHatInsights/ccx-notification-service/differ"
	"github.com/stretchr/testify/assert"
)

// TestAddMetricsWithNamespace function checks the basic behaviour of function
// AddMetricsWithNamespace from `metrics.go`
func TestAddMetricsWithNamespace(t *testing.T) {
	// add all metrics into the namespace "foobar"
	differ.AddMetricsWithNamespace("foobar")

	// check the registration
	assert.NotNil(t, differ.FetchContentErrors)
	assert.NotNil(t, differ.ReadClusterListErrors)
	assert.NotNil(t, differ.ProducerSetupErrors)
	assert.NotNil(t, differ.StorageSetupErrors)
	assert.NotNil(t, differ.ReadReportForClusterErrors)
	assert.NotNil(t, differ.DeserializeReportErrors)
	assert.NotNil(t, differ.ReportWithHighImpact)
	assert.NotNil(t, differ.NotificationNotSentSameState)
	assert.NotNil(t, differ.NotificationNotSentErrorState)
	assert.NotNil(t, differ.NotificationSent)
}
