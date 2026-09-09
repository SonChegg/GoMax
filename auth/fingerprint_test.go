package auth

import "testing"

func TestVersionCatalogResolvesRecommendedVersion(t *testing.T) {
	catalog, err := NewVersionCatalog()
	if err != nil {
		t.Fatalf("NewVersionCatalog: %v", err)
	}

	fp, err := catalog.Resolve(RecommendedAppVersion)
	if err != nil {
		t.Fatalf("Resolve(%s): %v", RecommendedAppVersion, err)
	}
	if fp.BuildNumber == 0 {
		t.Fatalf("expected a non-zero build number, got %+v", fp)
	}
	if _, ok := fp.SoMetaSHA256["arm64-v8a"]; !ok {
		t.Fatalf("expected an arm64-v8a so hash, got %+v", fp.SoMetaSHA256)
	}
}

func TestVersionCatalogUnknownVersionErrors(t *testing.T) {
	catalog, err := NewVersionCatalog()
	if err != nil {
		t.Fatalf("NewVersionCatalog: %v", err)
	}
	if _, err := catalog.Resolve("0.0.0-does-not-exist"); err == nil {
		t.Fatalf("expected an error for an unknown version")
	}
}

func TestFingerprintGeneratorProducesStableOutput(t *testing.T) {
	catalog, err := NewVersionCatalog()
	if err != nil {
		t.Fatalf("NewVersionCatalog: %v", err)
	}
	fp, err := catalog.Resolve(RecommendedAppVersion)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	gen := NewFingerprintGenerator(fp)
	out1, err := gen.GenerateFingerprint("device-123", 42, "arm64-v8a")
	if err != nil {
		t.Fatalf("GenerateFingerprint: %v", err)
	}
	out2, err := gen.GenerateFingerprint("device-123", 42, "arm64-v8a")
	if err != nil {
		t.Fatalf("GenerateFingerprint: %v", err)
	}

	if len(out1) != 96 {
		t.Fatalf("expected a 96-byte fingerprint (3x SHA-256), got %d bytes", len(out1))
	}
	if string(out1) != string(out2) {
		t.Fatalf("expected deterministic output for the same inputs")
	}

	out3, err := gen.GenerateFingerprint("device-456", 42, "arm64-v8a")
	if err != nil {
		t.Fatalf("GenerateFingerprint: %v", err)
	}
	if string(out1) == string(out3) {
		t.Fatalf("expected different device IDs to produce different fingerprints")
	}
}
