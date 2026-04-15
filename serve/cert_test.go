//go:build !js

package serve

import (
	"crypto/sha256"
	"testing"
)

func TestGenerateSelfSignedCert(t *testing.T) {
	info, err := GenerateSelfSignedCert("localhost")
	if err != nil {
		t.Fatal(err)
	}
	if len(info.DER) == 0 {
		t.Error("DER is empty")
	}
	if info.Hash == (sha256.Sum256(nil)) {
		t.Error("Hash is zero")
	}
	if len(info.Certificate.Certificate) == 0 {
		t.Error("no certificate chain loaded")
	}
}
