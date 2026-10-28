package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"fiscalisation/fiscal-backend/internal/domain"
)

func (h *Handler) reports(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		problem(w, 405, "METHOD_NOT_ALLOWED")
		return
	}
	items := h.svc.Reports(tenantID(r))
	if register := r.URL.Query().Get("register_id"); register != "" {
		filtered := items[:0]
		for _, item := range items {
			if item["register_id"] == register {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	page, err := paginate(r, items)
	if err != nil {
		problem(w, 400, "INVALID_PAGINATION")
		return
	}
	write(w, 200, page)
}

func (h *Handler) report(w http.ResponseWriter, r *http.Request) {
	p := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/public/v1/reports/"), "/"), "/")
	if len(p) == 1 && r.Method == "GET" {
		v, e := h.svc.Report(p[0], tenantID(r))
		if e != nil {
			problem(w, 404, "REPORT_NOT_FOUND")
			return
		}
		write(w, 200, v)
		return
	}
	if len(p) == 3 && p[1] == "artifacts" && r.Method == "GET" {
		v, e := h.svc.ReportArtifact(p[0], p[2], tenantID(r))
		if e != nil {
			problem(w, 404, "ARTIFACT_NOT_FOUND")
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(200)
		_, _ = w.Write(v)
		return
	}
	problem(w, 404, "NOT_FOUND")
}

func (h *Handler) auditEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		problem(w, 405, "METHOD_NOT_ALLOWED")
		return
	}
	from, err := optionalTime(r.URL.Query().Get("from"))
	if err != nil {
		problem(w, 400, "INVALID_AUDIT_PERIOD")
		return
	}
	to, err := optionalTime(r.URL.Query().Get("to"))
	if err != nil || (!from.IsZero() && !to.IsZero() && to.Before(from)) {
		problem(w, 400, "INVALID_AUDIT_PERIOD")
		return
	}
	items := h.svc.AuditEvents(tenantID(r))
	filtered := items[:0]
	for _, item := range items {
		if actor := r.URL.Query().Get("actor_id"); actor != "" && item.ActorID != actor {
			continue
		}
		if action := r.URL.Query().Get("action"); action != "" && item.Action != action {
			continue
		}
		if objectType := r.URL.Query().Get("object_type"); objectType != "" && item.ObjectType != objectType {
			continue
		}
		if objectID := r.URL.Query().Get("object_id"); objectID != "" && item.ObjectID != objectID {
			continue
		}
		if unp := r.URL.Query().Get("unp"); unp != "" && item.UNP != unp {
			continue
		}
		if !from.IsZero() && item.OccurredAt.Before(from) || !to.IsZero() && item.OccurredAt.After(to) {
			continue
		}
		filtered = append(filtered, item)
	}
	page, err := paginate(r, filtered)
	if err != nil {
		problem(w, 400, "INVALID_PAGINATION")
		return
	}
	write(w, 200, page)
}

func (h *Handler) exports(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		problem(w, 405, "METHOD_NOT_ALLOWED")
		return
	}
	body, ok := readBody(w, r)
	if !ok {
		return
	}
	if h.replay(w, r, body) {
		return
	}
	var in domain.ExportRequest
	if json.Unmarshal(body, &in) != nil {
		problem(w, 400, "INVALID_JSON")
		return
	}
	v, e := h.svc.CreateExport(in, tenantID(r))
	if e != nil {
		problem(w, 422, "EXPORT_REJECTED")
		return
	}
	h.saveReplay(w, r, body, 202, v)
}

func (h *Handler) periodizedExports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		problem(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
		return
	}
	body, ok := readBody(w, r)
	if !ok || h.replay(w, r, body) {
		return
	}
	var in domain.ExportRequest
	if json.Unmarshal(body, &in) != nil {
		problem(w, http.StatusBadRequest, "INVALID_JSON")
		return
	}
	v, err := h.svc.CreatePeriodizedExport(in, tenantID(r))
	if err != nil {
		problem(w, http.StatusUnprocessableEntity, "PERIODIZED_EXPORT_REJECTED")
		return
	}
	h.saveReplay(w, r, body, http.StatusAccepted, v)
}

func (h *Handler) export(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		problem(w, 405, "METHOD_NOT_ALLOWED")
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/public/v1/exports/"), "/"), "/")
	if len(parts) == 2 && parts[0] != "" && parts[1] == "periods" {
		v, err := h.svc.ExportPeriods(parts[0], tenantID(r))
		if err != nil {
			problem(w, http.StatusNotFound, "EXPORT_NOT_FOUND")
			return
		}
		write(w, http.StatusOK, v)
		return
	}
	if len(parts) == 3 && parts[0] != "" && parts[1] == "artifacts" && parts[2] != "" {
		artifact, media, err := h.svc.ExportPeriodArtifact(parts[0], parts[2], tenantID(r))
		if err != nil {
			artifact, media, err = h.svc.ExportArtifactByID(parts[0], parts[2], tenantID(r))
			if err != nil {
				problem(w, http.StatusNotFound, "EXPORT_ARTIFACT_NOT_FOUND")
				return
			}
		}
		w.Header().Set("Content-Type", media)
		w.Header().Set("Content-Disposition", `attachment; filename="compliance-export-`+parts[2]+exportExtension(media)+`"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(artifact)
		return
	}
	if len(parts) != 1 || parts[0] == "" {
		problem(w, http.StatusNotFound, "EXPORT_NOT_FOUND")
		return
	}
	id := parts[0]
	v, e := h.svc.Export(id, tenantID(r))
	if e != nil {
		problem(w, 404, "EXPORT_NOT_FOUND")
		return
	}
	write(w, 200, v)
}

func exportExtension(media string) string {
	switch media {
	case "application/json":
		return ".json"
	case "text/csv":
		return ".csv"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return ".xlsx"
	default:
		return ".bin"
	}
}
