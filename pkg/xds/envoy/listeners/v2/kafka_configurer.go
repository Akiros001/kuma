package v2

import (
	envoy_listener "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	envoy_kafka "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/kafka_broker/v2alpha1"

	util_xds "github.com/kumahq/kuma/pkg/util/xds"

	"github.com/kumahq/kuma/pkg/util/proto"
)

type KafkaConfigurer struct {
	StatsName string
}

var _ FilterChainConfigurer = &KafkaConfigurer{}

func (c *KafkaConfigurer) Configure(filterChain *envoy_listener.FilterChain) error {
	pbst, err := proto.MarshalAnyDeterministic(
		&envoy_kafka.KafkaBroker{
			StatPrefix: util_xds.SanitizeMetric(c.StatsName),
		})
	if err != nil {
		return err
	}

	filterChain.Filters = append([]*envoy_listener.Filter{
		{
			Name: "envoy.filters.network.kafka_broker",
			ConfigType: &envoy_listener.Filter_TypedConfig{
				TypedConfig: pbst,
			},
		},
	}, filterChain.Filters...)
	return nil
}
