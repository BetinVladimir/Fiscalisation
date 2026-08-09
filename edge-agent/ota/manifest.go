package ota

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"
)

type Manifest struct {
	Model                 string    `json:"model"`
	Version               string    `json:"version"`
	VersionCounter        uint64    `json:"version_counter"`
	MinBootloaderCounter  uint64    `json:"min_bootloader_counter"`
	HardwareRevisions     []string  `json:"hardware_revisions"`
	SHA256                string    `json:"sha256"`
	Size                  int64     `json:"size"`
	SBOMReference         string    `json:"sbom_reference"`
	RolloutRing           string    `json:"rollout_ring"`
	ExpiresAt             time.Time `json:"expires_at"`
	FiscalProtocolVersion string    `json:"fiscal_protocol_version"`
	CountryPackVersion    string    `json:"country_pack_version"`
	MinRollbackCounter    uint64    `json:"min_rollback_counter"`
	KeyID                 string    `json:"key_id"`
	Signature             string    `json:"signature,omitempty"`
}

type DeviceProfile struct {
	Model                 string
	HardwareRevision      string
	BootloaderCounter     uint64
	InstalledCounter      uint64
	RolloutRing           string
	FiscalProtocolVersion string
	CountryPackVersion    string
}

func (m Manifest) signingBytes() ([]byte, error) {
	m.Signature = ""
	return json.Marshal(m)
}

func Sign(m Manifest, privateKey ed25519.PrivateKey) (Manifest, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return Manifest{}, errors.New("invalid OTA signing key")
	}
	body, err := m.signingBytes()
	if err != nil {
		return Manifest{}, err
	}
	m.Signature = base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, body))
	return m, nil
}

func VerifyManifest(m Manifest, profile DeviceProfile, trustedKeys map[string]ed25519.PublicKey, now time.Time) error {
	key, ok := trustedKeys[m.KeyID]
	if !ok || len(key) != ed25519.PublicKeySize {
		return errors.New("OTA signing key is not trusted")
	}
	body, err := m.signingBytes()
	if err != nil {
		return err
	}
	signature, err := base64.RawURLEncoding.DecodeString(m.Signature)
	if err != nil || !ed25519.Verify(key, body, signature) {
		return errors.New("invalid OTA manifest signature")
	}
	if m.Model == "" || m.Model != profile.Model || !slices.Contains(m.HardwareRevisions, profile.HardwareRevision) {
		return errors.New("OTA target mismatch")
	}
	if m.RolloutRing == "" || m.RolloutRing != profile.RolloutRing {
		return errors.New("OTA rollout ring mismatch")
	}
	if !m.ExpiresAt.After(now) {
		return errors.New("OTA manifest expired")
	}
	if m.Version == "" || m.VersionCounter == 0 || m.VersionCounter <= profile.InstalledCounter {
		return errors.New("OTA downgrade or replay rejected")
	}
	if profile.BootloaderCounter < m.MinBootloaderCounter {
		return errors.New("bootloader below OTA minimum")
	}
	if m.MinRollbackCounter > m.VersionCounter {
		return errors.New("invalid OTA rollback floor")
	}
	if m.FiscalProtocolVersion != profile.FiscalProtocolVersion || m.CountryPackVersion != profile.CountryPackVersion {
		return errors.New("OTA fiscal protocol or country pack mismatch")
	}
	if len(m.SHA256) != sha256.Size*2 || m.Size <= 0 || m.SBOMReference == "" {
		return errors.New("incomplete OTA artifact metadata")
	}
	if _, err = hex.DecodeString(m.SHA256); err != nil {
		return errors.New("invalid OTA artifact digest")
	}
	return nil
}

func VerifyArtifact(m Manifest, artifact []byte) error {
	if int64(len(artifact)) != m.Size {
		return fmt.Errorf("OTA artifact size mismatch: got %d want %d", len(artifact), m.Size)
	}
	h := sha256.Sum256(artifact)
	if !equalDigest(hex.EncodeToString(h[:]), m.SHA256) {
		return errors.New("OTA artifact digest mismatch")
	}
	return nil
}

func equalDigest(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var different byte
	for i := range a {
		different |= a[i] ^ b[i]
	}
	return different == 0
}
