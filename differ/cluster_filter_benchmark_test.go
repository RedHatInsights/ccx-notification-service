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

	configurationTenBlockedClustersConfig = conf.ProcessingConfiguration{
		FilterAllowedClusters: false,
		FilterBlockedClusters: true,
		AllowedClusters: []string{
			string(cluster1.ClusterName),
			string(cluster2.ClusterName),
			string(cluster3.ClusterName),
			string(cluster4.ClusterName),
			string(cluster5.ClusterName),
			"11111111-1111-1111-111111111111",
			"22222222-2222-2222-222222222222",
			"33333333-3333-3333-333333333333",
			"44444444-4444-4444-444444444444",
			"55555555-5555-5555-555555555555",
		},
		BlockedClusters: []string{
			string(cluster1.ClusterName),
			string(cluster2.ClusterName),
			string(cluster3.ClusterName),
			string(cluster4.ClusterName),
			string(cluster5.ClusterName),
			"11111111-1111-1111-111111111111",
			"22222222-2222-2222-222222222222",
			"33333333-3333-3333-333333333333",
			"44444444-4444-4444-444444444444",
			"55555555-5555-5555-555555555555",
		},
	}

	configurationTenAllowClustersConfig = conf.ProcessingConfiguration{
		FilterAllowedClusters: true,
		FilterBlockedClusters: false,
		AllowedClusters: []string{
			string(cluster1.ClusterName),
			string(cluster2.ClusterName),
			string(cluster3.ClusterName),
			string(cluster4.ClusterName),
			string(cluster5.ClusterName),
			"11111111-1111-1111-111111111111",
			"22222222-2222-2222-222222222222",
			"33333333-3333-3333-333333333333",
			"44444444-4444-4444-444444444444",
			"55555555-5555-5555-555555555555",
		},
		BlockedClusters: []string{
			string(cluster1.ClusterName),
			string(cluster2.ClusterName),
			string(cluster3.ClusterName),
			string(cluster4.ClusterName),
			string(cluster5.ClusterName),
			"11111111-1111-1111-111111111111",
			"22222222-2222-2222-222222222222",
			"33333333-3333-3333-333333333333",
			"44444444-4444-4444-444444444444",
			"55555555-5555-5555-555555555555",
		},
	}

	configurationTenUnknownBlockedClustersConfig = conf.ProcessingConfiguration{
		FilterAllowedClusters: false,
		FilterBlockedClusters: true,
		AllowedClusters: []string{
			"11111111-1111-1111-111111111111",
			"22222222-2222-2222-222222222222",
			"33333333-3333-3333-333333333333",
			"44444444-4444-4444-444444444444",
			"55555555-5555-5555-555555555555",
			"66666666-6666-6666-666666666666",
			"77777777-7777-7777-777777777777",
			"88888888-8888-8888-888888888888",
			"99999999-9999-9999-999999999999",
			"aaaaaaaa-aaaa-aaaa-aaaaaaaaaaaa",
		},
		BlockedClusters: []string{
			"11111111-1111-1111-111111111111",
			"22222222-2222-2222-222222222222",
			"33333333-3333-3333-333333333333",
			"44444444-4444-4444-444444444444",
			"55555555-5555-5555-555555555555",
			"66666666-6666-6666-666666666666",
			"77777777-7777-7777-777777777777",
			"88888888-8888-8888-888888888888",
			"99999999-9999-9999-999999999999",
			"aaaaaaaa-aaaa-aaaa-aaaaaaaaaaaa",
		},
	}

	configurationTenUnknownAllowedClustersConfig = conf.ProcessingConfiguration{
		FilterAllowedClusters: true,
		FilterBlockedClusters: false,
		AllowedClusters: []string{
			"11111111-1111-1111-111111111111",
			"22222222-2222-2222-222222222222",
			"33333333-3333-3333-333333333333",
			"44444444-4444-4444-444444444444",
			"55555555-5555-5555-555555555555",
			"66666666-6666-6666-666666666666",
			"77777777-7777-7777-777777777777",
			"88888888-8888-8888-888888888888",
			"99999999-9999-9999-999999999999",
			"aaaaaaaa-aaaa-aaaa-aaaaaaaaaaaa",
		},
		BlockedClusters: []string{
			"11111111-1111-1111-111111111111",
			"22222222-2222-2222-222222222222",
			"33333333-3333-3333-333333333333",
			"44444444-4444-4444-444444444444",
			"55555555-5555-5555-555555555555",
			"66666666-6666-6666-666666666666",
			"77777777-7777-7777-777777777777",
			"88888888-8888-8888-888888888888",
			"99999999-9999-9999-999999999999",
			"aaaaaaaa-aaaa-aaaa-aaaaaaaaaaaa",
		},
	}
)

