package xds

import (
	"strconv"

	_struct "github.com/golang/protobuf/ptypes/struct"

	core_mesh "github.com/kumahq/kuma/pkg/core/resources/apis/mesh"
	"github.com/kumahq/kuma/pkg/core/resources/model/rest"

	"github.com/kumahq/kuma/pkg/core"
)

var metadataLog = core.Log.WithName("xds-server").WithName("metadata-tracker")

const (
	// Supported Envoy node metadata fields.

	fieldDataplaneTokenPath         = "dataplaneTokenPath"
	fieldDataplaneToken             = "dataplane.token"
	fieldDataplaneAdminPort         = "dataplane.admin.port"
	fieldDataplaneDNSPort           = "dataplane.dns.port"
	fieldDataplaneDNSEmptyPort      = "dataplane.dns.empty.port"
	fieldDataplaneDataplaneResource = "dataplane.resource"
	fieldDynamicMetadata            = "dynamicMetadata"
)

// DataplaneMetadata represents environment-specific part of a dataplane configuration.
//
// This information might change from one dataplane run to another
// and therefore it cannot be a part of Dataplane resource.
//
// On start-up, a dataplane captures its effective configuration (that might come
// from a file, environment variables and command line options) and includes it
// into request for a bootstrap config.
// Control Plane can use this information to fill in node metadata in the bootstrap
// config.
// Envoy will include node metadata from the bootstrap config
// at least into the very first discovery request on every xDS stream.
// This way, xDS server will be able to use Envoy node metadata
// to generate xDS resources that depend on environment-specific configuration.
type DataplaneMetadata struct {
	DataplaneTokenPath string
	DataplaneToken     string
	DataplaneResource  *core_mesh.DataplaneResource
	AdminPort          uint32
	DNSPort            uint32
	EmptyDNSPort       uint32
	DynamicMetadata    map[string]string
}

func (m *DataplaneMetadata) GetDataplaneTokenPath() string {
	if m == nil {
		return ""
	}
	return m.DataplaneTokenPath
}

func (m *DataplaneMetadata) GetDataplaneToken() string {
	if m == nil {
		return ""
	}
	return m.DataplaneToken
}

func (m *DataplaneMetadata) GetDataplaneResource() *core_mesh.DataplaneResource {
	if m == nil {
		return nil
	}
	return m.DataplaneResource
}

func (m *DataplaneMetadata) GetAdminPort() uint32 {
	if m == nil {
		return 0
	}
	return m.AdminPort
}

func (m *DataplaneMetadata) GetDNSPort() uint32 {
	if m == nil {
		return 0
	}
	return m.DNSPort
}

func (m *DataplaneMetadata) GetEmptyDNSPort() uint32 {
	if m == nil {
		return 0
	}
	return m.EmptyDNSPort
}

func (m *DataplaneMetadata) GetDynamicMetadata(key string) string {
	if m == nil || m.DynamicMetadata == nil {
		return ""
	}
	return m.DynamicMetadata[key]
}

func DataplaneMetadataFromXdsMetadata(xdsMetadata *_struct.Struct) *DataplaneMetadata {
	metadata := DataplaneMetadata{}
	if xdsMetadata == nil {
		return &metadata
	}
	if field := xdsMetadata.Fields[fieldDataplaneTokenPath]; field != nil {
		metadata.DataplaneTokenPath = field.GetStringValue()
	}
	if field := xdsMetadata.Fields[fieldDataplaneToken]; field != nil {
		metadata.DataplaneToken = field.GetStringValue()
	}
	metadata.AdminPort = uint32Metadata(xdsMetadata, fieldDataplaneAdminPort)
	metadata.DNSPort = uint32Metadata(xdsMetadata, fieldDataplaneDNSPort)
	metadata.EmptyDNSPort = uint32Metadata(xdsMetadata, fieldDataplaneDNSEmptyPort)
	if value := xdsMetadata.Fields[fieldDataplaneDataplaneResource]; value != nil {
		res, err := rest.UnmarshallToCore([]byte(value.GetStringValue()))
		if err != nil {
			metadataLog.Error(err, "invalid value in dataplane metadata", "field", fieldDataplaneDataplaneResource, "value", value)
		}
		dp, ok := res.(*core_mesh.DataplaneResource)
		if !ok {
			metadataLog.Error(err, "invalid value in dataplane metadata", "field", fieldDataplaneDataplaneResource, "value", value)
		}
		metadata.DataplaneResource = dp
	}
	if value := xdsMetadata.Fields[fieldDynamicMetadata]; value != nil {
		dynamicMetadata := map[string]string{}
		for field, val := range value.GetStructValue().GetFields() {
			dynamicMetadata[field] = val.GetStringValue()
		}
		metadata.DynamicMetadata = dynamicMetadata
	}
	return &metadata
}

func uint32Metadata(xdsMetadata *_struct.Struct, field string) uint32 {
	value := xdsMetadata.Fields[field]
	if value == nil {
		return 0
	}
	port, err := strconv.Atoi(value.GetStringValue())
	if err != nil {
		metadataLog.Error(err, "invalid value in dataplane metadata", "field", field, "value", value)
		return 0
	}
	return uint32(port)
}
