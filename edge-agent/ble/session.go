package ble

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

type Ticket struct {
	SessionID, TenantID, RegisterID, DeviceID, AppInstanceID, ActorSubject, ClientPublicKey string
	Scopes                                                                                  []string
	FencingToken                                                                            int64
	ExpiresAt                                                                               time.Time
	Nonce                                                                                   string
}
type signedTicket struct {
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}

func IssueTicket(v Ticket, key []byte) (string, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return "", e
	}
	m := hmac.New(sha256.New, key)
	m.Write(b)
	x := signedTicket{base64.RawURLEncoding.EncodeToString(b), base64.RawURLEncoding.EncodeToString(m.Sum(nil))}
	out, e := json.Marshal(x)
	return base64.RawURLEncoding.EncodeToString(out), e
}
func ParseTicket(raw string, key []byte, now time.Time) (Ticket, error) {
	var v Ticket
	b, e := base64.RawURLEncoding.DecodeString(raw)
	if e != nil {
		return v, e
	}
	var x signedTicket
	if json.Unmarshal(b, &x) != nil {
		return v, errors.New("invalid ticket")
	}
	p, e := base64.RawURLEncoding.DecodeString(x.Payload)
	if e != nil {
		return v, e
	}
	sig, e := base64.RawURLEncoding.DecodeString(x.Signature)
	if e != nil {
		return v, e
	}
	m := hmac.New(sha256.New, key)
	m.Write(p)
	if !hmac.Equal(sig, m.Sum(nil)) {
		return v, errors.New("invalid signature")
	}
	if json.Unmarshal(p, &v) != nil {
		return v, errors.New("invalid payload")
	}
	if !now.Before(v.ExpiresAt) {
		return v, errors.New("expired")
	}
	return v, nil
}

type Session struct {
	mu                 sync.Mutex
	txAEAD, rxAEAD     cipher.AEAD
	txPrefix, rxPrefix [4]byte
	rx, tx             uint64
}

func NewSession(key []byte) (*Session, error) {
	// Loopback constructor retained for deterministic simulator tests.
	k := derive(key, "loopback")
	return newDirectional(k, k, []byte("loop"), []byte("loop"))
}

// NewEndpoint derives independent keys/nonces for both BLE directions. Both
// peers use the same secret/sessionID and opposite roles ("client"/"edge").
func NewEndpoint(secret []byte, sessionID, role string) (*Session, error) {
	if sessionID == "" || (role != "client" && role != "edge") {
		return nil, errors.New("invalid endpoint")
	}
	c2e := derive(secret, sessionID+":client-to-edge")
	e2c := derive(secret, sessionID+":edge-to-client")
	if role == "client" {
		return newDirectional(c2e, e2c, []byte("c2e:"+sessionID), []byte("e2c:"+sessionID))
	}
	return newDirectional(e2c, c2e, []byte("e2c:"+sessionID), []byte("c2e:"+sessionID))
}
func derive(secret []byte, label string) []byte {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(label))
	return m.Sum(nil)
}
func newDirectional(txKey, rxKey, txLabel, rxLabel []byte) (*Session, error) {
	mk := func(key []byte) (cipher.AEAD, error) {
		b, e := aes.NewCipher(key[:32])
		if e != nil {
			return nil, e
		}
		return cipher.NewGCM(b)
	}
	tx, e := mk(txKey)
	if e != nil {
		return nil, e
	}
	rx, e := mk(rxKey)
	if e != nil {
		return nil, e
	}
	s := &Session{txAEAD: tx, rxAEAD: rx}
	a := sha256.Sum256(txLabel)
	b := sha256.Sum256(rxLabel)
	copy(s.txPrefix[:], a[:4])
	copy(s.rxPrefix[:], b[:4])
	return s, nil
}
func (s *Session) Seal(plain, aad []byte) (uint64, []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tx++
	n := nonce(s.txPrefix, s.tx)
	return s.tx, s.txAEAD.Seal(nil, n, plain, aad)
}
func (s *Session) Open(counter uint64, sealed, aad []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if counter <= s.rx {
		return nil, errors.New("replay")
	}
	plain, e := s.rxAEAD.Open(nil, nonce(s.rxPrefix, counter), sealed, aad)
	if e != nil {
		return nil, e
	}
	s.rx = counter
	return plain, nil
}
func nonce(prefix [4]byte, counter uint64) []byte {
	n := make([]byte, 12)
	copy(n, prefix[:])
	binary.BigEndian.PutUint64(n[4:], counter)
	return n
}
