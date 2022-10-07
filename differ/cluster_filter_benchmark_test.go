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
)

// Benchmark for null cluster list at input when both filters are disabled.
func BenchmarkNoFiltersNullListOfClustersFiltersDisabled(b *testing.B) {
	// configuration used during filtering
	config := configurationFiltersDisabled

	// null value
	var clusters []types.ClusterEntry

	// run benchmark
	for i := 0; i < b.N; i++ {
		_, _ = filterClusterList(clusters, config)
	}
}

// Benchmark for null cluster list at input when allow filter is enabled.
func BenchmarkNoFiltersNullListOfClustersAllowFilterEnabled(b *testing.B) {
	// configuration used during filtering
	config := configurationAllowFilterEnabled

	// null value
	var clusters []types.ClusterEntry

	// run benchmark
	for i := 0; i < b.N; i++ {
		_, _ = filterClusterList(clusters, config)
	}
}

// Benchmark for null cluster list at input when block filter is enabled.
func BenchmarkNoFiltersNullListOfClustersBlockFilterEnabled(b *testing.B) {
	// configuration used during filtering
	config := configurationBlockFilterEnabled

	// null value
	var clusters []types.ClusterEntry

	// run benchmark
	for i := 0; i < b.N; i++ {
		_, _ = filterClusterList(clusters, config)
	}
}

// Benchmark for null cluster list at input when both filters are enabled.
func BenchmarkNoFiltersNullListOfClustersFiltersEnabled(b *testing.B) {
	// configuration used during filtering
	config := configurationFiltersEnabled

	// null value
	var clusters []types.ClusterEntry

	// run benchmark
	for i := 0; i < b.N; i++ {
		_, _ = filterClusterList(clusters, config)
	}
}

// Benchmark for empty cluster list at input when both filters are disabled.
func BenchmarkNoFiltersEmptyListOfClustersFiltersDisabled(b *testing.B) {
	// configuration used during filtering
	config := configurationFiltersDisabled

	// empty value
	clusters := []types.ClusterEntry{}

	// run benchmark
	for i := 0; i < b.N; i++ {
		_, _ = filterClusterList(clusters, config)
	}
}

// Benchmark for empty cluster list at input when allow filter is enabled.
func BenchmarkNoFiltersEmptyListOfClustersAllowFilterEnabled(b *testing.B) {
	// configuration used during filtering
	config := configurationAllowFilterEnabled

	// empty value
	clusters := []types.ClusterEntry{}

	// run benchmark
	for i := 0; i < b.N; i++ {
		_, _ = filterClusterList(clusters, config)
	}
}

// Benchmark for empty cluster list at input when block filter is enabled.
func BenchmarkNoFiltersEmptyListOfClustersBlockFilterEnabled(b *testing.B) {
	// configuration used during filtering
	config := configurationBlockFilterEnabled

	// empty value
	clusters := []types.ClusterEntry{}

	// run benchmark
	for i := 0; i < b.N; i++ {
		_, _ = filterClusterList(clusters, config)
	}
}

// Benchmark for empty cluster list at input when both filters are enabled.
func BenchmarkNoFiltersEmptyListOfClustersFiltersEnabled(b *testing.B) {
	// configuration used during filtering
	config := configurationFiltersEnabled

	// empty value
	clusters := []types.ClusterEntry{}

	// run benchmark
	for i := 0; i < b.N; i++ {
		_, _ = filterClusterList(clusters, config)
	}
}

// Helper function to prepare list of at least N clusters
func prepareListOfNClusters(n int) []types.ClusterEntry {
	var clusters []types.ClusterEntry

	// add 5 clusters at once
	for i := 0; i < n; i += 5 {
		clusters = append(clusters, cluster1, cluster2, cluster3, cluster4, cluster5)
	}

	return clusters
}

// Check list processing for 10 clusters when both filters are disabled.
func BenchmarkNoFilters10Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationFiltersDisabled

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(10)

	// run benchmark
	for i := 0; i < b.N; i++ {
		_, _ = filterClusterList(clusters, config)
	}
}

// Check list processing for 100 clusters when both filters are disabled.
func BenchmarkNoFilters100Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationFiltersDisabled

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(100)

	// run benchmark
	for i := 0; i < b.N; i++ {
		_, _ = filterClusterList(clusters, config)
	}
}

// Check list processing for 1000 clusters when both filters are disabled.
func BenchmarkNoFilters1000Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationFiltersDisabled

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(1000)

	// run benchmark
	for i := 0; i < b.N; i++ {
		_, _ = filterClusterList(clusters, config)
	}
}

// Check list processing for 10000 clusters when both filters are disabled.
func BenchmarkNoFilters10000Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationFiltersDisabled

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(10000)

	// run benchmark
	for i := 0; i < b.N; i++ {
		_, _ = filterClusterList(clusters, config)
	}
}

// Check list processing for 10 clusters when block filter is enabled.
func BenchmarkEmptyBlockListFilter10Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationBlockFilterEnabled

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(10)

	// run benchmark
	for i := 0; i < b.N; i++ {
		_, _ = filterClusterList(clusters, config)
	}
}

// Check list processing for 100 clusters when block filter is enabled.
func BenchmarkEmptyBlockListFilter100Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationBlockFilterEnabled

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(100)

	// run benchmark
	for i := 0; i < b.N; i++ {
		_, _ = filterClusterList(clusters, config)
	}
}

// Check list processing for 1000 clusters when block filter is enabled.
func BenchmarkEmptyBlockListFilter1000Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationBlockFilterEnabled

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(1000)

	// run benchmark
	for i := 0; i < b.N; i++ {
		_, _ = filterClusterList(clusters, config)
	}
}

// Check list processing for 10000 clusters when block filter is enabled.
func BenchmarkEmptyBlockListFilter10000Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationBlockFilterEnabled

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(10000)

	// run benchmark
	for i := 0; i < b.N; i++ {
		_, _ = filterClusterList(clusters, config)
	}
}

// Check list processing for 10 clusters when allow filter is enabled.
func BenchmarkEmptyAllowListFilter10Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationAllowFilterEnabled

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(10)

	// run benchmark
	for i := 0; i < b.N; i++ {
		_, _ = filterClusterList(clusters, config)
	}
}

// Check list processing for 100 clusters when allow filter is enabled.
func BenchmarkEmptyAllowListFilter100Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationAllowFilterEnabled

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(100)

	// run benchmark
	for i := 0; i < b.N; i++ {
		_, _ = filterClusterList(clusters, config)
	}
}

// Check list processing for 1000 clusters when allow filter is enabled.
func BenchmarkEmptyAllowListFilter1000Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationAllowFilterEnabled

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(1000)

	// run benchmark
	for i := 0; i < b.N; i++ {
		_, _ = filterClusterList(clusters, config)
	}
}

// Check list processing for 10000 clusters when allow filter is enabled.
func BenchmarkEmptyAllowListFilter10000Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationAllowFilterEnabled

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(10000)

	// run benchmark
	for i := 0; i < b.N; i++ {
		_, _ = filterClusterList(clusters, config)
	}
}
