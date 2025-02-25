package v2_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	core_xds "github.com/kumahq/kuma/pkg/core/xds"

	. "github.com/kumahq/kuma/pkg/xds/envoy/listeners"

	util_proto "github.com/kumahq/kuma/pkg/util/proto"
	envoy_common "github.com/kumahq/kuma/pkg/xds/envoy"
)

var _ = Describe("OriginalDstForwarderConfigurer", func() {

	type testCase struct {
		listenerName     string
		listenerAddress  string
		listenerPort     uint32
		statsName        string
		listenerProtocol core_xds.SocketAddressProtocol
		clusters         []envoy_common.ClusterSubset
		expected         string
	}

	DescribeTable("should generate proper Envoy config",
		func(given testCase) {
			// when
			listener, err := NewListenerBuilder(envoy_common.APIV2).
				Configure(OutboundListener(given.listenerName, given.listenerAddress, given.listenerPort, given.listenerProtocol)).
				Configure(FilterChain(NewFilterChainBuilder(envoy_common.APIV2).
					Configure(TcpProxy(given.statsName, given.clusters...)))).
				Configure(OriginalDstForwarder()).
				Build()
			// then
			Expect(err).ToNot(HaveOccurred())

			// when
			actual, err := util_proto.ToYAML(listener)
			Expect(err).ToNot(HaveOccurred())
			// and
			Expect(actual).To(MatchYAML(given.expected))
		},
		Entry("basic tcp_proxy with original destination forwarder", testCase{
			listenerName:    "catch_all",
			listenerAddress: "0.0.0.0",
			listenerPort:    12345,
			statsName:       "pass_through",
			clusters:        []envoy_common.ClusterSubset{{ClusterName: "pass_through", Weight: 200}},
			expected: `
            name: catch_all
            trafficDirection: OUTBOUND
            address:
              socketAddress:
                address: 0.0.0.0
                portValue: 12345
            filterChains:
            - filters:
              - name: envoy.filters.network.tcp_proxy
                typedConfig:
                  '@type': type.googleapis.com/envoy.config.filter.network.tcp_proxy.v2.TcpProxy
                  cluster: pass_through
                  statPrefix: pass_through
            useOriginalDst: true
`,
		}),
	)

})
