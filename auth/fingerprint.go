package auth

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/SonChegg/PyMax/internal/data"
)

// ApkBuildFingerprint is the certificate/dex/so hash set for one Android
// APK build, a port of pymax's fingerprint.models.ApkBuildFingerprint.
type ApkBuildFingerprint struct {
	SignatureScheme       string            `json:"signature_scheme"`
	CertificateCount      int               `json:"certificate_count"`
	CertificateMetaSHA256 string            `json:"certificate_meta_sha256"`
	CertificateSHA256     []string          `json:"certificate_sha256"`
	DexMetaSHA256         string            `json:"dex_meta_sha256"`
	SoMetaSHA256Arm64V8a  string            `json:"so_meta_sha256_arm64_v8a"`
	SoMetaSHA256          map[string]string `json:"so_meta_sha256"`
	BuildNumber           int               `json:"build_number"`
}

// FingerprintGenerator derives the per-request "mode"/chat-cache-fingerprint
// bytes Max's mobile client sends on auth/login requests, a port of
// pymax's fingerprint.fingerprint.FingerprintGenerator.
type FingerprintGenerator struct {
	data ApkBuildFingerprint
}

// NewFingerprintGenerator builds a generator for one APK build's fingerprint.
func NewFingerprintGenerator(fp ApkBuildFingerprint) *FingerprintGenerator {
	return &FingerprintGenerator{data: fp}
}

// GenerateFingerprint derives the 96-byte fingerprint for a device/call
// pair, a port of pymax's FingerprintGenerator.generate_fingerprint.
func (g *FingerprintGenerator) GenerateFingerprint(deviceID string, callsSeed int64, arch string) ([]byte, error) {
	soHash, ok := g.data.SoMetaSHA256[arch]
	if !ok {
		return nil, fmt.Errorf("gomax: no so_meta_sha256 entry for arch %q", arch)
	}

	seedBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(seedBytes, uint64(callsSeed))
	deviceBytes := []byte(deviceID)

	h1, err := hashHexPlus(g.data.CertificateMetaSHA256, seedBytes, deviceBytes)
	if err != nil {
		return nil, err
	}
	h2, err := hashHexPlus(g.data.DexMetaSHA256, seedBytes, deviceBytes)
	if err != nil {
		return nil, err
	}
	h3, err := hashHexPlus(soHash, seedBytes, deviceBytes)
	if err != nil {
		return nil, err
	}

	out := make([]byte, 0, len(h1)+len(h2)+len(h3))
	out = append(out, h1...)
	out = append(out, h2...)
	out = append(out, h3...)
	return out, nil
}

func hashHexPlus(hexSeed string, seedBytes, deviceBytes []byte) ([]byte, error) {
	prefix, err := hex.DecodeString(hexSeed)
	if err != nil {
		return nil, fmt.Errorf("gomax: invalid fingerprint hex: %w", err)
	}
	h := sha256.New()
	h.Write(prefix)
	h.Write(seedBytes)
	h.Write(deviceBytes)
	return h.Sum(nil), nil
}

// RecommendedAppVersion is the Android client version gomax's Client uses
// by default, a port of pymax's versions.catalog.VersionCatalog.RECOMMENDED_APP_VERSION.
const RecommendedAppVersion = "26.25.0"

// VersionCatalog resolves Android client versions to their APK fingerprint,
// a port of pymax's versions.catalog.VersionCatalog (embedded catalog
// only; gomax does not implement the optional remote-catalog fetch).
type VersionCatalog struct {
	versions map[string]ApkBuildFingerprint
}

// NewVersionCatalog loads the embedded fingerprint catalog.
func NewVersionCatalog() (*VersionCatalog, error) {
	var raw map[string]ApkBuildFingerprint
	if err := json.Unmarshal(data.ApkFingerprintsJSON, &raw); err != nil {
		return nil, fmt.Errorf("gomax: parse embedded fingerprint catalog: %w", err)
	}
	return &VersionCatalog{versions: raw}, nil
}

// Add inserts or replaces a version's fingerprint in the catalog.
func (c *VersionCatalog) Add(version string, fp ApkBuildFingerprint, override bool) error {
	if _, exists := c.versions[version]; exists && !override {
		return fmt.Errorf("gomax: version %s already exists in registry", version)
	}
	c.versions[version] = fp
	return nil
}

// Resolve returns the fingerprint for version.
func (c *VersionCatalog) Resolve(version string) (ApkBuildFingerprint, error) {
	fp, ok := c.versions[version]
	if !ok {
		return ApkBuildFingerprint{}, fmt.Errorf("gomax: could not find version %s in registry", version)
	}
	return fp, nil
}
