package integration

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReconciliationCapabilityIsExplicitAndSeparate(t *testing.T) {
	for _, tc := range []struct {
		scopes map[string]bool
		want   int
	}{{map[string]bool{}, 403}, {map[string]bool{ExecutionAuthorizationIssueScope: true}, 403}, {map[string]bool{PaymentReconciliationAuthorizeScope: true}, 503}} {
		h := &HTTPHandler{}
		r := httptest.NewRequest(http.MethodPost, "/integration/v1/payment-reconciliations:authorize", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.authorizePaymentReconciliation(w, r, Principal{TenantID: "tenant", SystemID: "system", CredentialID: "credential", Scopes: tc.scopes})
		if w.Code != tc.want {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
