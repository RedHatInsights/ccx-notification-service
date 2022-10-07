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

// Documentation in literate-programming-style is available at:
// https://redhatinsights.github.io/ccx-notification-writer/packages/differ/cluster_filter_test.html

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/RedHatInsights/ccx-notification-service/conf"
	"github.com/RedHatInsights/ccx-notification-service/types"
)

// cluster entries to be used by unit tests
var (
	cluster1 = types.ClusterEntry{
		OrgID:         1,
		AccountNumber: 2,
		ClusterName:   "aaaaaaaa-0000-0000-0000-00000000000",
		KafkaOffset:   0,
	}
	cluster2 = types.ClusterEntry{
		OrgID:         1,
		AccountNumber: 2,
		ClusterName:   "bbbbbbbb-0000-0000-0000-00000000000",
		KafkaOffset:   0,
	}
	cluster3 = types.ClusterEntry{
		OrgID:         1,
		AccountNumber: 2,
		ClusterName:   "cccccccc-0000-0000-0000-00000000000",
		KafkaOffset:   0,
	}
	cluster4 = types.ClusterEntry{
		OrgID:         1,
		AccountNumber: 2,
		ClusterName:   "dddddddd-0000-0000-0000-00000000000",
		KafkaOffset:   0,
	}
)

// TestFilterNullClusterList test checks the filtering for null cluster list.
// In this case empty list of clusters is provided at input.
func TestFilterNullClusterList(t *testing.T) {
	// configuration to be used during filtering
	config := conf.ProcessingConfiguration{
		FilterAllowedClusters: false,
		FilterBlockedClusters: false,
	}

	// null value
	var clusters []types.ClusterEntry

	// start filter
	filtered, stat := filterClusterList(clusters, config)

	// check filter output
	assert.Empty(t, filtered)
	assert.Equal(t, 0, stat.Input)
	assert.Equal(t, 0, stat.Allowed)
	assert.Equal(t, 0, stat.Blocked)
	assert.Equal(t, 0, stat.Filtered)
}

// TestFilterEmptyClusterList test checks the filtering for null cluster list.
// In this case empty list of clusters is provided at input.
func TestFilterEmptyClusterList(t *testing.T) {
	// configuration to be used during filtering
	config := conf.ProcessingConfiguration{
		FilterAllowedClusters: false,
		FilterBlockedClusters: false,
	}

	// empty cluster list
	clusters := []types.ClusterEntry{}

	// start filter
	filtered, stat := filterClusterList(clusters, config)

	// check filter output
	assert.Empty(t, filtered)
	assert.Equal(t, 0, stat.Input)
	assert.Equal(t, 0, stat.Allowed)
	assert.Equal(t, 0, stat.Blocked)
	assert.Equal(t, 0, stat.Filtered)
}

// TestFilterNoFilters test checks filtering if filters are disabled.
// In this test case, list with three clusters is provided at input.
func TestFilterNoFilters(t *testing.T) {
	// configuration used during filtering
	config := conf.ProcessingConfiguration{
		FilterAllowedClusters: false,
		FilterBlockedClusters: false,
	}
	var clusters []types.ClusterEntry

	// list of clusters at input
	clusters = append(clusters, cluster1, cluster2, cluster3, cluster4)

	// start filter
	filtered, stat := filterClusterList(clusters, config)

	// check filter output
	assert.Len(t, filtered, 4)
	assert.Equal(t, 4, stat.Input)
	assert.Equal(t, 0, stat.Allowed)
	assert.Equal(t, 0, stat.Blocked)
	assert.Equal(t, 4, stat.Filtered)

	// check the cluster list
	assert.Equal(t, cluster1, filtered[0])
	assert.Equal(t, cluster2, filtered[1])
	assert.Equal(t, cluster3, filtered[2])
	assert.Equal(t, cluster4, filtered[3])
}

// TestFilterBlockOneCluster test checks filtering if filters are
// enabled. In this case block list contains one cluster.
func TestFilterBlockOneCluster(t *testing.T) {
	// configuration used during filtering
	config := conf.ProcessingConfiguration{
		FilterAllowedClusters: false,
		FilterBlockedClusters: true,
		BlockedClusters: []string{
			string(cluster1.ClusterName)},
	}
	var clusters []types.ClusterEntry

	// list of clusters at input
	clusters = append(clusters, cluster1, cluster2, cluster3, cluster4)

	// start filter
	filtered, stat := filterClusterList(clusters, config)

	// check filter output
	assert.Len(t, filtered, 3)
	assert.Equal(t, 4, stat.Input)
	assert.Equal(t, 0, stat.Allowed)
	assert.Equal(t, 1, stat.Blocked)
	assert.Equal(t, 3, stat.Filtered)

	// check the cluster list
	assert.Equal(t, cluster2, filtered[0])
	assert.Equal(t, cluster3, filtered[1])
	assert.Equal(t, cluster4, filtered[2])
}

