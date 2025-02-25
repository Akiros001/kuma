package v3

import (
	envoy_core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_grpc_credential "github.com/envoyproxy/go-control-plane/envoy/config/grpc_credential/v3"
	envoy_tls "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	envoy_type_matcher "github.com/envoyproxy/go-control-plane/envoy/type/matcher/v3"

	"github.com/kumahq/kuma/pkg/tls"
	"github.com/kumahq/kuma/pkg/util/proto"
	util_xds "github.com/kumahq/kuma/pkg/util/xds"

	"github.com/golang/protobuf/ptypes/wrappers"

	core_xds "github.com/kumahq/kuma/pkg/core/xds"
	xds_context "github.com/kumahq/kuma/pkg/xds/context"
	xds_tls "github.com/kumahq/kuma/pkg/xds/envoy/tls"
)

// SDS is served over the same server as XDS but it has to be a separate connection because it is not server over ADS
const sdsCluster = "ads_cluster"

// CreateDownstreamTlsContext creates DownstreamTlsContext for incoming connections
// It verifies that incoming connection has TLS certificate signed by Mesh CA with URI SAN of prefix spiffe://{mesh_name}/
// It secures inbound listener with certificate of "identity_cert" that will be received from the SDS (it contains URI SANs of all inbounds).
// Access to SDS is secured by TLS certificate (set in config or autogenerated at CP start) and path to dataplane token
func CreateDownstreamTlsContext(ctx xds_context.Context, metadata *core_xds.DataplaneMetadata) (*envoy_tls.DownstreamTlsContext, error) {
	if !ctx.Mesh.Resource.MTLSEnabled() {
		return nil, nil
	}
	validationSANMatcher := MeshSpiffeIDPrefixMatcher(ctx.Mesh.Resource.Meta.GetName())
	commonTlsContext, err := createCommonTlsContext(ctx, metadata, validationSANMatcher)
	if err != nil {
		return nil, err
	}
	return &envoy_tls.DownstreamTlsContext{
		CommonTlsContext:         commonTlsContext,
		RequireClientCertificate: &wrappers.BoolValue{Value: true},
	}, nil
}

// CreateUpstreamTlsContext creates UpstreamTlsContext for outgoing connections
// It verifies that the upstream server has TLS certificate signed by Mesh CA with URI SAN of spiffe://{mesh_name}/{upstream_service}
// The downstream client exposes for the upstream server cert with multiple URI SANs, which means that if DP has inbound with services "web" and "web-api" and communicates with "backend"
// the upstream server ("backend") will see that DP with TLS certificate of URIs of "web" and "web-api".
// There is no way to correlate incoming request to "web" or "web-api" with outgoing request to "backend" to expose only one URI SAN.
//
// Pass "*" for upstreamService to validate that upstream service is a service that is part of the mesh (but not specific one)
func CreateUpstreamTlsContext(ctx xds_context.Context, metadata *core_xds.DataplaneMetadata, upstreamService string, sni string) (*envoy_tls.UpstreamTlsContext, error) {
	if !ctx.Mesh.Resource.MTLSEnabled() {
		return nil, nil
	}
	var validationSANMatcher *envoy_type_matcher.StringMatcher
	if upstreamService == "*" {
		validationSANMatcher = MeshSpiffeIDPrefixMatcher(ctx.Mesh.Resource.Meta.GetName())
	} else {
		validationSANMatcher = ServiceSpiffeIDMatcher(ctx.Mesh.Resource.Meta.GetName(), upstreamService)
	}
	commonTlsContext, err := createCommonTlsContext(ctx, metadata, validationSANMatcher)
	if err != nil {
		return nil, err
	}
	return &envoy_tls.UpstreamTlsContext{
		CommonTlsContext: commonTlsContext,
		Sni:              sni,
	}, nil
}

