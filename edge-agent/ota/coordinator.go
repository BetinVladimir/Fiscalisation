package ota

import (
	"crypto/ed25519"
	"errors"
	"time"
)

const (
	PhaseStable           = "STABLE"
	PhasePendingBoot      = "PENDING_BOOT"
	PhaseAwaitingHealth   = "AWAITING_HEALTH"
	PhaseRolledBack       = "ROLLED_BACK"
	PhaseRecoveryRequired = "RECOVERY_REQUIRED"
)

type Slot struct {
	Name           string `json:"name"`
	Version        string `json:"version"`
	VersionCounter uint64 `json:"version_counter"`
	SHA256         string `json:"sha256"`
}

type State struct {
	Revision           uint64    `json:"revision"`
	Phase              string    `json:"phase"`
	Active             Slot      `json:"active"`
	Previous           *Slot     `json:"previous,omitempty"`
	Pending            *Slot     `json:"pending,omitempty"`
	MinRollbackCounter uint64    `json:"min_rollback_counter"`
	BootAttempts       int       `json:"boot_attempts"`
	HealthDeadline     time.Time `json:"health_deadline,omitempty"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type StateStore interface {
	Load() (State, error)
	Save(*State) error
}

type Coordinator struct {
	store           StateStore
	profile         DeviceProfile
	trustedKeys     map[string]ed25519.PublicKey
	healthTimeout   time.Duration
	maxBootAttempts int
}

func NewCoordinator(store StateStore, profile DeviceProfile, trustedKeys map[string]ed25519.PublicKey, healthTimeout time.Duration, maxBootAttempts int) (*Coordinator, error) {
	if store == nil || healthTimeout <= 0 || maxBootAttempts < 1 {
		return nil, errors.New("invalid OTA coordinator configuration")
	}
	state, err := store.Load()
	if err != nil {
		return nil, err
	}
	if state.Active.VersionCounter != profile.InstalledCounter || state.Active.Name == "" {
		return nil, errors.New("OTA persisted state does not match installed firmware")
	}
	return &Coordinator{store: store, profile: profile, trustedKeys: trustedKeys, healthTimeout: healthTimeout, maxBootAttempts: maxBootAttempts}, nil
}

func (c *Coordinator) Stage(manifest Manifest, artifact []byte, now time.Time) (State, error) {
	if err := VerifyManifest(manifest, c.profile, c.trustedKeys, now); err != nil {
		return State{}, err
	}
	if err := VerifyArtifact(manifest, artifact); err != nil {
		return State{}, err
	}
	state, err := c.store.Load()
	if err != nil {
		return State{}, err
	}
	if state.Phase == PhasePendingBoot || state.Phase == PhaseAwaitingHealth {
		return State{}, errors.New("OTA update already in progress")
	}
	if manifest.VersionCounter <= state.Active.VersionCounter {
		return State{}, errors.New("OTA persisted anti-downgrade rejection")
	}
	next := "B"
	if state.Active.Name == "B" {
		next = "A"
	}
	previous := state.Active
	state.Phase = PhasePendingBoot
	state.Previous = &previous
	state.Pending = &Slot{Name: next, Version: manifest.Version, VersionCounter: manifest.VersionCounter, SHA256: manifest.SHA256}
	state.MinRollbackCounter = manifest.MinRollbackCounter
	state.BootAttempts = 0
	state.HealthDeadline = time.Time{}
	state.UpdatedAt = now.UTC()
	if err = c.store.Save(&state); err != nil {
		return State{}, err
	}
	return state, nil
}

func (c *Coordinator) AuthorizePendingBoot(slot string, manifest Manifest, artifact []byte, now time.Time) (State, error) {
	state, err := c.store.Load()
	if err != nil {
		return State{}, err
	}
	if state.Phase != PhasePendingBoot && state.Phase != PhaseAwaitingHealth {
		return State{}, errors.New("no pending OTA boot")
	}
	if state.Pending == nil || slot != state.Pending.Name {
		return State{}, errors.New("unexpected OTA boot slot")
	}
	if err = VerifyManifest(manifest, c.profile, c.trustedKeys, now); err != nil {
		return State{}, err
	}
	if err = VerifyArtifact(manifest, artifact); err != nil {
		return State{}, err
	}
	if manifest.VersionCounter != state.Pending.VersionCounter || manifest.SHA256 != state.Pending.SHA256 {
		return State{}, errors.New("OTA staged manifest changed before boot")
	}
	state.BootAttempts++
	if state.BootAttempts > c.maxBootAttempts {
		return c.rollbackOrRecover(state, now)
	}
	state.Phase = PhaseAwaitingHealth
	state.HealthDeadline = now.UTC().Add(c.healthTimeout)
	state.UpdatedAt = now.UTC()
	if err = c.store.Save(&state); err != nil {
		return State{}, err
	}
	return state, nil
}

func (c *Coordinator) ConfirmHealthy(slot string, now time.Time) (State, error) {
	state, err := c.store.Load()
	if err != nil {
		return State{}, err
	}
	if state.Phase != PhaseAwaitingHealth || state.Pending == nil || state.Pending.Name != slot {
		return State{}, errors.New("OTA health confirmation not expected")
	}
	if now.After(state.HealthDeadline) {
		return c.rollbackOrRecover(state, now)
	}
	state.Active = *state.Pending
	c.profile.InstalledCounter = state.Active.VersionCounter
	state.Pending = nil
	state.Previous = nil
	state.Phase = PhaseStable
	state.BootAttempts = 0
	state.HealthDeadline = time.Time{}
	state.UpdatedAt = now.UTC()
	if err = c.store.Save(&state); err != nil {
		return State{}, err
	}
	return state, nil
}

func (c *Coordinator) EvaluateHealth(healthy bool, now time.Time) (State, error) {
	state, err := c.store.Load()
	if err != nil {
		return State{}, err
	}
	if state.Phase != PhaseAwaitingHealth {
		return State{}, errors.New("OTA is not awaiting health")
	}
	if healthy && !now.After(state.HealthDeadline) {
		return c.ConfirmHealthy(state.Pending.Name, now)
	}
	if !healthy || now.After(state.HealthDeadline) {
		return c.rollbackOrRecover(state, now)
	}
	return state, nil
}

func (c *Coordinator) rollbackOrRecover(state State, now time.Time) (State, error) {
	if state.Previous == nil || state.Previous.VersionCounter < state.MinRollbackCounter {
		state.Phase = PhaseRecoveryRequired
		state.Pending = nil
	} else {
		state.Active = *state.Previous
		state.Phase = PhaseRolledBack
		state.Pending = nil
		state.Previous = nil
	}
	state.HealthDeadline = time.Time{}
	state.UpdatedAt = now.UTC()
	if err := c.store.Save(&state); err != nil {
		return State{}, err
	}
	return state, nil
}
