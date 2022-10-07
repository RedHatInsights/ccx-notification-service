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

// Generated documentation is available at:
// https://pkg.go.dev/github.com/RedHatInsights/ccx-notification-service/differ
//
// Documentation in literate-programming-style is available at:
// https://redhatinsights.github.io/ccx-notification-service/packages/differ/cluster_filter.html

import (
	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/types"

	"github.com/RedHatInsights/insights-operator-utils/collections"
)

// ClusterFilterStatistic is a structure containing elementary statistic about
// clusters being filtered by filterClusterList function. It can be used for
// logging and debugging purposes.
type ClusterFilterStatistic struct {
	Input    int
	Allowed  int
	Blocked  int
	Filtered int
}

// filterClusterList function filters clusters according to given allow list
// and block list
func filterClusterList(clusters []types.ClusterEntry, configuration conf.ProcessingConfiguration) ([]types.ClusterEntry, ClusterFilterStatistic) {
	// initialize structure with statistic
	stat := ClusterFilterStatistic{
		Input:    0,
		Allowed:  0,
		Blocked:  0,
		Filtered: 0,
	}

	// optimization phase - don't process/filter clusters if filtering is
	// completely disabled (this includes both allowed clusters filter and
	// blocked clusters filter)
	if !configuration.FilterAllowedClusters && !configuration.FilterBlockedClusters {
		// just update the statistic
		stat.Input = len(clusters)
		stat.Filtered = len(clusters)

		// and return original cluster list
		return clusters, stat
	}

	// list of filtered clusters
	filtered := []types.ClusterEntry{}

	for _, cluster := range clusters {
		clusterName := string(cluster.ClusterName)

		// update statistic
		stat.Input++

		// cluster might be explicitly allowed if "filter
		// allowed clusters" command line option is enabled
		if configuration.FilterAllowedClusters {
			// if cluster is in list of allowed clusters
			// -> put it into the output list
			// -> skip rest of the loop
			if collections.StringInSlice(clusterName, configuration.AllowedClusters) {
				stat.Allowed++
				stat.Filtered++
				filtered = append(filtered, cluster)
			}
			// don't do anything else with the cluster
			continue
		}

		// cluster might be blocked if "filter blocked clusters"
		// configuration option is enabled
		if configuration.FilterBlockedClusters {
			// if cluster is in list of blocked clusters
			// -> ignore it
			// -> skip rest of the loop
			if collections.StringInSlice(clusterName, configuration.BlockedClusters) {
				stat.Blocked++
				// don't do anything else with the cluster
				continue
			}
			// not blocked
			// -> put it into the output list
			stat.Filtered++
			filtered = append(filtered, cluster)
		}
	}

	// return filtered list of clusters
	return filtered, stat
}
