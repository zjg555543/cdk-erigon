package communication

import (
	"context"
	"fmt"
	"io"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

var NoRequestHandlers = map[string]bool{
	MetadataProtocolV1: true,
	MetadataProtocolV2: true,
}

type response struct {
	data []byte
	code byte
	err  error
}

func SendRequestRawToPeer(ctx context.Context, host host.Host, data []byte, topic string, peerId peer.ID) ([]byte, byte, error) {
	stream, err := writeRequestRaw(host, ctx, data, peerId, topic)
	if err != nil {
		return nil, 189, err
	}
	defer stream.Close()
	res := verifyResponse(ctx, stream, peerId)
	return res.data, res.code, res.err
}

func writeRequestRaw(host host.Host, ctx context.Context, data []byte, peerId peer.ID, topic string) (network.Stream, error) {
	stream, err := host.NewStream(ctx, peerId, protocol.ID(topic))
	if err != nil {
		return nil, fmt.Errorf("failed to begin stream, err=%s", err)
	}

	if deadline, ok := ctx.Deadline(); ok {
		stream.SetWriteDeadline(deadline)
	}

	if _, ok := NoRequestHandlers[topic]; !ok {
		if _, err := stream.Write(data); err != nil {
			return nil, err
		}
	}

	return stream, stream.CloseWrite()
}

func verifyResponse(ctx context.Context, stream network.Stream, peerId peer.ID) (resp response) {
	code := make([]byte, 1)
	if deadline, ok := ctx.Deadline(); ok {
		stream.SetReadDeadline(deadline)
	}
	_, resp.err = io.ReadFull(stream, code)
	if resp.err != nil {
		return
	}
	resp.code = code[0]
	resp.data, resp.err = io.ReadAll(stream)
	if resp.err != nil {
		return
	}
	return
}
