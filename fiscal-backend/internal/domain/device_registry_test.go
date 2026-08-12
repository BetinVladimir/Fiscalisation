package domain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func manufacturingInput(t *testing.T, key *ecdsa.PrivateKey, serial string) ManufacturingDeviceInput {
	t.Helper()
	jwk := map[string]any{"kty": "EC", "crv": "P-256", "x": base64.RawURLEncoding.EncodeToString(key.X.FillBytes(make([]byte, 32))), "y": base64.RawURLEncoding.EncodeToString(key.Y.FillBytes(make([]byte, 32)))}
	_, _, thumb, err := activationPublicKey(jwk)
	if err != nil {
		t.Fatal(err)
	}
	in := ManufacturingDeviceInput{Serial: serial, DevicePublicKeyJWK: jwk, HardwareRevision: "S3-CAM-R1", FirmwareVersion: "1.0.0", ManufacturingBatch: "B-2026-08", ManufacturingStationID: "station-1", FirmwareSHA256: strings64("a"), RegistrationEvidenceSHA256: strings64("b")}
	digest := sha256.Sum256(canonicalManufacturingProof(in, thumb))
	signature, _ := signP1363(key, digest[:])
	in.Proof = base64.RawURLEncoding.EncodeToString(signature)
	return in
}
func strings64(v string) string {
	out := ""
	for i := 0; i < 64; i++ {
		out += v
	}
	return out
}

func TestDeviceRegistryManufacturingAndLifecycle(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(repo, NewSimulator(true))
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	created, err := svc.RegisterManufacturedDevice(manufacturingInput(t, key, "S3-000001"))
	if err != nil {
		t.Fatal(err)
	}
	if created["state"] != "MANUFACTURED" || created["tenant_id"] != nil {
		t.Fatalf("unexpected created %#v", created)
	}
	id := created["id"].(string)
	version := created["version"].(int64)
	assigned, err := svc.TransitionPlatformDevice(id, "ASSIGNED", "tenant-a", "shipment", "operator", version)
	if err != nil {
		t.Fatal(err)
	}
	if assigned["tenant_id"] != "tenant-a" || assigned["binding_version"].(int64) != 1 {
		t.Fatalf("unexpected assignment %#v", assigned)
	}
	if _, err = svc.TransitionPlatformDevice(id, "RETIRED", "", "", "operator", assigned["version"].(int64)); err == nil {
		t.Fatal("retirement without reason accepted")
	}
	if _, err = svc.RegisterManufacturedDevice(manufacturingInput(t, key, "S3-000001")); err != nil {
		t.Fatalf("idempotent registration rejected: %v", err)
	}
}

func TestDeviceRegistryRejectsForgedProofAndSerialKeyConflict(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(repo, NewSimulator(true))
	first, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	second, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if _, err := svc.RegisterManufacturedDevice(manufacturingInput(t, first, "S3-000002")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RegisterManufacturedDevice(manufacturingInput(t, second, "S3-000002")); err == nil {
		t.Fatal("serial/key conflict accepted")
	}
	forged := manufacturingInput(t, second, "S3-000003")
	forged.Proof = "invalid"
	if _, err := svc.RegisterManufacturedDevice(forged); err == nil {
		t.Fatal("forged proof accepted")
	}
}
