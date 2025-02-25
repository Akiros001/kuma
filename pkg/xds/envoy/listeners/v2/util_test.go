package v2_test

import (
	"errors"

	envoy_type "github.com/envoyproxy/go-control-plane/envoy/type"
	"github.com/golang/protobuf/ptypes/wrappers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	. "github.com/kumahq/kuma/pkg/xds/envoy/listeners/v2"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"

	envoy_listener "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	envoy_hcm "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	envoy_tcp "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/tcp_proxy/v2"

	util_error "github.com/kumahq/kuma/pkg/util/error"
	util_proto "github.com/kumahq/kuma/pkg/util/proto"
)

var _ = Describe("UpdateFilterConfig()", func() {

	Context("happy path", func() {
		type testCase struct {
			filterChain *envoy_listener.FilterChain
			filterName  string
			updateFunc  func(proto.Message) error
			expected    string
		}

		DescribeTable("should update filter config",
			func(given testCase) {
				// when
				err := UpdateFilterConfig(given.filterChain, given.filterName, given.updateFunc)
				// then
				Expect(err).ToNot(HaveOccurred())

				// when
				actual, err := util_proto.ToYAML(given.filterChain)
				// then
				Expect(err).ToNot(HaveOccurred())
				// and
				Expect(actual).To(MatchYAML(given.expected))
			},
			Entry("0 filters", testCase{
				filterChain: &envoy_listener.FilterChain{},
				filterName:  "envoy.filters.network.tcp_proxy",
				updateFunc:  func(proto.Message) error { return errors.New("should never happen") },
				expected:    `{}`,
			}),
			Entry("1 filter", func() testCase {
				pbst, err := ptypes.MarshalAny(&envoy_tcp.TcpProxy{})
				util_error.MustNot(err)
				return testCase{
					filterChain: &envoy_listener.FilterChain{
						Filters: []*envoy_listener.Filter{{
							Name: "envoy.filters.network.tcp_proxy",
							ConfigType: &envoy_listener.Filter_TypedConfig{
								TypedConfig: pbst,
							},
						}},
					},
					filterName: "envoy.filters.network.tcp_proxy",
					updateFunc: func(filterConfig proto.Message) error {
						proxy := filterConfig.(*envoy_tcp.TcpProxy)
						proxy.ClusterSpecifier = &envoy_tcp.TcpProxy_Cluster{
							Cluster: "backend",
						}
						return nil
					},
					expected: `
                    filters:
                    - name: envoy.filters.network.tcp_proxy
                      typedConfig:
                        '@type': type.googleapis.com/envoy.config.filter.network.tcp_proxy.v2.TcpProxy
                        cluster: backend
`,
				}
			}()),
			Entry("2 filters", func() testCase {
				pbst, err := ptypes.MarshalAny(&envoy_tcp.TcpProxy{})
				util_error.MustNot(err)
				return testCase{
					filterChain: &envoy_listener.FilterChain{
						Filters: []*envoy_listener.Filter{
							{
								Name: "envoy.filters.network.rbac",
							},
							{
								Name: "envoy.filters.network.tcp_proxy",
								ConfigType: &envoy_listener.Filter_TypedConfig{
									TypedConfig: pbst,
								},
							},
						},
					},
					filterName: "envoy.filters.network.tcp_proxy",
					updateFunc: func(filterConfig proto.Message) error {
						proxy := filterConfig.(*envoy_tcp.TcpProxy)
						proxy.ClusterSpecifier = &envoy_tcp.TcpProxy_Cluster{
							Cluster: "backend",
						}
						return nil
					},
					expected: `
                    filters:
                    - name: envoy.filters.network.rbac
                    - name: envoy.filters.network.tcp_proxy
                      typedConfig:
                        '@type': type.googleapis.com/envoy.config.filter.network.tcp_proxy.v2.TcpProxy
                        cluster: backend
`,
				}
			}()),
		)
	})

	Context("error path", func() {

		type testCase struct {
			filterChain *envoy_listener.FilterChain
			filterName  string
			updateFunc  func(proto.Message) error
			expectedErr string
		}

		DescribeTable("should return an error",
			func(given testCase) {
				// when
				err := UpdateFilterConfig(given.filterChain, given.filterName, given.updateFunc)
				// then
				Expect(err).To(HaveOccurred())
				// and
				Expect(err.Error()).To(Equal(given.expectedErr))
			},
			Entry("1 filter without config", testCase{
				filterChain: &envoy_listener.FilterChain{
					Filters: []*envoy_listener.Filter{{
						Name: "envoy.filters.network.tcp_proxy",
					}},
				},
				filterName:  "envoy.filters.network.tcp_proxy",
				updateFunc:  func(proto.Message) error { return errors.New("should never happen") },
				expectedErr: `filters[0]: config cannot be 'nil'`,
			}),
			Entry("1 filter with a wrong config type", func() testCase {
				pbst, err := ptypes.MarshalAny(&envoy_hcm.HttpConnectionManager{})
				util_error.MustNot(err)
				return testCase{
					filterChain: &envoy_listener.FilterChain{
						Filters: []*envoy_listener.Filter{{
							Name: "envoy.filters.network.tcp_proxy",
							ConfigType: &envoy_listener.Filter_TypedConfig{
								TypedConfig: pbst,
							},
						}},
					},
					filterName:  "envoy.filters.network.tcp_proxy",
					updateFunc:  func(proto.Message) error { return errors.New("wrong config type") },
					expectedErr: `wrong config type`,
				}
			}()),
		)
	})
})

