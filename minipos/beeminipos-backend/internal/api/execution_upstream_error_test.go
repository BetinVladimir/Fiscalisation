package api

import (
	"errors"
	"fiscalisation/beeminipos-backend/internal/fiscalintegration"
	"net/http/httptest"
	"testing"
)

func TestExecutionUpstreamStatusPreservesDenialAndAvailability(t *testing.T) {
	for _, status := range []int{401, 403, 400, 409, 500, 503} {
		w := httptest.NewRecorder()
		writeExecutionUpstreamError(w, &fiscalintegration.HTTPStatusError{StatusCode: status, Body: "upstream"})
		expected := 409
		if status == 401 || status == 403 {
			expected = status
		} else if status >= 500 {
			expected = 503
		}
		if w.Code != expected {
			t.Fatal(status, w.Code)
		}
	}
	w := httptest.NewRecorder()
	writeExecutionUpstreamError(w, errors.New("network unavailable"))
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
}
