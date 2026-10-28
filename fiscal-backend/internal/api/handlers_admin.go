package api

import (
	appversion "fiscalisation/fiscal-backend/internal/version"
	"net/http"
)

// installedVersions returns the currently deployed component versions.
// GET /admin/v1/installed-versions
func (h *Handler) installedVersions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		problem(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED")
		return
	}
	write(w, http.StatusOK, map[string]string{
		"backend":             appversion.Backend,
		"apk_minimum":         appversion.APKMinimum,
		"firmware_s3_minimum": appversion.FirmwareS3Minimum,
	})
}
