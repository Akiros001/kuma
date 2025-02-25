package v2_test

import (
	mesh_proto "github.com/kumahq/kuma/api/mesh/v1alpha1"
	core_xds "github.com/kumahq/kuma/pkg/core/xds"
	util_proto "github.com/kumahq/kuma/pkg/util/proto"
	"github.com/kumahq/kuma/pkg/xds/generator"
	modifications "github.com/kumahq/kuma/pkg/xds/generator/modifications/v2"

	envoy_api "github.com/envoyproxy/go-control-plane/envoy/api/v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("HTTP Filter modifications", func() {

	type testCase struct {
		listeners     []string
		modifications []string
		expected      string
	}

	DescribeTable("should apply modifications",
		func(given testCase) {
			// given
			set := core_xds.NewResourceSet()
			for _, listenerYAML := range given.listeners {
				listener := &envoy_api.Listener{}
				err := util_proto.FromYAML([]byte(listenerYAML), listener)
				Expect(err).ToNot(HaveOccurred())
				set.Add(&core_xds.Resource{
					Name:     listener.Name,
					Origin:   generator.OriginInbound,
					Resource: listener,
				})
			}

			var mods []*mesh_proto.ProxyTemplate_Modifications
			for _, modificationYAML := range given.modifications {
				modification := &mesh_proto.ProxyTemplate_Modifications{}
				err := util_proto.FromYAML([]byte(modificationYAML), modification)
				Expect(err).ToNot(HaveOccurred())
				mods = append(mods, modification)
			}

			// when
			err := modifications.Apply(set, mods)

			// then
			Expect(err).ToNot(HaveOccurred())
			resp, err := set.List().ToDeltaDiscoveryResponse()
			Expect(err).ToNot(HaveOccurred())
			actual, err := util_proto.ToYAML(resp)
			Expect(err).ToNot(HaveOccurred())
			Expect(actual).To(MatchYAML(given.expected))
		},
		Entry("should add filter as a first", testCase{
			listeners: []string{
				`
                name: inbound:192.168.0.1:8080
                trafficDirection: INBOUND
                address:
                  socketAddress:
                    address: 192.168.0.1
                    portValue: 8080
                filterChains:
                - filters:
                  - name: envoy.filters.network.http_connection_manager
                    typedConfig:
                      '@type': type.googleapis.com/envoy.config.filter.network.http_connection_manager.v2.HttpConnectionManager
                      statPrefix: localhost_8080
                      httpFilters:
                      - name: envoy.filters.http.router`,
			},
			modifications: []string{`
                httpFilter:
                   operation: addLast
                   value: |
                     name: envoy.filters.http.cors
`,
			},
			expected: `
            resources:
            - name: inbound:192.168.0.1:8080
              resource:
                '@type': type.googleapis.com/envoy.api.v2.Listener
                address:
                  socketAddress:
                    address: 192.168.0.1
                    portValue: 8080
                filterChains:
                - filters:
                  - name: envoy.filters.network.http_connection_manager
                    typedConfig:
                      '@type': type.googleapis.com/envoy.config.filter.network.http_connection_manager.v2.HttpConnectionManager
                      httpFilters:
                      - name: envoy.filters.http.router
                      - name: envoy.filters.http.cors
                      statPrefix: localhost_8080
                name: inbound:192.168.0.1:8080
                trafficDirection: INBOUND`,
		}),
		Entry("should remove all filters from all listeners when there is no match section", testCase{
			listeners: []string{
				`
                address:
                  socketAddress:
                    address: 192.168.0.1
                    portValue: 8080
                filterChains:
                - filters:
                  - name: envoy.filters.network.http_connection_manager
                    typedConfig:
                      '@type': type.googleapis.com/envoy.config.filter.network.http_connection_manager.v2.HttpConnectionManager
                      httpFilters:
                      - name: envoy.filters.http.router
                      - name: envoy.filters.http.cors
                      statPrefix: localhost_8080
                name: inbound:192.168.0.1:8080
                trafficDirection: INBOUND`,
			},
			modifications: []string{`
                httpFilter:
                   operation: remove
`,
			},
			expected: `
            resources:
            - name: inbound:192.168.0.1:8080
              resource:
                '@type': type.googleapis.com/envoy.api.v2.Listener
                address:
                  socketAddress:
                    address: 192.168.0.1
                    portValue: 8080
                filterChains:
                - filters:
                  - name: envoy.filters.network.http_connection_manager
                    typedConfig:
                      '@type': type.googleapis.com/envoy.config.filter.network.http_connection_manager.v2.HttpConnectionManager
                      statPrefix: localhost_8080
                name: inbound:192.168.0.1:8080
                trafficDirection: INBOUND`,
		}),
		Entry("should remove all filters from all listeners when there is inbound match section", testCase{
			listeners: []string{
				`
                address:
                  socketAddress:
                    address: 192.168.0.1
                    portValue: 8080
                filterChains:
                - filters:
                  - name: envoy.filters.network.http_connection_manager
                    typedConfig:
                      '@type': type.googleapis.com/envoy.config.filter.network.http_connection_manager.v2.HttpConnectionManager
                      httpFilters:
                      - name: envoy.filters.http.router
                      - name: envoy.filters.http.cors
                      statPrefix: localhost_8080
                name: inbound:192.168.0.1:8080
                trafficDirection: INBOUND`,
			},
			modifications: []string{`
                httpFilter:
                   operation: remove
                   match:
                     origin: inbound
`,
			},
			expected: `
            resources:
            - name: inbound:192.168.0.1:8080
              resource:
                '@type': type.googleapis.com/envoy.api.v2.Listener
                address:
                  socketAddress:
                    address: 192.168.0.1
                    portValue: 8080
                filterChains:
                - filters:
                  - name: envoy.filters.network.http_connection_manager
                    typedConfig:
                      '@type': type.googleapis.com/envoy.config.filter.network.http_connection_manager.v2.HttpConnectionManager
                      statPrefix: localhost_8080
                name: inbound:192.168.0.1:8080
                trafficDirection: INBOUND`,
		}),
		Entry("should remove all filters from picked listener", testCase{
			listeners: []string{
				`
                address:
                  socketAddress:
                    address: 192.168.0.1
                    portValue: 8080
                filterChains:
                - filters:
                  - name: envoy.filters.network.http_connection_manager
                    typedConfig:
                      '@type': type.googleapis.com/envoy.config.filter.network.http_connection_manager.v2.HttpConnectionManager
                      httpFilters:
                      - name: envoy.filters.http.router
                      - name: envoy.filters.http.cors
                      statPrefix: localhost_8080
                name: inbound:192.168.0.1:8080
                trafficDirection: INBOUND`,
				`
                address:
                  socketAddress:
                    address: 192.168.0.1
                    portValue: 8081
                filterChains:
                - filters:
                  - name: envoy.filters.network.http_connection_manager
                    typedConfig:
                      '@type': type.googleapis.com/envoy.config.filter.network.http_connection_manager.v2.HttpConnectionManager
                      httpFilters:
                      - name: envoy.filters.http.router
                      - name: envoy.filters.http.cors
                      statPrefix: localhost_8081
                name: inbound:192.168.0.1:8081
                trafficDirection: INBOUND`,
			},
			modifications: []string{`
                httpFilter:
                   operation: remove
                   match:
                     listenerName: inbound:192.168.0.1:8080
`,
			},
			expected: `
            resources:
            - name: inbound:192.168.0.1:8080
              resource:
                '@type': type.googleapis.com/envoy.api.v2.Listener
                address:
                  socketAddress:
                    address: 192.168.0.1
                    portValue: 8080
                filterChains:
                - filters:
                  - name: envoy.filters.network.http_connection_manager
                    typedConfig:
                      '@type': type.googleapis.com/envoy.config.filter.network.http_connection_manager.v2.HttpConnectionManager
                      statPrefix: localhost_8080
                name: inbound:192.168.0.1:8080
                trafficDirection: INBOUND
            - name: inbound:192.168.0.1:8081
              resource:
                '@type': type.googleapis.com/envoy.api.v2.Listener
                address:
                  socketAddress:
                    address: 192.168.0.1
                    portValue: 8081
                filterChains:
                - filters:
                  - name: envoy.filters.network.http_connection_manager
                    typedConfig:
                      '@type': type.googleapis.com/envoy.config.filter.network.http_connection_manager.v2.HttpConnectionManager
                      httpFilters:
                      - name: envoy.filters.http.router
                      - name: envoy.filters.http.cors
                      statPrefix: localhost_8081
                name: inbound:192.168.0.1:8081
                trafficDirection: INBOUND`,
		}),
		Entry("should remove all filters of given name from all listeners", testCase{
			listeners: []string{
				`
                address:
                  socketAddress:
                    address: 192.168.0.1
                    portValue: 8080
                filterChains:
                - filters:
                  - name: envoy.filters.network.http_connection_manager
                    typedConfig:
                      '@type': type.googleapis.com/envoy.config.filter.network.http_connection_manager.v2.HttpConnectionManager
                      httpFilters:
                      - name: envoy.filters.http.router
                      - name: envoy.filters.http.cors
                      statPrefix: localhost_8080
                name: inbound:192.168.0.1:8080
                trafficDirection: INBOUND`,
				`
                address:
                  socketAddress:
                    address: 192.168.0.1
                    portValue: 8081
                filterChains:
                - filters:
                  - name: envoy.filters.network.http_connection_manager
                    typedConfig:
                      '@type': type.googleapis.com/envoy.config.filter.network.http_connection_manager.v2.HttpConnectionManager
                      httpFilters:
                      - name: envoy.filters.http.router
                      - name: envoy.filters.http.cors
                      statPrefix: localhost_8081
                name: inbound:192.168.0.1:8081
                trafficDirection: INBOUND`,
			},
			modifications: []string{`
                httpFilter:
                   operation: remove
                   match:
                     name: envoy.filters.http.cors
`,
			},
			expected: `
            resources:
            - name: inbound:192.168.0.1:8080
              resource:
                '@type': type.googleapis.com/envoy.api.v2.Listener
                address:
                  socketAddress:
                    address: 192.168.0.1
                    portValue: 8080
                filterChains:
                - filters:
                  - name: envoy.filters.network.http_connection_manager
                    typedConfig:
                      '@type': type.googleapis.com/envoy.config.filter.network.http_connection_manager.v2.HttpConnectionManager
                      httpFilters:
                      - name: envoy.filters.http.router
                      statPrefix: localhost_8080
                name: inbound:192.168.0.1:8080
                trafficDirection: INBOUND
            - name: inbound:192.168.0.1:8081
              resource:
                '@type': type.googleapis.com/envoy.api.v2.Listener
                address:
                  socketAddress:
                    address: 192.168.0.1
                    portValue: 8081
                filterChains:
                - filters:
                  - name: envoy.filters.network.http_connection_manager
                    typedConfig:
                      '@type': type.googleapis.com/envoy.config.filter.network.http_connection_manager.v2.HttpConnectionManager
                      httpFilters:
                      - name: envoy.filters.http.router
                      statPrefix: localhost_8081
                name: inbound:192.168.0.1:8081
                trafficDirection: INBOUND`,
		}),
		Entry("should add filter after already defined", testCase{
			listeners: []string{
				`
                address:
                  socketAddress:
                    address: 192.168.0.1
                    portValue: 8080
                filterChains:
                - filters:
                  - name: envoy.filters.network.http_connection_manager
                    typedConfig:
                      '@type': type.googleapis.com/envoy.config.filter.network.http_connection_manager.v2.HttpConnectionManager
                      httpFilters:
                      - name: envoy.filters.http.router
                      - name: envoy.filters.http.gzip
                      statPrefix: localhost_8080
                name: inbound:192.168.0.1:8080
                trafficDirection: INBOUND`,
			},
			modifications: []string{`
                httpFilter:
                   operation: addAfter
                   match:
                     name: envoy.filters.http.router
                   value: |
                     name: envoy.filters.http.cors
`,
			},
			expected: `
            resources:
            - name: inbound:192.168.0.1:8080
              resource:
                '@type': type.googleapis.com/envoy.api.v2.Listener
                address:
                  socketAddress:
                    address: 192.168.0.1
                    portValue: 8080
                filterChains:
                - filters:
                  - name: envoy.filters.network.http_connection_manager
                    typedConfig:
                      '@type': type.googleapis.com/envoy.config.filter.network.http_connection_manager.v2.HttpConnectionManager
                      httpFilters:
                      - name: envoy.filters.http.router
                      - name: envoy.filters.http.cors
                      - name: envoy.filters.http.gzip
                      statPrefix: localhost_8080
                name: inbound:192.168.0.1:8080
                trafficDirection: INBOUND`,
		}),
		Entry("should not add filter when name is not matched", testCase{
			listeners: []string{
				`
                address:
                  socketAddress:
                    address: 192.168.0.1
                    portValue: 8080
                filterChains:
                - filters:
                  - name: envoy.filters.network.http_connection_manager
                    typedConfig:
                      '@type': type.googleapis.com/envoy.config.filter.network.http_connection_manager.v2.HttpConnectionManager
                      httpFilters:
                      - name: envoy.filters.http.router
                      statPrefix: localhost_8080
                name: inbound:192.168.0.1:8080
                trafficDirection: INBOUND`,
			},
			modifications: []string{`
                httpFilter:
                   operation: addAfter
                   match:
                     name: envoy.filters.http.gzip
                   value: |
                     name: envoy.filters.http.cors
`,
			},
			expected: `
            resources:
            - name: inbound:192.168.0.1:8080
              resource:
                '@type': type.googleapis.com/envoy.api.v2.Listener
                address:
                  socketAddress:
                    address: 192.168.0.1
                    portValue: 8080
                filterChains:
                - filters:
                  - name: envoy.filters.network.http_connection_manager
                    typedConfig:
                      '@type': type.googleapis.com/envoy.config.filter.network.http_connection_manager.v2.HttpConnectionManager
                      httpFilters:
                      - name: envoy.filters.http.router
                      statPrefix: localhost_8080
                name: inbound:192.168.0.1:8080
                trafficDirection: INBOUND`,
		}),
		Entry("should add filter before already defined", testCase{
			listeners: []string{
				`
                address:
                  socketAddress:
                    address: 192.168.0.1
                    portValue: 8080
                filterChains:
                - filters:
                  - name: envoy.filters.network.http_connection_manager
                    typedConfig:
                      '@type': type.googleapis.com/envoy.config.filter.network.http_connection_manager.v2.HttpConnectionManager
                      httpFilters:
                      - name: envoy.filters.http.router
                      - name: envoy.filters.http.gzip
                      statPrefix: localhost_8080
                name: inbound:192.168.0.1:8080
                trafficDirection: INBOUND`,
			},
			modifications: []string{`
                httpFilter:
                   operation: addBefore
                   match:
                     name: envoy.filters.http.gzip
                   value: |
                     name: envoy.filters.http.cors
`,
			},
			expected: `
            resources:
            - name: inbound:192.168.0.1:8080
              resource:
                '@type': type.googleapis.com/envoy.api.v2.Listener
                address:
                  socketAddress:
                    address: 192.168.0.1
                    portValue: 8080
                filterChains:
                - filters:
                  - name: envoy.filters.network.http_connection_manager
                    typedConfig:
                      '@type': type.googleapis.com/envoy.config.filter.network.http_connection_manager.v2.HttpConnectionManager
                      httpFilters:
                      - name: envoy.filters.http.router
                      - name: envoy.filters.http.cors
                      - name: envoy.filters.http.gzip
                      statPrefix: localhost_8080
                name: inbound:192.168.0.1:8080
                trafficDirection: INBOUND`,
		}),
		Entry("should patch resource matching filter name", testCase{
			listeners: []string{
				`
                address:
                  socketAddress:
                    address: 192.168.0.1
                    portValue: 8080
                filterChains:
                - filters:
                  - name: envoy.filters.network.http_connection_manager
                    typedConfig:
                      '@type': type.googleapis.com/envoy.config.filter.network.http_connection_manager.v2.HttpConnectionManager
                      httpFilters:
                      - name: envoy.filters.http.router
                        typedConfig:
                          '@type': type.googleapis.com/envoy.config.filter.http.router.v2.Router
                          startChildSpan: true
                      - name: envoy.filters.http.gzip
                      statPrefix: localhost_8080
                name: inbound:192.168.0.1:8080
                trafficDirection: INBOUND`,
			},
			modifications: []string{`
                httpFilter:
                   operation: patch
                   match:
                     name: envoy.filters.http.router
                   value: |
                     typedConfig:
                       '@type': type.googleapis.com/envoy.config.filter.http.router.v2.Router
                       dynamicStats: false
`,
			},
			expected: `
            resources:
            - name: inbound:192.168.0.1:8080
              resource:
                '@type': type.googleapis.com/envoy.api.v2.Listener
                address:
                  socketAddress:
                    address: 192.168.0.1
                    portValue: 8080
                filterChains:
                - filters:
                  - name: envoy.filters.network.http_connection_manager
                    typedConfig:
                      '@type': type.googleapis.com/envoy.config.filter.network.http_connection_manager.v2.HttpConnectionManager
                      httpFilters:
                      - name: envoy.filters.http.router
                        typedConfig:
                          '@type': type.googleapis.com/envoy.config.filter.http.router.v2.Router
                          startChildSpan: true
                          dynamicStats: false
                      - name: envoy.filters.http.gzip
                      statPrefix: localhost_8080
                name: inbound:192.168.0.1:8080
                trafficDirection: INBOUND`,
		}),
	)
})
