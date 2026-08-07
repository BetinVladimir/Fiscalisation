package ble

import (
	"bytes"
	"errors"
	"sync"
)

type FlowStatus struct {
	MessageID         [16]byte `json:"message_id"`
	HighestContiguous int      `json:"highest_contiguous"`
	MissingBitmap     uint64   `json:"missing_bitmap"`
	Complete          bool     `json:"complete"`
}

type Reassembler struct {
	mu         sync.Mutex
	messageID  [16]byte
	chunkCount uint16
	chunks     map[uint16][]byte
	active     bool
	maxChunks  uint16
}

func NewReassembler(maxChunks uint16) *Reassembler {
	if maxChunks == 0 {
		maxChunks = 1024
	}
	return &Reassembler{maxChunks: maxChunks, chunks: map[uint16][]byte{}}
}

func (r *Reassembler) Add(frame Frame, plain []byte) (FlowStatus, []byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if frame.ChunkCount == 0 || frame.ChunkCount > r.maxChunks || frame.ChunkIndex >= frame.ChunkCount {
		return FlowStatus{}, nil, errors.New("BAD_CHUNK")
	}
	if !r.active {
		r.active = true
		r.messageID = frame.MessageID
		r.chunkCount = frame.ChunkCount
		r.chunks = map[uint16][]byte{}
	}
	if r.messageID != frame.MessageID {
		return r.statusLocked(), nil, errors.New("BUSY")
	}
	if r.chunkCount != frame.ChunkCount {
		return r.statusLocked(), nil, errors.New("BAD_CHUNK")
	}
	if old, ok := r.chunks[frame.ChunkIndex]; ok {
		if !bytes.Equal(old, plain) {
			return r.statusLocked(), nil, errors.New("BAD_CHUNK")
		}
	} else {
		r.chunks[frame.ChunkIndex] = append([]byte(nil), plain...)
	}
	status := r.statusLocked()
	if !status.Complete {
		return status, nil, nil
	}
	var out bytes.Buffer
	for i := uint16(0); i < r.chunkCount; i++ {
		out.Write(r.chunks[i])
	}
	r.active = false
	r.chunks = map[uint16][]byte{}
	return status, out.Bytes(), nil
}
func (r *Reassembler) Resume(messageID [16]byte) FlowStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.active || r.messageID != messageID {
		return FlowStatus{MessageID: messageID, HighestContiguous: -1}
	}
	return r.statusLocked()
}
func (r *Reassembler) Cancel(messageID [16]byte) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.active || r.messageID != messageID {
		return false
	}
	r.active = false
	r.chunks = map[uint16][]byte{}
	return true
}
func (r *Reassembler) statusLocked() FlowStatus {
	s := FlowStatus{MessageID: r.messageID, HighestContiguous: -1}
	for i := uint16(0); i < r.chunkCount; i++ {
		if _, ok := r.chunks[i]; ok {
			if int(i) == s.HighestContiguous+1 {
				s.HighestContiguous = int(i)
			}
		} else if i < 64 {
			s.MissingBitmap |= uint64(1) << i
		}
	}
	s.Complete = len(r.chunks) == int(r.chunkCount)
	return s
}
