package api

import (
	"bytes"
	"encoding/json"
	"net/http"
)

func (h *Handler) executionAuthorizations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		problem(w, 405, "METHOD_NOT_ALLOWED")
		return
	}
	body, ok := readBody(w, r)
	if !ok || h.replay(w, r, body) {
		return
	}
	var in struct {
		ServerSaleID                    string `json:"server_sale_id,omitempty"`
		WorkstationSessionID            string `json:"workstation_session_id"`
		ClientSigningPublicKeyDERBase64 string `json:"client_signing_public_key_der_base64"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&in) != nil || in.WorkstationSessionID == "" || in.ClientSigningPublicKeyDERBase64 == "" {
		problem(w, 400, "INVALID_EXECUTION_AUTHORIZATION_REQUEST")
		return
	}
	envelope, err := h.svc.IssueExecutionAuthorization(in.WorkstationSessionID, in.ClientSigningPublicKeyDERBase64, tenantID(r), actorSubject(r), in.ServerSaleID)
	if err != nil {
		problem(w, 409, "EXECUTION_AUTHORIZATION_REJECTED")
		return
	}
	h.saveReplay(w, r, body, 201, envelope)
}

func (h *Handler) operatorIdentityBinding(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		problem(w, 405, "METHOD_NOT_ALLOWED")
		return
	}
	body, ok := readBody(w, r)
	if !ok || h.replay(w, r, body) {
		return
	}
	var in struct {
		OperatorID      string `json:"operator_id"`
		ActorSubject    string `json:"actor_subject"`
		ExpectedVersion int64  `json:"expected_version"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&in) != nil {
		problem(w, 400, "INVALID_OPERATOR_IDENTITY_MAPPING")
		return
	}
	if err := h.svc.BindOperatorIdentity(in.OperatorID, in.ActorSubject, tenantID(r), actorSubject(r), in.ExpectedVersion); err != nil {
		problem(w, 409, "OPERATOR_IDENTITY_MAPPING_REJECTED")
		return
	}
	h.saveEmptyReplay(w, r, body, http.StatusNoContent)
}
