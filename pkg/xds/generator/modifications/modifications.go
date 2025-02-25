package modifications

import (
	"github.com/pkg/errors"

	mesh_proto "github.com/kumahq/kuma/api/mesh/v1alpha1"
	core_xds "github.com/kumahq/kuma/pkg/core/xds"
	envoy_common "github.com/kumahq/kuma/pkg/xds/envoy"
	modifications_v2 "github.com/kumahq/kuma/pkg/xds/generator/modifications/v2"
	modifications_v3 "github.com/kumahq/kuma/pkg/xds/generator/modifications/v3"
)

func Apply(resources *core_xds.ResourceSet, modifications []*mesh_proto.ProxyTemplate_Modifications, apiVersion envoy_common.APIVersion) error {
	switch apiVersion {
	case envoy_common.APIV2:
		return modifications_v2.Apply(resources, modifications)
	case envoy_common.APIV3:
		return modifications_v3.Apply(resources, modifications)
	default:
		return errors.Errorf("unknown API version %s", apiVersion)
	}
}
