package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"fiscalisation/edge-agent/journal"
)

type Uploader struct {
	journal   *journal.Journal
	verifier  *Verifier
	edgeID    string
	deviceID  string
	key       []byte
	endpoint  string
	authToken string
	client    *http.Client
}

func NewUploader(j *journal.Journal, edgeID, deviceID string, key []byte, endpoint, authToken string) (*Uploader, error) {
	u, err := url.Parse(endpoint)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || !strings.HasSuffix(u.Path, "/public/v1/edge-sync/batches") || len(key) < 16 || edgeID == "" || deviceID == "" {
		return nil, errors.New("invalid Edge sync uploader configuration")
	}
	return &Uploader{journal: j, verifier: NewVerifier(edgeID, key, j), edgeID: edgeID, deviceID: deviceID, key: append([]byte(nil), key...), endpoint: endpoint, authToken: authToken, client: &http.Client{Timeout: 15 * time.Second}}, nil
}

func (u *Uploader) UploadOnce(ctx context.Context, limit int) (bool, error) {
	batch, err := BuildNextBatch(u.journal, u.edgeID, u.deviceID, limit, u.key)
	if err != nil {
		if err.Error() == "no unacknowledged events" {
			return false, nil
		}
		return false, err
	}
	if err = u.verifier.Expect(batch); err != nil {
		return false, err
	}
	if err = u.journal.SetSyncPending(u.edgeID, batch.FirstSeq, batch.LastSeq, batch.BatchSHA256); err != nil {
		return false, err
	}
	body, err := json.Marshal(batch)
	if err != nil {
		return false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.endpoint, bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Version", "2026-08-07")
	req.Header.Set("Idempotency-Key", "edge-sync-"+batch.BatchSHA256)
	if u.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+u.authToken)
	}
	resp, err := u.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return false, errors.New("Edge sync upload rejected")
	}
	var ack Ack
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&ack); err != nil {
		return false, errors.New("invalid Edge sync acknowledgement")
	}
	if err = u.verifier.Apply(ack); err != nil {
		return false, err
	}
	return true, nil
}
