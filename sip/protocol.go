// Package sip defines the Sip-compatible wire protocol: message type
// constants, structured message types, and WebSocket/WebTransport
// encode/decode helpers shared by the server and any future clients.
package sip

import (
	"encoding/binary"
	"fmt"
)

// Message type constants — Sip-compatible ('0'-'7') plus Ghostty extension ('8').
const (
	MsgInput    byte = '0' // Terminal input (client → server)
	MsgOutput   byte = '1' // Terminal output (server → client)
	MsgResize   byte = '2' // Resize terminal (client → server)
	MsgPing     byte = '3' // Keepalive (client → server)
	MsgPong     byte = '4' // Keepalive response (server → client)
	MsgTitle    byte = '5' // Window title (server → client)
	MsgOptions  byte = '6' // Session config (server → client)
	MsgClose    byte = '7' // Session ended (server → client)
	MsgKittyKbd byte = '8' // Kitty keyboard state (bidirectional)
)

// MaxMessageSize is the maximum allowed message size (1MB).
const MaxMessageSize = 1 << 20

// ResizeMessage carries terminal dimensions.
type ResizeMessage struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

// OptionsMessage carries session configuration sent on connect.
type OptionsMessage struct {
	ReadOnly bool `json:"readOnly"`
}

// KittyKbdMessage carries Kitty keyboard protocol flag state.
type KittyKbdMessage struct {
	Flags int `json:"flags"`
}

// EncodeWSMessage encodes a WebSocket protocol message: [type][payload].
func EncodeWSMessage(msgType byte, payload []byte) []byte {
	msg := make([]byte, 1+len(payload))
	msg[0] = msgType
	copy(msg[1:], payload)
	return msg
}

// DecodeWSMessage decodes a WebSocket protocol message.
func DecodeWSMessage(data []byte) (msgType byte, payload []byte, err error) {
	if len(data) == 0 {
		return 0, nil, fmt.Errorf("empty message")
	}
	return data[0], data[1:], nil
}

// EncodeWTMessage encodes a WebTransport protocol message:
// [4-byte big-endian length][type][payload].
// Length includes the type byte.
func EncodeWTMessage(msgType byte, payload []byte) []byte {
	bodyLen := 1 + len(payload)
	msg := make([]byte, 4+bodyLen)
	binary.BigEndian.PutUint32(msg[:4], uint32(bodyLen))
	msg[4] = msgType
	copy(msg[5:], payload)
	return msg
}

// DecodeWTMessage decodes a single WebTransport protocol message from data.
// The expected layout is [4-byte big-endian length][type][payload], where
// length includes the type byte. Returns an error if data is malformed or
// the declared length exceeds MaxMessageSize.
func DecodeWTMessage(data []byte) (msgType byte, payload []byte, err error) {
	if len(data) < 4 {
		return 0, nil, fmt.Errorf("too short for length prefix: %d bytes", len(data))
	}
	length := binary.BigEndian.Uint32(data[:4])
	if length == 0 {
		return 0, nil, fmt.Errorf("zero length message")
	}
	if uint64(length) > uint64(MaxMessageSize) {
		return 0, nil, fmt.Errorf("message length %d exceeds MaxMessageSize %d", length, MaxMessageSize)
	}
	if uint64(len(data)-4) < uint64(length) {
		return 0, nil, fmt.Errorf("truncated body: have %d bytes, need %d", len(data)-4, length)
	}
	body := data[4 : 4+length]
	return body[0], body[1:], nil
}