// TestFilterBlockTwoClusters test checks filtering if filters are
// enabled. In this case block list contains two clusters, both of them
// are known.
func TestFilterBlockTwoClusters(t *testing.T) {
	// configuration used during filtering
	config := conf.ProcessingConfiguration{
		FilterAllowedClusters: false,
		FilterBlockedClusters: true,
		BlockedClusters: []string{
			string(cluster1.ClusterName),
			string(cluster4.ClusterName)},
	}
	var clusters []types.ClusterEntry

	// list of clusters at input
	clusters = append(clusters, cluster1, cluster2, cluster3, cluster4)

	// start filter
	filtered, stat := filterClusterList(clusters, config)

	// check filter output
	assert.Len(t, filtered, 2)
	assert.Equal(t, 4, stat.Input)
	assert.Equal(t, 0, stat.Allowed)
	assert.Equal(t, 2, stat.Blocked)
	assert.Equal(t, 2, stat.Filtered)

	// check the cluster list
	assert.Equal(t, cluster2, filtered[0])
	assert.Equal(t, cluster3, filtered[1])
}

// TestFilterBlockTwoClustersOneUnknown test checks filtering if filters
// are enabled. In this case block list contains two clusters, one of
// them is unknown.
func TestFilterBlockTwoClustersOneUnknown(t *testing.T) {
	// configuration used during filtering
	config := conf.ProcessingConfiguration{
		FilterAllowedClusters: false,
		FilterBlockedClusters: true,
		BlockedClusters: []string{
			string(cluster1.ClusterName),
			"ffffffff-0000-0000-ffffffffffff"},
	}
	var clusters []types.ClusterEntry

	// list of clusters at input
	clusters = append(clusters, cluster1, cluster2, cluster3, cluster4)

	// start filter
	filtered, stat := filterClusterList(clusters, config)

	// check filter output
	assert.Len(t, filtered, 3)
	assert.Equal(t, 4, stat.Input)
	assert.Equal(t, 0, stat.Allowed)
	assert.Equal(t, 1, stat.Blocked)
	assert.Equal(t, 3, stat.Filtered)

	// check the cluster list
	assert.Equal(t, cluster2, filtered[0])
	assert.Equal(t, cluster3, filtered[1])
	assert.Equal(t, cluster4, filtered[2])
}

// TestFilterBlockTwoClustersAllUnknown test checks filtering if filters
// are enabled. In this case block list contains two clusters, both
// unknown.
func TestFilterBlockTwoClustersAllUnknown(t *testing.T) {
	// configuration used during filtering
	config := conf.ProcessingConfiguration{
		FilterAllowedClusters: false,
		FilterBlockedClusters: true,
		BlockedClusters: []string{
			"ffffffff-ffff-ffff-0123456789ab",
			"ffffffff-0000-0000-ffffffffffff"},
	}
	var clusters []types.ClusterEntry

	// list of clusters at input
	clusters = append(clusters, cluster1, cluster2, cluster3, cluster4)

	// start filter
	filtered, stat := filterClusterList(clusters, config)

	// check filter output
	assert.Len(t, filtered, 4)
	assert.Equal(t, 4, stat.Input)
	assert.Equal(t, 0, stat.Allowed)
	assert.Equal(t, 0, stat.Blocked)
	assert.Equal(t, 4, stat.Filtered)

	// check the cluster list
	assert.Equal(t, cluster1, filtered[0])
	assert.Equal(t, cluster2, filtered[1])
	assert.Equal(t, cluster3, filtered[2])
	assert.Equal(t, cluster4, filtered[3])
}

// TestFilterAllowNoClusters test checks filtering if filters are
// enabled. In this case list of allowed clusters is empty.
func TestFilterAllowNoClusters(t *testing.T) {
	// configuration used during filtering
	config := conf.ProcessingConfiguration{
		FilterAllowedClusters: true,
		FilterBlockedClusters: false,
		AllowedClusters:       []string{}}

	var clusters []types.ClusterEntry

	// list of clusters at input
	clusters = append(clusters, cluster1, cluster2, cluster3, cluster4)

	// start filter
	filtered, stat := filterClusterList(clusters, config)

	// check filter output
	assert.Len(t, filtered, 0)
	assert.Equal(t, 4, stat.Input)
	assert.Equal(t, 0, stat.Allowed)
	assert.Equal(t, 0, stat.Blocked)
	assert.Equal(t, 0, stat.Filtered)
}

