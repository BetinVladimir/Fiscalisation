package ble

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"time"
)

const BLEProtocolVersion = "2026-08-07"

type ControlMessage struct {
	Type            string         `cbor:"type" json:"type"`
	ProtocolVersion string         `cbor:"protocol_version" json:"protocol_version"`
	SessionID       string         `cbor:"session_id" json:"session_id"`
	Counter         uint64         `cbor:"counter" json:"counter"`
	MessageID       *string        `cbor:"message_id,omitempty" json:"message_id,omitempty"`
	Payload         map[string]any `cbor:"payload" json:"payload"`
}

type HandshakeServer struct {
	key       []byte
	now       func() time.Time
	ticket    Ticket
	secret    []byte
	clientN   []byte
	edgeN     []byte
	state     string
	maxChunk  int
	window    int
	transport *Session
	revoked   func(string, time.Time) bool
}

func (s *HandshakeServer) SetRevocationChecker(v func(string, time.Time) bool) { s.revoked = v }

func NewHandshakeServer(ticketSigningKey []byte, maxChunk, window int) *HandshakeServer {
	return &HandshakeServer{key: append([]byte(nil), ticketSigningKey...), now: time.Now, state: "NEW", maxChunk: maxChunk, window: window}
}

func (s *HandshakeServer) HandleHello(v ControlMessage) (ControlMessage, error) {
	if s.state != "NEW" || v.Type != "HELLO" || v.ProtocolVersion != BLEProtocolVersion || v.Counter != 0 || v.SessionID == "" {
		return ControlMessage{}, errors.New("invalid HELLO state or envelope")
	}
	rawTicket, ok := v.Payload["ticket"].(string)
	if !ok {
		return ControlMessage{}, errors.New("ticket missing")
	}
	ticket, err := ParseTicket(rawTicket, s.key, s.now().UTC())
	if err != nil || ticket.SessionID != v.SessionID || ticket.TenantID == "" || ticket.LocationID == "" || ticket.RegisterID == "" || ticket.DeviceID == "" || ticket.FiscalDeviceID == "" || !containsString(ticket.Scopes, "fiscal.execute") || (s.revoked != nil && s.revoked(ticket.SessionID, s.now().UTC())) {
		return ControlMessage{}, errors.New("ticket rejected")
	}
	clientNonce, err := decodeHandshakeBytes(v.Payload, "client_nonce", 16)
	if err != nil {
		return ControlMessage{}, err
	}
	clientPublic, err := decodeHandshakeBytes(v.Payload, "ephemeral_public_key", 32)
	if err != nil {
		return ControlMessage{}, err
	}
	ticketPublic, err := base64.RawURLEncoding.DecodeString(ticket.ClientPublicKey)
	if err != nil || len(ticketPublic) != 32 || subtle.ConstantTimeCompare(ticketPublic, clientPublic) != 1 {
		return ControlMessage{}, errors.New("client proof-of-possession key rejected")
	}
	edge, edgePublic, err := NewEphemeralKey()
	if err != nil {
		return ControlMessage{}, err
	}
	edgeNonce := make([]byte, 16)
	if _, err = rand.Read(edgeNonce); err != nil {
		return ControlMessage{}, err
	}
	ticketDigest := sha256.Sum256([]byte(rawTicket))
	secret, err := edge.DeriveSessionSecret(clientPublic, ticketDigest[:], clientNonce, edgeNonce, handshakeContext(ticket))
	if err != nil {
		return ControlMessage{}, err
	}
	s.ticket, s.secret, s.clientN, s.edgeN, s.state = ticket, secret, clientNonce, edgeNonce, "CHALLENGE_SENT"
	return ControlMessage{Type: "CHALLENGE", ProtocolVersion: BLEProtocolVersion, SessionID: ticket.SessionID, Counter: 0, Payload: map[string]any{
		"edge_nonce": base64.RawURLEncoding.EncodeToString(edgeNonce), "ephemeral_public_key": base64.RawURLEncoding.EncodeToString(edgePublic), "max_chunk": s.maxChunk, "window": s.window,
	}}, nil
}

