package sipclient

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildTLSConfig_None(t *testing.T) {
	cfg, err := BuildTLSConfig(false, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatalf("cfg should be non-nil even with defaults")
	}
	if cfg.InsecureSkipVerify {
		t.Errorf("InsecureSkipVerify should be false by default")
	}
	if cfg.RootCAs != nil {
		t.Errorf("RootCAs should be nil (system default)")
	}
}

func TestBuildTLSConfig_SkipVerify(t *testing.T) {
	cfg, err := BuildTLSConfig(true, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.InsecureSkipVerify {
		t.Errorf("InsecureSkipVerify should be true")
	}
}

func TestBuildTLSConfig_CAFile(t *testing.T) {
	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.pem")
	writeSelfSignedCert(t, caPath)
	cfg, err := BuildTLSConfig(false, caPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RootCAs == nil {
		t.Fatalf("RootCAs should be populated from ca file")
	}
}

func TestBuildTLSConfig_CAFileMissing(t *testing.T) {
	_, err := BuildTLSConfig(false, "/does/not/exist.pem")
	if err == nil || !strings.Contains(err.Error(), "ca-file") {
		t.Errorf("err = %v; want mention of ca-file", err)
	}
}

// writeSelfSignedCert creates a minimal PEM-encoded self-signed cert at path.
func writeSelfSignedCert(t *testing.T, path string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}()
	if err := pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		t.Fatal(err)
	}
	_ = tls.Certificate{} // ensure tls import is used
}