func createCommonTlsContext(ctx xds_context.Context, metadata *core_xds.DataplaneMetadata, validationSANMatcher *envoy_type_matcher.StringMatcher) (*envoy_tls.CommonTlsContext, error) {
	meshCaSecret, err := sdsSecretConfig(ctx, xds_tls.MeshCaResource, metadata)
	if err != nil {
		return nil, err
	}
	identitySecret, err := sdsSecretConfig(ctx, xds_tls.IdentityCertResource, metadata)
	if err != nil {
		return nil, err
	}
	return &envoy_tls.CommonTlsContext{
		ValidationContextType: &envoy_tls.CommonTlsContext_CombinedValidationContext{
			CombinedValidationContext: &envoy_tls.CommonTlsContext_CombinedCertificateValidationContext{
				DefaultValidationContext: &envoy_tls.CertificateValidationContext{
					MatchSubjectAltNames: []*envoy_type_matcher.StringMatcher{validationSANMatcher},
				},
				ValidationContextSdsSecretConfig: meshCaSecret,
			},
		},
		TlsCertificateSdsSecretConfigs: []*envoy_tls.SdsSecretConfig{
			identitySecret,
		},
	}, nil
}

func sdsSecretConfig(context xds_context.Context, name string, metadata *core_xds.DataplaneMetadata) (*envoy_tls.SdsSecretConfig, error) {
	sdsConfig := &envoy_tls.SdsSecretConfig{
		Name: name,
		SdsConfig: &envoy_core.ConfigSource{
			ResourceApiVersion: envoy_core.ApiVersion_V3,
		},
	}
	if metadata.GetDataplaneTokenPath() != "" {
		specifier, err := googleGrpcSdsSpecifier(context, name, metadata)
		if err != nil {
			return nil, err
		}
		sdsConfig.SdsConfig.ConfigSourceSpecifier = specifier
	} else {
		sdsConfig.SdsConfig.ConfigSourceSpecifier = envoyGrpcSdsSpecifier(metadata)
	}
	return sdsConfig, nil
}

func envoyGrpcSdsSpecifier(metadata *core_xds.DataplaneMetadata) *envoy_core.ConfigSource_ApiConfigSource {
	specifier := &envoy_core.ConfigSource_ApiConfigSource{
		ApiConfigSource: &envoy_core.ApiConfigSource{
			ApiType:             envoy_core.ApiConfigSource_GRPC,
			TransportApiVersion: envoy_core.ApiVersion_V3,
			GrpcServices: []*envoy_core.GrpcService{
				{
					TargetSpecifier: &envoy_core.GrpcService_EnvoyGrpc_{
						EnvoyGrpc: &envoy_core.GrpcService_EnvoyGrpc{
							ClusterName: sdsCluster,
						},
					},
				},
			},
		},
	}
	if metadata.GetDataplaneToken() != "" {
		specifier.ApiConfigSource.GrpcServices[0].InitialMetadata = []*envoy_core.HeaderValue{
			{
				Key:   "authorization",
				Value: metadata.DataplaneToken,
			},
		}
	}
	return specifier
}

