package ble

import (
	"encoding/binary"
	"errors"
)

const (
	FrameVersion = byte(1)
	HeaderSize   = 34
	GCMTagSize   = 16
)

type Frame struct {
	Flags      byte
	MessageID  [16]byte
	Counter    uint64
	ChunkIndex uint16
	ChunkCount uint16
	Ciphertext []byte
	Tag        [GCMTagSize]byte
}

func header(flags byte, messageID [16]byte, counter uint64, chunkIndex, chunkCount, ciphertextLength uint16) []byte {
	b := make([]byte, HeaderSize)
	b[0], b[1], b[2], b[3] = 'B', 'F', FrameVersion, flags
	copy(b[4:20], messageID[:])
	binary.BigEndian.PutUint64(b[20:28], counter)
	binary.BigEndian.PutUint16(b[28:30], chunkIndex)
	binary.BigEndian.PutUint16(b[30:32], chunkCount)
	binary.BigEndian.PutUint16(b[32:34], ciphertextLength)
	return b
}

func (s *Session) SealFrame(messageID [16]byte, chunkIndex, chunkCount uint16, flags byte, plaintext []byte) ([]byte, error) {
	if chunkCount == 0 || chunkIndex >= chunkCount || len(plaintext) > 65535 {
		return nil, errors.New("invalid chunk")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tx++
	h := header(flags, messageID, s.tx, chunkIndex, chunkCount, uint16(len(plaintext)))
	sealed := s.txAEAD.Seal(nil, nonce(s.txPrefix, s.tx), plaintext, h)
	return append(h, sealed...), nil
}

func ParseFrame(raw []byte) (Frame, error) {
	var f Frame
	if len(raw) < HeaderSize+GCMTagSize || raw[0] != 'B' || raw[1] != 'F' || raw[2] != FrameVersion {
		return f, errors.New("invalid frame header")
	}
	n := int(binary.BigEndian.Uint16(raw[32:34]))
	if len(raw) != HeaderSize+n+GCMTagSize {
		return f, errors.New("invalid frame length")
	}
	f.Flags = raw[3]
	copy(f.MessageID[:], raw[4:20])
	f.Counter = binary.BigEndian.Uint64(raw[20:28])
	f.ChunkIndex = binary.BigEndian.Uint16(raw[28:30])
	f.ChunkCount = binary.BigEndian.Uint16(raw[30:32])
	if f.ChunkCount == 0 || f.ChunkIndex >= f.ChunkCount {
		return Frame{}, errors.New("invalid chunk metadata")
	}
	f.Ciphertext = append([]byte(nil), raw[HeaderSize:HeaderSize+n]...)
	copy(f.Tag[:], raw[HeaderSize+n:])
	return f, nil
}

func (s *Session) OpenFrame(raw []byte) (Frame, []byte, error) {
	f, err := ParseFrame(raw)
	if err != nil {
		return f, nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if f.Counter <= s.rx {
		return f, nil, errors.New("replay")
	}
	sealed := append(append([]byte(nil), f.Ciphertext...), f.Tag[:]...)
	plain, err := s.rxAEAD.Open(nil, nonce(s.rxPrefix, f.Counter), sealed, raw[:HeaderSize])
	if err != nil {
		return f, nil, err
	}
	s.rx = f.Counter
	return f, plain, nil
}

func MaxChunkPlaintext(attMTU int) (int, error) {
	n := attMTU - 3 - HeaderSize - GCMTagSize
	if n < 1 {
		return 0, errors.New("ATT MTU too small")
	}
	return n, nil
}

func ChunkPlaintext(payload []byte, attMTU int) ([][]byte, error) {
	n, err := MaxChunkPlaintext(attMTU)
	if err != nil {
		return nil, err
	}
	if len(payload) == 0 {
		return [][]byte{{}}, nil
	}
	chunks := make([][]byte, 0, (len(payload)+n-1)/n)
	for len(payload) > 0 {
		cut := n
		if len(payload) < cut {
			cut = len(payload)
		}
		chunks = append(chunks, append([]byte(nil), payload[:cut]...))
		payload = payload[cut:]
	}
	if len(chunks) > 65535 {
		return nil, errors.New("message too large")
	}
	return chunks, nil
}
