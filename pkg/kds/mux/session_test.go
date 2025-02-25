package mux_test

import (
	"context"

	envoy_api_v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	mesh_proto "github.com/kumahq/kuma/api/mesh/v1alpha1"
	"github.com/kumahq/kuma/pkg/kds/mux"
)

type testMultiplexStream struct {
	input  chan *mesh_proto.Message
	output chan *mesh_proto.Message
}

func (t *testMultiplexStream) Send(msg *mesh_proto.Message) error {
	t.output <- msg
	return nil
}

func (t *testMultiplexStream) Recv() (*mesh_proto.Message, error) {
	return <-t.input, nil
}

func (t *testMultiplexStream) Context() context.Context {
	panic("implement me")
}

var _ = Describe("Multiplex Session", func() {

	Context("basic Send/Recv operations", func() {
		var clientSession mux.Session
		var serverSession mux.Session

		BeforeEach(func() {
			input := make(chan *mesh_proto.Message, 1)
			output := make(chan *mesh_proto.Message, 1)
			clientSession = mux.NewSession("global", &testMultiplexStream{input: input, output: output}, nil)
			serverSession = mux.NewSession("remote-1", &testMultiplexStream{input: output, output: input}, nil)
		})

		It("should Send to clientSession's ClientStream and Recv from serverSession's ServerStream", func() {
			err := clientSession.ClientStream().Send(&envoy_api_v2.DiscoveryRequest{VersionInfo: "1"})
			Expect(err).ToNot(HaveOccurred())
			msg, err := serverSession.ServerStream().Recv()
			Expect(err).ToNot(HaveOccurred())
			Expect(msg.VersionInfo).To(Equal("1"))
		})
		It("should Send to serverSession's ServerStream and Recv from clientSession's ClientStream", func() {
			err := serverSession.ServerStream().Send(&envoy_api_v2.DiscoveryResponse{VersionInfo: "2"})
			Expect(err).ToNot(HaveOccurred())
			msg, err := clientSession.ClientStream().Recv()
			Expect(err).ToNot(HaveOccurred())
			Expect(msg.VersionInfo).To(Equal("2"))
		})
		It("should Send to clientSession's ServerStream and Recv from serverSession's ClientStream", func() {
			err := clientSession.ServerStream().Send(&envoy_api_v2.DiscoveryResponse{VersionInfo: "3"})
			Expect(err).ToNot(HaveOccurred())
			msg, err := serverSession.ClientStream().Recv()
			Expect(err).ToNot(HaveOccurred())
			Expect(msg.VersionInfo).To(Equal("3"))
		})
		It("should Send to serverSession's ClientStream and Recv from clientSession's ServerStream", func() {
			err := serverSession.ClientStream().Send(&envoy_api_v2.DiscoveryRequest{VersionInfo: "4"})
			Expect(err).ToNot(HaveOccurred())
			msg, err := clientSession.ServerStream().Recv()
			Expect(err).ToNot(HaveOccurred())
			Expect(msg.VersionInfo).To(Equal("4"))
		})
	})

	Context("concurrent operations", func() {
		var clientSession mux.Session
		var serverSession mux.Session

		BeforeEach(func() {
			input := make(chan *mesh_proto.Message, 1)
			output := make(chan *mesh_proto.Message, 1)
			clientSession = mux.NewSession("global", &testMultiplexStream{input: input, output: output}, nil)
			serverSession = mux.NewSession("remote-1", &testMultiplexStream{input: output, output: input}, nil)
		})

		Context("Recv", func() {
			It("should block while proper Send called", func(done Done) {
				go func() {
					request, err := serverSession.ServerStream().Recv()
					Expect(err).ToNot(HaveOccurred())
					Expect(request.VersionInfo).To(Equal("1"))
					close(done)
				}()
				err := clientSession.ServerStream().Send(&envoy_api_v2.DiscoveryResponse{VersionInfo: "2"})
				Expect(err).ToNot(HaveOccurred())
				err = clientSession.ClientStream().Send(&envoy_api_v2.DiscoveryRequest{VersionInfo: "1"})
				Expect(err).ToNot(HaveOccurred())

				resp, err := serverSession.ClientStream().Recv()
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.VersionInfo).To(Equal("2"))
			}, 1)
		})
	})
})