func (s *HandshakeServer) HandleAuthProof(v ControlMessage) (ControlMessage, error) {
	if s.state != "CHALLENGE_SENT" || v.Type != "AUTH_PROOF" || v.ProtocolVersion != BLEProtocolVersion || v.SessionID != s.ticket.SessionID || v.Counter != 1 {
		return ControlMessage{}, errors.New("invalid AUTH_PROOF state or envelope")
	}
	ciphertext, err := decodeHandshakeBytes(v.Payload, "ciphertext", aes.BlockSize)
	if err != nil {
		return ControlMessage{}, err
	}
	plain, err := openHandshakeProof(s.secret, s.ticket.SessionID, ciphertext)
	if err != nil {
		return ControlMessage{}, errors.New("AUTH_PROOF rejected")
	}
	var proof map[string]any
	if err = StrictUnmarshal(plain, &proof); err != nil {
		return ControlMessage{}, errors.New("AUTH_PROOF malformed")
	}
	expected := handshakeProof(s.secret, s.ticket.SessionID, s.clientN, s.edgeN)
	provided, err := decodeHandshakeBytes(proof, "proof", sha256.Size)
	if err != nil || subtle.ConstantTimeCompare(provided, expected) != 1 {
		return ControlMessage{}, errors.New("AUTH_PROOF mismatch")
	}
	s.transport, err = NewEndpoint(s.secret, s.ticket.SessionID, "edge")
	if err != nil {
		return ControlMessage{}, err
	}
	s.state = "READY"
	return ControlMessage{Type: "READY", ProtocolVersion: BLEProtocolVersion, SessionID: s.ticket.SessionID, Counter: 1, Payload: map[string]any{"next_expected_counter": uint64(1), "max_chunk": s.maxChunk, "window": s.window}}, nil
}

func (s *HandshakeServer) Ready() bool       { return s.state == "READY" }
func (s *HandshakeServer) Session() *Session { return s.transport }

func SealHandshakeProof(secret []byte, sessionID string, clientNonce, edgeNonce []byte) ([]byte, error) {
	plain, err := CanonicalMarshal(map[string]any{"proof": base64.RawURLEncoding.EncodeToString(handshakeProof(secret, sessionID, clientNonce, edgeNonce)), "session_id": sessionID})
	if err != nil {
		return nil, err
	}
	aead, nonce, err := handshakeAEAD(secret, sessionID)
	if err != nil {
		return nil, err
	}
	return aead.Seal(nil, nonce, plain, handshakeAAD(sessionID)), nil
}

func openHandshakeProof(secret []byte, sessionID string, ciphertext []byte) ([]byte, error) {
	aead, nonce, err := handshakeAEAD(secret, sessionID)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, nonce, ciphertext, handshakeAAD(sessionID))
}

func handshakeAEAD(secret []byte, sessionID string) (cipher.AEAD, []byte, error) {
	if len(secret) != 32 || sessionID == "" {
		return nil, nil, errors.New("invalid handshake secret")
	}
	block, err := aes.NewCipher(secret)
	if err != nil {
		return nil, nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	digest := sha256.Sum256([]byte("BeeFiscal BLE auth nonce|" + sessionID))
	return aead, digest[:12], nil
}

func handshakeAAD(sessionID string) []byte {
	b, _ := CanonicalMarshal(map[string]any{"counter": uint64(1), "protocol_version": BLEProtocolVersion, "session_id": sessionID, "type": "AUTH_PROOF"})
	return b
}
func handshakeProof(secret []byte, sessionID string, clientNonce, edgeNonce []byte) []byte {
	v := append(append(append(append([]byte(nil), secret...), []byte(sessionID)...), clientNonce...), edgeNonce...)
	sum := sha256.Sum256(v)
	return sum[:]
}
func handshakeContext(v Ticket) string {
	return v.TenantID + "|" + v.LocationID + "|" + v.RegisterID + "|" + v.DeviceID + "|" + v.FiscalDeviceID + "|" + v.SessionID + "|" + BLEProtocolVersion
}
func decodeHandshakeBytes(v map[string]any, key string, minimum int) ([]byte, error) {
	raw, ok := v[key].(string)
	if !ok {
		return nil, errors.New("missing " + key)
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(b) < minimum {
		return nil, errors.New("invalid " + key)
	}
	return b, nil
}
func containsString(v []string, wanted string) bool {
	for _, item := range v {
		if item == wanted {
			return true
		}
	}
	return false
}
