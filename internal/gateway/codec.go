package gateway

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"

	vttv1 "github.com/PatrikLager/vtt-platform/contract/gen/go/vtt/v1"
)

// EncodeFrame marshals frame to its protojson wire representation (spec §3:
// one WebSocket endpoint, protojson TEXT frames). Frames are marshaled per
// connection, by each connection's own pump — not once and fanned out — see
// server.go for the rationale.
// The encode seam lives on Server (server.go's encodeFrame field), NOT on a
// package-global var. It was global until 2026-08-07, when review found a DATA
// RACE: presence made encodeFrame reachable from a connection's TEARDOWN, so
// one test swapping the global raced a DIFFERENT connection unwinding after
// its own test had returned. A per-Server field cannot race across servers,
// and each test owns the Server it builds.

func EncodeFrame(frame *vttv1.ServerFrame) ([]byte, error) {
	b, err := protojson.Marshal(frame)
	if err != nil {
		return nil, fmt.Errorf("gateway: encode frame: %w", err)
	}
	return b, nil
}

// DecodeCommand unmarshals raw protojson bytes into a ClientCommand.
func DecodeCommand(raw []byte) (*vttv1.ClientCommand, error) {
	var cmd vttv1.ClientCommand
	if err := protojson.Unmarshal(raw, &cmd); err != nil {
		return nil, fmt.Errorf("gateway: decode command: %w", err)
	}
	return &cmd, nil
}
