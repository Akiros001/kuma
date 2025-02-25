package v2_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	. "github.com/kumahq/kuma/pkg/xds/envoy/routes"

	util_proto "github.com/kumahq/kuma/pkg/util/proto"
	envoy_common "github.com/kumahq/kuma/pkg/xds/envoy"
)

var _ = Describe("DefaultRouteConfigurer", func() {

	type testCase struct {
		clusters []envoy_common.ClusterSubset
		expected string
	}

	DescribeTable("should generate proper Envoy config",
		func(given testCase) {
			// when
			routeConfiguration, err := NewVirtualHostBuilder(envoy_common.APIV2).
				Configure(DefaultRoute(given.clusters...)).
				Build()
			// then
			Expect(err).ToNot(HaveOccurred())

			// when
			actual, err := util_proto.ToYAML(routeConfiguration)
			// then
			Expect(err).ToNot(HaveOccurred())
			// and
			Expect(actual).To(MatchYAML(given.expected))
		},
		Entry("basic VirtualHost with a single destination cluster", testCase{
			clusters: []envoy_common.ClusterSubset{
				{ClusterName: "backend", Weight: 200},
			},
			expected: `
            routes:
            - match:
                prefix: /
              route:
                cluster: backend
                timeout: 0s
`,
		}),
		Entry("basic VirtualHost with weighted destination clusters", testCase{
			clusters: []envoy_common.ClusterSubset{
				{ClusterName: "backend", Weight: 30, Tags: map[string]string{"version": "v1"}},
				{ClusterName: "backend", Weight: 70, Tags: map[string]string{"version": "v2"}},
			},
			expected: `
            routes:
            - match:
                prefix: /
              route:
                weightedClusters:
                  clusters:
                  - metadataMatch:
                      filterMetadata:
                        envoy.lb:
                          version: v1
                    name: backend
                    weight: 30
                  - metadataMatch:
                      filterMetadata:
                        envoy.lb:
                          version: v2
                    name: backend
                    weight: 70
                  totalWeight: 100
                timeout: 0s
`,
		}),
		Entry("basic VirtualHost with weighted destination clusters with totalWeight less than 100", testCase{
			clusters: []envoy_common.ClusterSubset{
				{ClusterName: "backend", Weight: 30, Tags: map[string]string{"version": "v1"}},
				{ClusterName: "backend", Weight: 60, Tags: map[string]string{"version": "v2"}},
			},
			expected: `
            routes:
            - match:
                prefix: /
              route:
                weightedClusters:
                  clusters:
                  - metadataMatch:
                      filterMetadata:
                        envoy.lb:
                          version: v1
                    name: backend
                    weight: 30
                  - metadataMatch:
                      filterMetadata:
                        envoy.lb:
                          version: v2
                    name: backend
                    weight: 60
                  totalWeight: 90
                timeout: 0s
`,
		}),
		Entry("subset with external service", testCase{
			clusters: []envoy_common.ClusterSubset{
				{ClusterName: "backend", Weight: 30, Tags: map[string]string{"version": "v1"}},
				{ClusterName: "backend", Weight: 60, Tags: map[string]string{"version": "v2"}, IsExternalService: true},
			},
			expected: `
            routes:
            - match:
                prefix: /
              route:
                autoHostRewrite: true
                timeout: 0s
                weightedClusters:
                  clusters:
                  - metadataMatch:
                      filterMetadata:
                        envoy.lb:
                          version: v1
                    name: backend
                    weight: 30
                  - metadataMatch:
                      filterMetadata:
                        envoy.lb:
                          version: v2
                    name: backend
                    weight: 60
                  totalWeight: 90
`,
		}),
	)
})
