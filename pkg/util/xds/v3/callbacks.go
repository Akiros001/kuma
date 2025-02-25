package v3

import (
	"context"

	envoy_sd "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	envoy_xds "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	_struct "github.com/golang/protobuf/ptypes/struct"

	"github.com/kumahq/kuma/pkg/util/xds"
)

// stream callbacks

type adapterCallbacks struct {
	callbacks xds.Callbacks
}

// AdaptCallbacks translate Kuma callbacks to real go-control-plane Callbacks
func AdaptCallbacks(callbacks xds.Callbacks) envoy_xds.Callbacks {
	return &adapterCallbacks{
		callbacks: callbacks,
	}
}

var _ envoy_xds.Callbacks = &adapterCallbacks{}

func (a *adapterCallbacks) OnFetchRequest(ctx context.Context, request *envoy_sd.DiscoveryRequest) error {
	panic("implement me")
}

func (a *adapterCallbacks) OnFetchResponse(request *envoy_sd.DiscoveryRequest, response *envoy_sd.DiscoveryResponse) {
	panic("implement me")
}

func (a *adapterCallbacks) OnStreamOpen(ctx context.Context, streamID int64, typeURL string) error {
	return a.callbacks.OnStreamOpen(ctx, streamID, typeURL)
}

func (a *adapterCallbacks) OnStreamClosed(streamID int64) {
	a.callbacks.OnStreamClosed(streamID)
}

func (a *adapterCallbacks) OnStreamRequest(streamID int64, request *envoy_sd.DiscoveryRequest) error {
	return a.callbacks.OnStreamRequest(streamID, &discoveryRequest{request})
}

func (a *adapterCallbacks) OnStreamResponse(streamID int64, request *envoy_sd.DiscoveryRequest, response *envoy_sd.DiscoveryResponse) {
	a.callbacks.OnStreamResponse(streamID, &discoveryRequest{request}, &discoveryResponse{response})
}

// rest callbacks

type adapterRestCallbacks struct {
	callbacks xds.RestCallbacks
}

// AdaptRestCallbacks translate Kuma callbacks to real go-control-plane Callbacks
func AdaptRestCallbacks(callbacks xds.RestCallbacks) envoy_xds.Callbacks {
	return &adapterRestCallbacks{
		callbacks: callbacks,
	}
}

func (a *adapterRestCallbacks) OnFetchRequest(ctx context.Context, request *envoy_sd.DiscoveryRequest) error {
	return a.callbacks.OnFetchRequest(ctx, &discoveryRequest{request})
}

func (a *adapterRestCallbacks) OnFetchResponse(request *envoy_sd.DiscoveryRequest, response *envoy_sd.DiscoveryResponse) {
	a.callbacks.OnFetchResponse(&discoveryRequest{request}, &discoveryResponse{response})
}

func (a *adapterRestCallbacks) OnStreamOpen(ctx context.Context, streamID int64, typeURL string) error {
	return nil
}

func (a *adapterRestCallbacks) OnStreamClosed(streamID int64) {
}

func (a *adapterRestCallbacks) OnStreamRequest(streamID int64, request *envoy_sd.DiscoveryRequest) error {
	return nil
}

func (a *adapterRestCallbacks) OnStreamResponse(streamID int64, request *envoy_sd.DiscoveryRequest, response *envoy_sd.DiscoveryResponse) {
}

// Both rest and stream

type adapterMultiCallbacks struct {
	callbacks xds.MultiCallbacks
}

// AdaptMultiCallbacks translate Kuma callbacks to real go-control-plane Callbacks
func AdaptMultiCallbacks(callbacks xds.MultiCallbacks) envoy_xds.Callbacks {
	return &adapterMultiCallbacks{
		callbacks: callbacks,
	}
}

func (a *adapterMultiCallbacks) OnFetchRequest(ctx context.Context, request *envoy_sd.DiscoveryRequest) error {
	return a.callbacks.OnFetchRequest(ctx, &discoveryRequest{request})
}

func (a *adapterMultiCallbacks) OnFetchResponse(request *envoy_sd.DiscoveryRequest, response *envoy_sd.DiscoveryResponse) {
	a.callbacks.OnFetchResponse(&discoveryRequest{request}, &discoveryResponse{response})
}

func (a *adapterMultiCallbacks) OnStreamOpen(ctx context.Context, streamID int64, typeURL string) error {
	return a.callbacks.OnStreamOpen(ctx, streamID, typeURL)
}

func (a *adapterMultiCallbacks) OnStreamClosed(streamID int64) {
	a.callbacks.OnStreamClosed(streamID)
}

func (a *adapterMultiCallbacks) OnStreamRequest(streamID int64, request *envoy_sd.DiscoveryRequest) error {
	return a.callbacks.OnStreamRequest(streamID, &discoveryRequest{request})
}

func (a *adapterMultiCallbacks) OnStreamResponse(streamID int64, request *envoy_sd.DiscoveryRequest, response *envoy_sd.DiscoveryResponse) {
	a.callbacks.OnStreamResponse(streamID, &discoveryRequest{request}, &discoveryResponse{response})
}

// DiscoveryRequest facade

type discoveryRequest struct {
	*envoy_sd.DiscoveryRequest
}

func (d *discoveryRequest) Metadata() *_struct.Struct {
	return d.GetNode().GetMetadata()
}

func (d *discoveryRequest) VersionInfo() string {
	return d.GetVersionInfo()
}

func (d *discoveryRequest) NodeId() string {
	return d.GetNode().GetId()
}

func (d *discoveryRequest) Node() interface{} {
	return d.GetNode()
}

func (d *discoveryRequest) HasErrors() bool {
	return d.ErrorDetail != nil
}

func (d *discoveryRequest) ErrorMsg() string {
	return d.GetErrorDetail().GetMessage()
}

var _ xds.DiscoveryRequest = &discoveryRequest{}

type discoveryResponse struct {
	*envoy_sd.DiscoveryResponse
}
