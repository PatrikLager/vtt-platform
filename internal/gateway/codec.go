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
