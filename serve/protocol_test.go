package serve

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestMessageTypes(t *testing.T) {
	if MsgInput != '0' {
		t.Errorf("MsgInput = %d, want %d", MsgInput, '0')
	}
	if MsgClose != '7' {
		t.Errorf("MsgClose = %d, want %d", MsgClose, '7')
	}
	if MsgKittyKbd != '8' {
		t.Errorf("MsgKittyKbd = %d, want %d", MsgKittyKbd, '8')
	}
}

func TestEncodeWebSocketMessage(t *testing.T) {
	payload := []byte("hello")
	msg := EncodeWSMessage(MsgInput, payload)
	if msg[0] != MsgInput {
		t.Errorf("type byte = %d, want %d", msg[0], MsgInput)
	}
	if !bytes.Equal(msg[1:], payload) {
		t.Errorf("payload = %q, want %q", msg[1:], payload)
	}
}

func TestDecodeWebSocketMessage(t *testing.T) {
	raw := append([]byte{MsgOutput}, []byte("world")...)
	msgType, payload, err := DecodeWSMessage(raw)
	if err != nil {
		t.Fatal(err)
	}
	if msgType != MsgOutput {
		t.Errorf("type = %d, want %d", msgType, MsgOutput)
	}
	if !bytes.Equal(payload, []byte("world")) {
		t.Errorf("payload = %q, want %q", payload, "world")
	}
}

func TestDecodeWSMessageEmpty(t *testing.T) {
	_, _, err := DecodeWSMessage([]byte{})
	if err == nil {
		t.Error("expected error for empty message")
	}
}

func TestEncodeWTMessage(t *testing.T) {
	payload := []byte("data")
	msg := EncodeWTMessage(MsgResize, payload)
	length := binary.BigEndian.Uint32(msg[:4])
	if length != uint32(1+len(payload)) {
		t.Errorf("length = %d, want %d", length, 1+len(payload))
	}
	if msg[4] != MsgResize {
		t.Errorf("type = %d, want %d", msg[4], MsgResize)
	}
	if !bytes.Equal(msg[5:], payload) {
		t.Errorf("payload = %q, want %q", msg[5:], payload)
	}
}

func TestResizeMessageJSON(t *testing.T) {
	rm := ResizeMessage{Cols: 80, Rows: 24}
	data, err := json.Marshal(rm)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ResizeMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Cols != 80 || decoded.Rows != 24 {
		t.Errorf("decoded = %+v, want {80, 24}", decoded)
	}
}

func TestOptionsMessageJSON(t *testing.T) {
	om := OptionsMessage{ReadOnly: true}
	data, err := json.Marshal(om)
	if err != nil {
		t.Fatal(err)
	}
	var decoded OptionsMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.ReadOnly {
		t.Error("expected ReadOnly=true")
	}
}

func TestKittyKbdMessageJSON(t *testing.T) {
	km := KittyKbdMessage{Flags: 3}
	data, err := json.Marshal(km)
	if err != nil {
		t.Fatal(err)
	}
	var decoded KittyKbdMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Flags != 3 {
		t.Errorf("flags = %d, want 3", decoded.Flags)
	}
}
