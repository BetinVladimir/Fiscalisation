package domain

import (
	"encoding/json"
	"errors"
	"sort"
	"time"
)

func (s *Service) OpenShift(register, operator, tenant string) (Shift, error) {
	if register == "" || operator == "" {
		return Shift{}, errors.New("invalid shift")
	}
	if tenant != "" && !s.registerHasActiveFiscalDevice(register, tenant) {
		return Shift{}, errors.New("fiscal device unavailable")
	}
	if tenant != "" && !s.activeOperatorResourceForTenant(operator, tenant, time.Now().UTC()) {
		return Shift{}, errors.New("operator unavailable")
	}
	return s.repo.OpenShift(register, operator, tenant)
}
func (s *Service) activeOperatorResourceForTenant(idOrCode, tenant string, at time.Time) bool {
	for _, operator := range s.repo.Resources("operator", tenant) {
		if operator.ID != idOrCode && stringField(operator.Data, "code") != idOrCode {
			continue
		}
		activeFrom, err := time.Parse(time.RFC3339, stringField(operator.Data, "active_from"))
		if err != nil || activeFrom.After(at) {
			return false
		}
		activeTo := stringField(operator.Data, "active_to")
		if activeTo == "" {
			return true
		}
		until, err := time.Parse(time.RFC3339, activeTo)
		return err == nil && at.Before(until)
	}
	return false
}
func (s *Service) CloseShift(id string) (Shift, error) {
	return s.repo.CloseShift(id)
}
func (s *Service) CloseShiftForTenant(id, tenant string) (Shift, error) {
	if _, err := s.repo.ShiftForTenant(id, tenant); err != nil {
		return Shift{}, err
	}
	return s.repo.CloseShift(id)
}
func (s *Service) GetShift(id, tenant string) (Shift, error) {
	return s.repo.ShiftForTenant(id, tenant)
}
func (s *Service) Shifts(tenant string) []Shift {
	items := s.repo.Shifts(tenant)
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	return items
}
func (s *Service) OpenWorkstationSession(workstation, operatorCode, appInstance, actor, tenant string) (WorkstationSession, error) {
	if workstation == "" || appInstance == "" || actor == "" || !bgOperatorPattern.MatchString(operatorCode) {
		_ = s.repo.AppendAudit(tenant, actor, "LOGIN_FAILED", "workstation_session", workstation, "", nil, map[string]any{"operator_code": operatorCode, "app_instance_id": appInstance, "reason": "INVALID_SESSION_INPUT"}) // best-effort: audit log; primary error already returned to caller
		return WorkstationSession{}, errors.New("invalid workstation session")
	}
	if _, err := s.activeFiscalDeviceSnapshot(workstation, tenant, time.Now().UTC()); err != nil {
		_ = s.repo.AppendAudit(tenant, actor, "LOGIN_FAILED", "workstation_session", workstation, "", nil, map[string]any{"operator_code": operatorCode, "app_instance_id": appInstance, "reason": "FISCAL_DEVICE_UNAVAILABLE"}) // best-effort: audit log; primary error already returned
		return WorkstationSession{}, err
	}
	operatorID := ""
	for _, op := range s.repo.Resources("operator", tenant) {
		if stringField(op.Data, "code") == operatorCode {
			operatorID = op.ID
			break
		}
	}
	if operatorID == "" || !s.activeOperatorForTenant(operatorCode, tenant, time.Now().UTC()) {
		_ = s.repo.AppendAudit(tenant, actor, "LOGIN_FAILED", "workstation_session", workstation, "", nil, map[string]any{"operator_code": operatorCode, "app_instance_id": appInstance, "reason": "OPERATOR_UNAVAILABLE"}) // best-effort: audit log; primary error already returned
		return WorkstationSession{}, errors.New("operator unavailable")
	}
	now := time.Now().UTC()
	sessionID, uuidErr := newUUID()
	if uuidErr != nil {
		return WorkstationSession{}, uuidErr
	}
	v := WorkstationSession{SessionID: sessionID, TenantID: tenant, WorkstationID: workstation, OperatorID: operatorID, OperatorCode: operatorCode, AppInstanceID: appInstance, ActorSubject: actor, ExpiresAt: now.Add(8 * time.Hour), CreatedAt: now}
	data := asMap(v)
	data["actor_subject"] = actor
	err := s.repo.PutResource(ResourceRecord{Kind: "workstation_session", TenantID: tenant, ID: v.SessionID, Version: 1, Data: data, CreatedAt: now, UpdatedAt: now})
	return v, err
}
func (s *Service) LogoutWorkstationSession(id, workstation, actor, tenant string) error {
	r, err := s.repo.Resource("workstation_session", id)
	if err != nil || r.TenantID != tenant || stringField(r.Data, "workstation_id") != workstation || stringField(r.Data, "actor_subject") != actor {
		return ErrNotFound
	}
	if revoked, _ := r.Data["revoked"].(bool); revoked {
		return nil
	}
	before := cloneMap(r.Data)
	r.Data["revoked"] = true
	r.Data["revoked_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	r.Version++
	r.UpdatedAt = time.Now().UTC()
	if err = s.repo.PutResource(r); err != nil {
		return err
	}
	return s.repo.AppendAudit(tenant, actor, "LOGOUT", "workstation_session", id, "", before, cloneMap(r.Data))
}
func (s *Service) WorkstationSession(id, workstation, tenant string) (WorkstationSession, error) {
	r, err := s.repo.Resource("workstation_session", id)
	if err != nil || r.TenantID != tenant {
		return WorkstationSession{}, ErrNotFound
	}
	var v WorkstationSession
	b, _ := json.Marshal(r.Data)
	revoked, _ := r.Data["revoked"].(bool)
	if json.Unmarshal(b, &v) != nil || revoked || v.WorkstationID != workstation || !time.Now().UTC().Before(v.ExpiresAt) || !s.activeOperatorForTenant(v.OperatorCode, tenant, time.Now().UTC()) {
		return WorkstationSession{}, errors.New("workstation session inactive")
	}
	return v, nil
}
