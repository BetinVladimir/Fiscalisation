package ota

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) (Manifest, []byte, DeviceProfile, ed25519.PublicKey, ed25519.PrivateKey, time.Time) {
	t.Helper()
	seed := sha256.Sum256([]byte("beeloy-ota-test-signing-root"))
	privateKey := ed25519.NewKeyFromSeed(seed[:])
	publicKey := privateKey.Public().(ed25519.PublicKey)
	artifact := []byte("signed firmware image with no payment data")
	digest := sha256.Sum256(artifact)
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	profile := DeviceProfile{Model: "EDGE-DP150", HardwareRevision: "rev-c", BootloaderCounter: 7, InstalledCounter: 10, RolloutRing: "pilot", FiscalProtocolVersion: "datecs-2.11.4", CountryPackVersion: "bg-eur-2026"}
	manifest := Manifest{Model: profile.Model, Version: "1.1.0", VersionCounter: 11, MinBootloaderCounter: 6, HardwareRevisions: []string{"rev-b", profile.HardwareRevision}, SHA256: hex.EncodeToString(digest[:]), Size: int64(len(artifact)), SBOMReference: "urn:uuid:11111111-1111-4111-8111-111111111111", RolloutRing: profile.RolloutRing, ExpiresAt: now.Add(24 * time.Hour), FiscalProtocolVersion: profile.FiscalProtocolVersion, CountryPackVersion: profile.CountryPackVersion, MinRollbackCounter: 10, KeyID: "release-2026"}
	manifest, err := Sign(manifest, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	return manifest, artifact, profile, publicKey, privateKey, now
}

func TestManifestAndArtifactVerifyBeforeWriteAndBoot(t *testing.T) {
	manifest, artifact, profile, publicKey, privateKey, now := fixture(t)
	keys := map[string]ed25519.PublicKey{"release-2026": publicKey}
	if err := VerifyManifest(manifest, profile, keys, now); err != nil {
		t.Fatal(err)
	}
	if err := VerifyArtifact(manifest, artifact); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		mutate func(*Manifest, *[]byte)
	}{
		{"signature", func(m *Manifest, _ *[]byte) { m.Version = "attacker" }},
		{"untrusted key", func(m *Manifest, _ *[]byte) { m.KeyID = "unknown" }},
		{"model", func(m *Manifest, _ *[]byte) { signed := *m; signed.Model = "OTHER"; *m, _ = Sign(signed, privateKey) }},
		{"hardware", func(m *Manifest, _ *[]byte) {
			signed := *m
			signed.HardwareRevisions = []string{"rev-x"}
			*m, _ = Sign(signed, privateKey)
		}},
		{"ring", func(m *Manifest, _ *[]byte) {
			signed := *m
			signed.RolloutRing = "global"
			*m, _ = Sign(signed, privateKey)
		}},
		{"expiry", func(m *Manifest, _ *[]byte) { signed := *m; signed.ExpiresAt = now; *m, _ = Sign(signed, privateKey) }},
		{"downgrade", func(m *Manifest, _ *[]byte) {
			signed := *m
			signed.VersionCounter = profile.InstalledCounter
			*m, _ = Sign(signed, privateKey)
		}},
		{"bootloader", func(m *Manifest, _ *[]byte) {
			signed := *m
			signed.MinBootloaderCounter = 8
			*m, _ = Sign(signed, privateKey)
		}},
		{"protocol", func(m *Manifest, _ *[]byte) {
			signed := *m
			signed.FiscalProtocolVersion = "changed"
			*m, _ = Sign(signed, privateKey)
		}},
		{"country", func(m *Manifest, _ *[]byte) {
			signed := *m
			signed.CountryPackVersion = "not-bg"
			*m, _ = Sign(signed, privateKey)
		}},
		{"artifact", func(_ *Manifest, a *[]byte) { *a = append([]byte{}, (*a)...); (*a)[0] ^= 1 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, a := manifest, append([]byte{}, artifact...)
			tc.mutate(&m, &a)
			manifestErr := VerifyManifest(m, profile, keys, now)
			artifactErr := VerifyArtifact(m, a)
			if manifestErr == nil && artifactErr == nil {
				t.Fatal("unsafe OTA accepted")
			}
		})
	}
}

