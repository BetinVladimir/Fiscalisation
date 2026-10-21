package domain

import (
	"fmt"
	"strings"
	"testing"
)

func TestAuthorizationActorWireByteBound(t *testing.T) {
	for _, actor := range []string{strings.Repeat("a", 136), strings.Repeat("я", 128), strings.Repeat("я", 128) + "x", string([]byte{0xff})} {
		t.Run(fmt.Sprintf("bytes-%d", len(actor)), func(t *testing.T) {
			f := newReconciliationFixture(t)
			for _, kind := range []string{"operator", "workstation_session"} {
				id := "operator-1"
				if kind == "workstation_session" {
					id = "session-1"
				}
				r, _ := f.repo.Resource(kind, id)
				r.Data["actor_subject"] = actor
				r.Version++
				if err := f.repo.PutResource(r); err != nil {
					t.Fatal(err)
				}
			}
			_, executionErr := f.svc.IssueExecutionAuthorization("session-1", f.auth.ClientSigningPublicKeyDERBase64, "tenant-1", actor)
			valid := len(actor) <= 256 && actor != string([]byte{0xff})
			if (executionErr == nil) != valid {
				t.Fatalf("execution bytes%d: %v", len(actor), executionErr)
			}
			g := seedReconciliation(t)
			_, adminErr := g.svc.IssuePaymentReconciliation(reconciliationInput(g), "tenant-1", actor, true)
			if (adminErr == nil) != valid {
				t.Fatalf("ADMIN bytes%d: %v", len(actor), adminErr)
			}
			if !valid && len(g.repo.Resources("payment_reconciliation_authorization", "tenant-1")) != 0 {
				t.Fatal("unusable grant persisted")
			}
		})
	}
}