var _ = Describe("NewUnexpectedFilterConfigTypeError()", func() {

	type testCase struct {
		inputActual   proto.Message
		inputExpected proto.Message
		expectedErr   string
	}

	DescribeTable("should generate proper error message",
		func(given testCase) {
			// when
			err := NewUnexpectedFilterConfigTypeError(given.inputActual, given.inputExpected)
			// then
			Expect(err).To(HaveOccurred())
			// and
			Expect(err.Error()).To(Equal(given.expectedErr))
		},
		Entry("TcpProxy instead of HttpConnectionManager", testCase{
			inputActual:   &envoy_tcp.TcpProxy{},
			inputExpected: &envoy_hcm.HttpConnectionManager{},
			expectedErr:   `filter config has unexpected type: expected *envoy_config_filter_network_http_connection_manager_v2.HttpConnectionManager, got *envoy_config_filter_network_tcp_proxy_v2.TcpProxy`,
		}),
	)
})

var _ = Describe("ConvertPercentage", func() {
	type testCase struct {
		input    *wrappers.DoubleValue
		expected *envoy_type.FractionalPercent
	}
	DescribeTable("should properly converts from percent to fractional percen",
		func(given testCase) {
			fpercent := ConvertPercentage(given.input)
			Expect(fpercent).To(Equal(given.expected))
		},
		Entry("integer input", testCase{
			input:    &wrappers.DoubleValue{Value: 50},
			expected: &envoy_type.FractionalPercent{Numerator: 50, Denominator: envoy_type.FractionalPercent_HUNDRED},
		}),
		Entry("fractional input with 1 digit after dot", testCase{
			input:    &wrappers.DoubleValue{Value: 50.1},
			expected: &envoy_type.FractionalPercent{Numerator: 501000, Denominator: envoy_type.FractionalPercent_TEN_THOUSAND},
		}),
		Entry("fractional input with 5 digit after dot", testCase{
			input:    &wrappers.DoubleValue{Value: 50.12345},
			expected: &envoy_type.FractionalPercent{Numerator: 50123450, Denominator: envoy_type.FractionalPercent_MILLION},
		}),
		Entry("fractional input with 7 digit after dot, last digit less than 5", testCase{
			input:    &wrappers.DoubleValue{Value: 50.1234561},
			expected: &envoy_type.FractionalPercent{Numerator: 50123456, Denominator: envoy_type.FractionalPercent_MILLION},
		}),
		Entry("fractional input with 7 digit after dot, last digit more than 5", testCase{
			input:    &wrappers.DoubleValue{Value: 50.1234567},
			expected: &envoy_type.FractionalPercent{Numerator: 50123457, Denominator: envoy_type.FractionalPercent_MILLION},
		}),
	)
})

var _ = Describe("ConvertBandwidth", func() {
	type testCase struct {
		input    string
		expected uint64
	}
	DescribeTable("should properly converts to kbps from gbps, mbps, kbps",
		func(given testCase) {
			// when
			limitKbps, err := ConvertBandwidthToKbps(given.input)
			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(limitKbps).To(Equal(given.expected))
		},
		Entry("kbps input", testCase{
			input:    "120 kbps",
			expected: 120,
		}),
		Entry("mbps input", testCase{
			input:    "120 mbps",
			expected: 120000,
		}),
		Entry("gbps input", testCase{
			input:    "120 gbps",
			expected: 120000000,
		}),
	)
})