// runBenchmark function run the benchmark with specified cluster list and filter configuration
func runBenchmark(b *testing.B, clusters []types.ClusterEntry, config conf.ProcessingConfiguration) {
	// be sure to check just the benchmark time, not setup time
	b.ResetTimer()

	// run benchmark
	for i := 0; i < b.N; i++ {
		_, _ = filterClusterList(clusters, config)
	}
}

// Benchmark for null cluster list at input when both filters are disabled.
func BenchmarkNoFiltersNullListOfClustersFiltersDisabled(b *testing.B) {
	// configuration used during filtering
	config := configurationFiltersDisabled

	// null value
	var clusters []types.ClusterEntry

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Benchmark for null cluster list at input when allow filter is enabled.
func BenchmarkNoFiltersNullListOfClustersAllowFilterEnabled(b *testing.B) {
	// configuration used during filtering
	config := configurationAllowFilterEnabled

	// null value
	var clusters []types.ClusterEntry

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Benchmark for null cluster list at input when block filter is enabled.
func BenchmarkNoFiltersNullListOfClustersBlockFilterEnabled(b *testing.B) {
	// configuration used during filtering
	config := configurationBlockFilterEnabled

	// null value
	var clusters []types.ClusterEntry

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Benchmark for null cluster list at input when both filters are enabled.
func BenchmarkNoFiltersNullListOfClustersFiltersEnabled(b *testing.B) {
	// configuration used during filtering
	config := configurationFiltersEnabled

	// null value
	var clusters []types.ClusterEntry

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Benchmark for empty cluster list at input when both filters are disabled.
func BenchmarkNoFiltersEmptyListOfClustersFiltersDisabled(b *testing.B) {
	// configuration used during filtering
	config := configurationFiltersDisabled

	// empty value
	clusters := []types.ClusterEntry{}

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Benchmark for empty cluster list at input when allow filter is enabled.
func BenchmarkNoFiltersEmptyListOfClustersAllowFilterEnabled(b *testing.B) {
	// configuration used during filtering
	config := configurationAllowFilterEnabled

	// empty value
	clusters := []types.ClusterEntry{}

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Benchmark for empty cluster list at input when block filter is enabled.
func BenchmarkNoFiltersEmptyListOfClustersBlockFilterEnabled(b *testing.B) {
	// configuration used during filtering
	config := configurationBlockFilterEnabled

	// empty value
	clusters := []types.ClusterEntry{}

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Benchmark for empty cluster list at input when both filters are enabled.
func BenchmarkNoFiltersEmptyListOfClustersFiltersEnabled(b *testing.B) {
	// configuration used during filtering
	config := configurationFiltersEnabled

	// empty value
	clusters := []types.ClusterEntry{}

	// start benchmark
	runBenchmark(b, clusters, config)
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

// Check cluster list processing for 10 clusters when both filters are disabled.
func BenchmarkNoFilters10Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationFiltersDisabled

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(10)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check cluster list processing for 100 clusters when both filters are disabled.
func BenchmarkNoFilters100Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationFiltersDisabled

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(100)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check cluster list processing for 1000 clusters when both filters are disabled.
func BenchmarkNoFilters1000Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationFiltersDisabled

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(1000)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check cluster list processing for 10000 clusters when both filters are disabled.
func BenchmarkNoFilters10000Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationFiltersDisabled

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(10000)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check cluster list processing for 10 clusters when block filter is enabled.
func BenchmarkEmptyBlockListFilter10Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationBlockFilterEnabled

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(10)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check cluster list processing for 100 clusters when block filter is enabled.
func BenchmarkEmptyBlockListFilter100Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationBlockFilterEnabled

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(100)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check cluster list processing for 1000 clusters when block filter is enabled.
func BenchmarkEmptyBlockListFilter1000Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationBlockFilterEnabled

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(1000)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check cluster list processing for 10000 clusters when block filter is enabled.
func BenchmarkEmptyBlockListFilter10000Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationBlockFilterEnabled

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(10000)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check cluster list processing for 10 clusters when allow filter is enabled.
func BenchmarkEmptyAllowListFilter10Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationAllowFilterEnabled

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(10)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check cluster list processing for 100 clusters when allow filter is enabled.
func BenchmarkEmptyAllowListFilter100Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationAllowFilterEnabled

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(100)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check cluster list processing for 1000 clusters when allow filter is enabled.
func BenchmarkEmptyAllowListFilter1000Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationAllowFilterEnabled

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(1000)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check cluster list processing for 10000 clusters when allow filter is enabled.
func BenchmarkEmptyAllowListFilter10000Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationAllowFilterEnabled

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(10000)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check cluster list processing for block filter with 10 clusters
func Benchmark1ClusterInBlockListFilter10Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationOneClusterBlockList

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(10)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check cluster list processing for block filter with 100 clusters
func Benchmark1ClusterInBlockListFilter100Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationOneClusterBlockList

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(100)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check cluster list processing for block filter with 1000 clusters
func Benchmark1ClusterInBlockListFilter1000Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationOneClusterBlockList

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(1000)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check cluster list processing for block filter with 10000 clusters
func Benchmark1ClusterInBlockListFilter10000Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationOneClusterBlockList

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(10000)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check cluster list processing for allow list with 10 clusters
func Benchmark1ClusterInAllowListFilter10Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationOneClusterAllowList

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(10)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check cluster list processing for allow list with 100 clusters
func Benchmark1ClusterInAllowListFilter100Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationOneClusterAllowList

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(100)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check cluster list processing for allow list with 1000 clusters
func Benchmark1ClusterInAllowListFilter1000Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationOneClusterAllowList

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(1000)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check cluster list processing for allow list with 10000 clusters
func Benchmark1ClusterInAllowListFilter10000Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationOneClusterAllowList

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(10000)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check cluster list processing for filter with 10 known clusters and cluster
// list with 10 clusters
func Benchmark10ClustersInBlockListFilter10Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationTenBlockedClustersConfig

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(10)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check cluster list processing for filter with 10 known clusters and cluster
// list with 100 clusters
func Benchmark10ClustersInBlockListFilter100Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationTenBlockedClustersConfig

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(100)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check cluster list processing for filter with 10 known clusters and cluster
// list with 1000 clusters
func Benchmark10ClustersInBlockListFilter1000Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationTenBlockedClustersConfig

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(1000)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check cluster list processing for filter with 10 known clusters and cluster
// list with 10000 clusters
func Benchmark10ClustersInBlockListFilter10000Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationTenBlockedClustersConfig

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(10000)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check cluster list processing for filter with 10 known clusters and cluster
// list with 10 clusters
func Benchmark10ClustersInAllowListFilter10Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationTenAllowClustersConfig

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(10)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check cluster list processing for filter with 10 known clusters and cluster
// list with 100 clusters
func Benchmark10ClustersInAllowListFilter100Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationTenAllowClustersConfig

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(100)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check cluster list processing for filter with 10 known clusters and cluster
// list with 1000
func Benchmark10ClustersInAllowListFilter1000Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationTenAllowClustersConfig

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(1000)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check cluster list processing for filter with 10 known clusters and cluster
// list with 10000
func Benchmark10ClustersInAllowListFilter10000Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationTenAllowClustersConfig

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(10000)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check filtering by unknown clusters
func Benchmark10UnknownClustersInBlockListFilter10Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationTenUnknownBlockedClustersConfig

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(10)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check filtering by unknown clusters
func Benchmark10UnknownClustersInBlockListFilter100Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationTenUnknownBlockedClustersConfig

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(100)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check filtering by unknown clusters
func Benchmark10UnknownClustersInBlockListFilter1000Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationTenUnknownBlockedClustersConfig

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(1000)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check filtering by unknown clusters
func Benchmark10UnknownClustersInBlockListFilter10000Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationTenUnknownBlockedClustersConfig

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(10000)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check filtering by unknown clusters
func Benchmark10UnknownClustersInAllowListFilter10Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationTenUnknownAllowedClustersConfig

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(10)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check filtering by unknown clusters
func Benchmark10UnknownClustersInAllowListFilter100Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationTenUnknownAllowedClustersConfig

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(100)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check filtering by unknown clusters
func Benchmark10UnknownClustersInAllowListFilter1000Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationTenUnknownAllowedClustersConfig

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(1000)

	// start benchmark
	runBenchmark(b, clusters, config)
}

// Check filtering by unknown clusters
func Benchmark10UnknownClustersInAllowListFilter10000Clusters(b *testing.B) {
	// configuration used during filtering
	config := configurationTenUnknownAllowedClustersConfig

	// fill-in list of clusters at input
	clusters := prepareListOfNClusters(10000)

	// start benchmark
	runBenchmark(b, clusters, config)
}