// TestFilterAllowOneCluster test checks filtering if filters are
// enabled. In this case list of allowed clusters contains one known
// cluster.
func TestFilterAllowOneCluster(t *testing.T) {
	// configuration used during filtering
	config := conf.ProcessingConfiguration{
		FilterAllowedClusters: true,
		FilterBlockedClusters: false,
		AllowedClusters: []string{
			string(cluster1.ClusterName)}}

	var clusters []types.ClusterEntry

	// list of clusters at input
	clusters = append(clusters, cluster1, cluster2, cluster3, cluster4)

	// start filter
	filtered, stat := filterClusterList(clusters, config)

	// check filter output
	assert.Len(t, filtered, 1)
	assert.Equal(t, 4, stat.Input)
	assert.Equal(t, 1, stat.Allowed)
	assert.Equal(t, 0, stat.Blocked)
	assert.Equal(t, 1, stat.Filtered)

	// check the cluster list
	assert.Equal(t, cluster1, filtered[0])
}

// TestFilterAllowTwoClusters test checks filtering if filters are
// enabled. In this case list of allowed clusters contains two known
// clusters.
func TestFilterAllowTwoClusters(t *testing.T) {
	// configuration used during filtering
	config := conf.ProcessingConfiguration{
		FilterAllowedClusters: true,
		FilterBlockedClusters: false,
		AllowedClusters: []string{
			string(cluster1.ClusterName),
			string(cluster2.ClusterName)}}

	var clusters []types.ClusterEntry

	// list of clusters at input
	clusters = append(clusters, cluster1, cluster2, cluster3, cluster4)

	// start filter
	filtered, stat := filterClusterList(clusters, config)

	// check filter output
	assert.Len(t, filtered, 2)
	assert.Equal(t, 4, stat.Input)
	assert.Equal(t, 2, stat.Allowed)
	assert.Equal(t, 0, stat.Blocked)
	assert.Equal(t, 2, stat.Filtered)

	// check the cluster list
	assert.Equal(t, cluster1, filtered[0])
	assert.Equal(t, cluster2, filtered[1])
}

// TestFilterAllowTwoClustersOneUnknown test checks filtering if filters
// are enabled. In this case allow list contains two clusters, one known
// and one unknown.
func TestFilterAllowTwoClustersOneUnknown(t *testing.T) {
	// configuration used during filtering
	config := conf.ProcessingConfiguration{
		FilterAllowedClusters: true,
		FilterBlockedClusters: false,
		AllowedClusters: []string{
			string(cluster1.ClusterName),
			"ffffffff-ffff-ffff-0123456789ab"}}

	var clusters []types.ClusterEntry

	// list of clusters at input
	clusters = append(clusters, cluster1, cluster2, cluster3, cluster4)

	// start filter
	filtered, stat := filterClusterList(clusters, config)

	// check filter output
	assert.Len(t, filtered, 1)
	assert.Equal(t, 4, stat.Input)
	assert.Equal(t, 1, stat.Allowed)
	assert.Equal(t, 0, stat.Blocked)
	assert.Equal(t, 1, stat.Filtered)

	// check the cluster list
	assert.Equal(t, cluster1, filtered[0])
}

// TestFilterAllowTwoClustersAllUnknown test checks filtering if filters
// are enabled. In this case allow list contains two unknown clusters.
func TestFilterAllowTwoClustersAllUnknown(t *testing.T) {
	// configuration used during filtering
	config := conf.ProcessingConfiguration{
		FilterAllowedClusters: true,
		FilterBlockedClusters: false,
		AllowedClusters: []string{
			"ffffffff-ffff-ffff-0123456789ab",
			"ffffffff-0000-0000-ffffffffffff"},
	}
	var clusters []types.ClusterEntry

	// list of clusters at input
	clusters = append(clusters, cluster1, cluster2, cluster3, cluster4)

	// start filter
	filtered, stat := filterClusterList(clusters, config)

	// check filter output
	assert.Len(t, filtered, 0)
	assert.Equal(t, 4, stat.Input)
	assert.Equal(t, 0, stat.Allowed)
	assert.Equal(t, 0, stat.Blocked)
	assert.Equal(t, 0, stat.Filtered)
}

// TestFilterAllowAndBlock test checks filtering if both filters are
// enabled and allow list and block list contain some clusters.
func TestFilterAllowAndBlock(t *testing.T) {
	// configuration used during filtering
	config := conf.ProcessingConfiguration{
		FilterAllowedClusters: true,
		FilterBlockedClusters: true,
		AllowedClusters: []string{
			string(cluster1.ClusterName),
			string(cluster2.ClusterName)},
		BlockedClusters: []string{
			string(cluster3.ClusterName),
			string(cluster4.ClusterName)}}

	var clusters []types.ClusterEntry

	// list of clusters at input
	clusters = append(clusters, cluster1, cluster2, cluster3, cluster4)

	// start filter
	filtered, stat := filterClusterList(clusters, config)

	// check filter output
	assert.Len(t, filtered, 2)
	assert.Equal(t, 4, stat.Input)
	assert.Equal(t, 2, stat.Allowed)
	assert.Equal(t, 0, stat.Blocked)
	assert.Equal(t, 2, stat.Filtered)

	// check the cluster list
	assert.Equal(t, cluster1, filtered[0])
	assert.Equal(t, cluster2, filtered[1])
}