func googleGrpcSdsSpecifier(context xds_context.Context, name string, metadata *core_xds.DataplaneMetadata) (*envoy_core.ConfigSource_ApiConfigSource, error) {
	withCallCredentials := func(grpc *envoy_core.GrpcService_GoogleGrpc) (*envoy_core.GrpcService_GoogleGrpc, error) {
		if metadata.GetDataplaneTokenPath() == "" {
			return grpc, nil
		}

		config := &envoy_grpc_credential.FileBasedMetadataConfig{
			SecretData: &envoy_core.DataSource{
				Specifier: &envoy_core.DataSource_Filename{
					Filename: metadata.GetDataplaneTokenPath(),
				},
			},
		}
		typedConfig, err := proto.MarshalAnyDeterministic(config)
		if err != nil {
			return nil, err
		}

		grpc.CallCredentials = append(grpc.CallCredentials, &envoy_core.GrpcService_GoogleGrpc_CallCredentials{
			CredentialSpecifier: &envoy_core.GrpcService_GoogleGrpc_CallCredentials_FromPlugin{
				FromPlugin: &envoy_core.GrpcService_GoogleGrpc_CallCredentials_MetadataCredentialsFromPlugin{
					Name: "envoy.grpc_credentials.file_based_metadata",
					ConfigType: &envoy_core.GrpcService_GoogleGrpc_CallCredentials_MetadataCredentialsFromPlugin_TypedConfig{
						TypedConfig: typedConfig,
					},
				},
			},
		})
		grpc.CredentialsFactoryName = "envoy.grpc_credentials.file_based_metadata"

		return grpc, nil
	}
	googleGrpc, err := withCallCredentials(&envoy_core.GrpcService_GoogleGrpc{
		TargetUri:  context.SDSLocation(),
		StatPrefix: util_xds.SanitizeMetric("sds_" + name),
		ChannelCredentials: &envoy_core.GrpcService_GoogleGrpc_ChannelCredentials{
			CredentialSpecifier: &envoy_core.GrpcService_GoogleGrpc_ChannelCredentials_SslCredentials{
				SslCredentials: &envoy_core.GrpcService_GoogleGrpc_SslCredentials{
					RootCerts: &envoy_core.DataSource{
						Specifier: &envoy_core.DataSource_InlineBytes{
							InlineBytes: context.ControlPlane.SdsTlsCert,
						},
					},
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	return &envoy_core.ConfigSource_ApiConfigSource{
		ApiConfigSource: &envoy_core.ApiConfigSource{
			ApiType:             envoy_core.ApiConfigSource_GRPC,
			TransportApiVersion: envoy_core.ApiVersion_V3,
			GrpcServices: []*envoy_core.GrpcService{
				{
					TargetSpecifier: &envoy_core.GrpcService_GoogleGrpc_{
						GoogleGrpc: googleGrpc,
					},
				},
			},
		},
	}, nil
}

func UpstreamTlsContextOutsideMesh(ca, cert, key []byte, hostname string) (*envoy_tls.UpstreamTlsContext, error) {
	var tlsCertificates []*envoy_tls.TlsCertificate
	if cert != nil && key != nil {
		tlsCertificates = []*envoy_tls.TlsCertificate{
			{
				CertificateChain: dataSourceFromBytes(cert),
				PrivateKey:       dataSourceFromBytes(key),
			},
		}
	}

	var validationContextType *envoy_tls.CommonTlsContext_ValidationContext
	if ca != nil {
		validationContextType = &envoy_tls.CommonTlsContext_ValidationContext{
			ValidationContext: &envoy_tls.CertificateValidationContext{
				TrustedCa: dataSourceFromBytes(ca),
				MatchSubjectAltNames: []*envoy_type_matcher.StringMatcher{
					{
						MatchPattern: &envoy_type_matcher.StringMatcher_Exact{
							Exact: hostname,
						},
					},
				},
			},
		}
	}

	return &envoy_tls.UpstreamTlsContext{
		CommonTlsContext: &envoy_tls.CommonTlsContext{
			TlsCertificates:       tlsCertificates,
			ValidationContextType: validationContextType,
		},
	}, nil
}

func dataSourceFromBytes(bytes []byte) *envoy_core.DataSource {
	return &envoy_core.DataSource{
		Specifier: &envoy_core.DataSource_InlineBytes{
			InlineBytes: bytes,
		},
	}
}

func MeshSpiffeIDPrefixMatcher(mesh string) *envoy_type_matcher.StringMatcher {
	return &envoy_type_matcher.StringMatcher{
		MatchPattern: &envoy_type_matcher.StringMatcher_Prefix{
			Prefix: xds_tls.MeshSpiffeIDPrefix(mesh),
		},
	}
}

func ServiceSpiffeIDMatcher(mesh string, service string) *envoy_type_matcher.StringMatcher {
	return &envoy_type_matcher.StringMatcher{
		MatchPattern: &envoy_type_matcher.StringMatcher_Exact{
			Exact: xds_tls.ServiceSpiffeID(mesh, service),
		},
	}
}

func KumaIDMatcher(tagName, tagValue string) *envoy_type_matcher.StringMatcher {
	return &envoy_type_matcher.StringMatcher{
		MatchPattern: &envoy_type_matcher.StringMatcher_Exact{
			Exact: xds_tls.KumaID(tagName, tagValue),
		},
	}
}

func StaticDownstreamTlsContext(keyPair *tls.KeyPair) *envoy_tls.DownstreamTlsContext {
	return &envoy_tls.DownstreamTlsContext{
		CommonTlsContext: &envoy_tls.CommonTlsContext{
			TlsCertificates: []*envoy_tls.TlsCertificate{
				{
					CertificateChain: &envoy_core.DataSource{
						Specifier: &envoy_core.DataSource_InlineBytes{
							InlineBytes: keyPair.CertPEM,
						},
					},
					PrivateKey: &envoy_core.DataSource{
						Specifier: &envoy_core.DataSource_InlineBytes{
							InlineBytes: keyPair.KeyPEM,
						},
					},
				},
			},
		},
	}
}
