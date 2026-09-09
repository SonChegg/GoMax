package transport

import (
	"bytes"
	"crypto/x509"
	"testing"

	"github.com/SonChegg/PyMax/internal/data"
)

// TestNewTCPTransportTrustsEmbeddedCA covers the trust pool NewTCPTransport
// builds for its tls.Config: it must layer the embedded root CA on top of
// the system trust store (matching pymax's transport.tcp.TCPTransport,
// which builds its SSL context with ssl.create_default_context() and then
// calls load_verify_locations() to add the embedded CA), rather than
// replacing the system trust store outright.
func TestNewTCPTransportTrustsEmbeddedCA(t *testing.T) {
	tr, err := NewTCPTransport("example.invalid", 443, "", true)
	if err != nil {
		t.Fatalf("NewTCPTransport: %v", err)
	}

	if tr.tlsConfig == nil || tr.tlsConfig.RootCAs == nil {
		t.Fatalf("expected a non-nil RootCAs pool")
	}

	// The embedded CA's subject must appear in the transport's pool.
	embeddedOnly := x509.NewCertPool()
	if !embeddedOnly.AppendCertsFromPEM(data.RootCACert) {
		t.Fatalf("embedded root CA failed to parse")
	}
	//nolint:staticcheck // Subjects is deprecated but fine for this presence check
	wantSubjects := embeddedOnly.Subjects()
	if len(wantSubjects) == 0 {
		t.Fatalf("embedded CA pool has no subjects")
	}

	//nolint:staticcheck // Subjects is deprecated but fine for this presence check
	gotSubjects := tr.tlsConfig.RootCAs.Subjects()
	found := false
	for _, s := range gotSubjects {
		if bytes.Equal(s, wantSubjects[0]) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected the transport's RootCAs pool to include the embedded CA's subject")
	}

	// The embedded sub CA (the official Android client trusts both root and
	// sub) must also be present.
	subOnly := x509.NewCertPool()
	if !subOnly.AppendCertsFromPEM(data.SubCACert) {
		t.Fatalf("embedded sub CA failed to parse")
	}
	//nolint:staticcheck // Subjects is deprecated but fine for this presence check
	wantSubSubjects := subOnly.Subjects()
	if len(wantSubSubjects) == 0 {
		t.Fatalf("embedded sub CA pool has no subjects")
	}
	foundSub := false
	for _, s := range gotSubjects {
		if bytes.Equal(s, wantSubSubjects[0]) {
			foundSub = true
			break
		}
	}
	if !foundSub {
		t.Fatalf("expected the transport's RootCAs pool to include the embedded sub CA's subject")
	}

	// If the system pool is available in this environment, the transport's
	// pool must be strictly larger than an embedded-CA-only pool, proving
	// the system CAs were layered in underneath rather than discarded.
	if sysPool, sysErr := x509.SystemCertPool(); sysErr == nil && sysPool != nil {
		//nolint:staticcheck // Subjects is deprecated but fine for this count-based check
		sysSubjects := len(sysPool.Subjects())
		if sysSubjects > 0 && len(gotSubjects) <= len(wantSubjects) {
			t.Fatalf("expected the transport's pool (%d subjects) to be larger than the embedded-only pool (%d subjects) when the system pool (%d subjects) is non-empty", len(gotSubjects), len(wantSubjects), sysSubjects)
		}
	}
}