func TestABStageHealthConfirmationAndAntiReplaySurviveRestart(t *testing.T) {
	manifest, artifact, profile, publicKey, _, now := fixture(t)
	path := filepath.Join(t.TempDir(), "ota.sqlite")
	initial := State{Phase: PhaseStable, Active: Slot{Name: "A", Version: "1.0.0", VersionCounter: 10, SHA256: strings.Repeat("0", 64)}, UpdatedAt: now}
	store, err := OpenSQLiteStore(path, initial)
	if err != nil {
		t.Fatal(err)
	}
	coordinator, err := NewCoordinator(store, profile, map[string]ed25519.PublicKey{"release-2026": publicKey}, 2*time.Minute, 2)
	if err != nil {
		t.Fatal(err)
	}
	staged, err := coordinator.Stage(manifest, artifact, now)
	if err != nil || staged.Pending == nil || staged.Pending.Name != "B" {
		t.Fatalf("stage=%+v err=%v", staged, err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = OpenSQLiteStore(path, initial)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	coordinator, err = NewCoordinator(store, profile, map[string]ed25519.PublicKey{"release-2026": publicKey}, 2*time.Minute, 2)
	if err != nil {
		t.Fatal(err)
	}
	booting, err := coordinator.AuthorizePendingBoot("B", manifest, artifact, now.Add(time.Minute))
	if err != nil || booting.Phase != PhaseAwaitingHealth {
		t.Fatalf("boot=%+v err=%v", booting, err)
	}
	stable, err := coordinator.ConfirmHealthy("B", now.Add(90*time.Second))
	if err != nil || stable.Phase != PhaseStable || stable.Active.VersionCounter != 11 || stable.Active.Name != "B" {
		t.Fatalf("stable=%+v err=%v", stable, err)
	}
	if _, err = coordinator.Stage(manifest, artifact, now.Add(2*time.Minute)); err == nil {
		t.Fatal("replayed firmware accepted")
	}
}

func TestFailedBootRollsBackOnlyToApprovedVersionOtherwiseRecovery(t *testing.T) {
	manifest, artifact, profile, publicKey, privateKey, now := fixture(t)
	for _, tc := range []struct {
		name     string
		floor    uint64
		expected string
	}{{"approved rollback", 10, PhaseRolledBack}, {"vulnerable previous", 11, PhaseRecoveryRequired}} {
		t.Run(tc.name, func(t *testing.T) {
			m := manifest
			m.MinRollbackCounter = tc.floor
			m, _ = Sign(m, privateKey)
			store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ota.sqlite"), State{Phase: PhaseStable, Active: Slot{Name: "A", Version: "1.0.0", VersionCounter: 10, SHA256: strings.Repeat("0", 64)}, UpdatedAt: now})
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			coordinator, err := NewCoordinator(store, profile, map[string]ed25519.PublicKey{"release-2026": publicKey}, time.Minute, 2)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = coordinator.Stage(m, artifact, now); err != nil {
				t.Fatal(err)
			}
			if _, err = coordinator.AuthorizePendingBoot("B", m, artifact, now.Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			state, err := coordinator.EvaluateHealth(false, now.Add(2*time.Second))
			if err != nil || state.Phase != tc.expected {
				t.Fatalf("state=%+v err=%v", state, err)
			}
			if tc.expected == PhaseRolledBack && state.Active.VersionCounter != 10 {
				t.Fatal("did not restore approved slot")
			}
		})
	}
}

func TestBootAttemptLimitAndDeadlineFailClosed(t *testing.T) {
	manifest, artifact, profile, publicKey, _, now := fixture(t)
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ota.sqlite"), State{Phase: PhaseStable, Active: Slot{Name: "A", Version: "1.0.0", VersionCounter: 10, SHA256: strings.Repeat("0", 64)}, UpdatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	coordinator, _ := NewCoordinator(store, profile, map[string]ed25519.PublicKey{"release-2026": publicKey}, time.Minute, 1)
	if _, err = coordinator.Stage(manifest, artifact, now); err != nil {
		t.Fatal(err)
	}
	if _, err = coordinator.AuthorizePendingBoot("B", manifest, artifact, now); err != nil {
		t.Fatal(err)
	}
	state, err := coordinator.ConfirmHealthy("B", now.Add(2*time.Minute))
	if err != nil || state.Phase != PhaseRolledBack {
		t.Fatalf("late health=%+v err=%v", state, err)
	}
}

func TestSQLiteStoreRejectsStaleOTAState(t *testing.T) {
	_, _, _, _, _, now := fixture(t)
	store, err := OpenSQLiteStore(filepath.Join(t.TempDir(), "ota.sqlite"), State{Phase: PhaseStable, Active: Slot{Name: "A", Version: "1.0.0", VersionCounter: 10, SHA256: strings.Repeat("0", 64)}, UpdatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	first, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	stale := first
	first.Phase = PhasePendingBoot
	if err = store.Save(&first); err != nil {
		t.Fatal(err)
	}
	stale.Phase = PhaseRecoveryRequired
	if err = store.Save(&stale); err == nil {
		t.Fatal("stale OTA supervisor overwrote newer decision")
	}
	current, err := store.Load()
	if err != nil || current.Phase != PhasePendingBoot {
		t.Fatalf("current=%+v err=%v", current, err)
	}
}
