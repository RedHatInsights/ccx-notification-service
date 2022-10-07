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

// Benchmarks for function filterClusterList.

// Documentation in literate-programming-style is available at:
// https://redhatinsights.github.io/ccx-notification-writer/packages/differ/cluster_filter_test.html

import (
	"testing"

	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/types"
)

// configuration variants used during filtering
var (
	configurationFiltersDisabled = conf.ProcessingConfiguration{
		FilterAllowedClusters: false,
		FilterBlockedClusters: false,
	}

	configurationAllowFilterEnabled = conf.ProcessingConfiguration{
		FilterAllowedClusters: true,
		FilterBlockedClusters: false,
	}

	configurationBlockFilterEnabled = conf.ProcessingConfiguration{
		FilterAllowedClusters: false,
		FilterBlockedClusters: true,
	}

	configurationFiltersEnabled = conf.ProcessingConfiguration{
		FilterAllowedClusters: true,
		FilterBlockedClusters: true,
	}

	configurationOneClusterBlockList = conf.ProcessingConfiguration{
		FilterAllowedClusters: false,
		FilterBlockedClusters: true,
		BlockedClusters: []string{
			string(cluster2.ClusterName)},
	}

	configurationOneClusterAllowList = conf.ProcessingConfiguration{
		FilterAllowedClusters: true,
		FilterBlockedClusters: false,
		AllowedClusters: []string{
			string(cluster2.ClusterName)},
	}
)
