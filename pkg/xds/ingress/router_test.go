package ingress

import (
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	mesh_proto "github.com/kumahq/kuma/api/mesh/v1alpha1"
	"github.com/kumahq/kuma/pkg/core/resources/apis/mesh"
	"github.com/kumahq/kuma/pkg/core/xds"
)

var _ = Describe("Ingress BuildDestinationMap", func() {
	It("should generate destination map by ingress", func() {
		ingress := &mesh.DataplaneResource{
			Spec: &mesh_proto.Dataplane{
				Networking: &mesh_proto.Dataplane_Networking{
					Ingress: &mesh_proto.Dataplane_Networking_Ingress{
						AvailableServices: []*mesh_proto.Dataplane_Networking_Ingress_AvailableService{
							{
								Tags: map[string]string{"kuma.io/service": "backend", "version": "v1", "region": "us"},
							},
							{
								Tags: map[string]string{"kuma.io/service": "backend"},
							},
							{
								Tags: map[string]string{"kuma.io/service": "web", "version": "v2", "region": "eu"},
							},
						},
					},
				},
			},
		}
		actual := BuildDestinationMap(ingress)
		expected := xds.DestinationMap{
			"backend": []mesh_proto.TagSelector{
				{
					"region":          "us",
					"version":         "v1",
					"kuma.io/service": "backend",
				},
				{
					"kuma.io/service": "backend",
				},
			},
			"web": []mesh_proto.TagSelector{
				{
					"region":          "eu",
					"kuma.io/service": "web",
					"version":         "v2",
				},
			},
		}
		gomega.Expect(actual).To(gomega.Equal(expected))
	})
})
