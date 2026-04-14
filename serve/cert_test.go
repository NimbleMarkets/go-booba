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
	if info.TLSConfig == nil {
		t.Error("TLSConfig is nil")
	}
	if len(info.DER) == 0 {
		t.Error("DER is empty")
	}
	if info.Hash == (sha256.Sum256(nil)) {
		t.Error("Hash is zero")
	}
	if len(info.TLSConfig.Certificates) == 0 {
		t.Error("no certificates in TLS config")
	}
}
