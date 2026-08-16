package integration

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"
)

func appAuthTestService(t *testing.T) *Service {
	t.Helper()
	databaseURL := os.Getenv("PG_INTEGRATION_URL")
	if databaseURL == "" {
		t.Skip("PG_INTEGRATION_URL required")
	}
	s, err := New(databaseURL, []byte("abcdef0123456789abcdef0123456789"), []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	s.SetAppSigningKey([]byte("app-auth-test-signing-key-32-bytes"))
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func seedAppMembership(t *testing.T, s *Service) (email, instance string, tenant AppTenant) {
	t.Helper()
	tenantID, err := uuid()
	if err != nil {
		t.Fatal(err)
	}
	userID, err := uuid()
	if err != nil {
		t.Fatal(err)
	}
	email = "app-auth-" + tenantID + "@example.test"
	instance, err = uuid()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(`insert into tenant_user_memberships(tenant_id,user_id,normalized_email,roles,status) values($1,$2,$3,array['ADMIN']::text[],'ACTIVE')`, tenantID, userID, email); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = s.db.Exec(`delete from app_issued_tokens where session_id in (select id from app_auth_sessions where normalized_email=$1)`, email)
		_, _ = s.db.Exec(`delete from app_auth_sessions where normalized_email=$1`, email)
		_, _ = s.db.Exec(`delete from app_auth_challenges where normalized_email=$1`, email)
		_, _ = s.db.Exec(`delete from tenant_user_memberships where tenant_id=$1 and normalized_email=$2`, tenantID, email)
	})
	return email, instance, AppTenant{TenantID: tenantID, DisplayName: tenantID, Roles: []string{"ADMIN"}}
}

func concurrentResults[T any](left, right func() (T, error)) ([]T, []error) {
	var wg sync.WaitGroup
	values := make([]T, 2)
	errs := make([]error, 2)
	for i, call := range []func() (T, error){left, right} {
		wg.Add(1)
		go func(index int, run func() (T, error)) {
			defer wg.Done()
			values[index], errs[index] = run()
		}(i, call)
	}
	wg.Wait()
	return values, errs
}

func assertOneWinner(t *testing.T, errs []error) int {
	t.Helper()
	winner, unauthorized := -1, 0
	for i, err := range errs {
		if err == nil {
			winner = i
		} else if errors.Is(err, ErrUnauthorized) {
			unauthorized++
		} else {
			t.Fatalf("unexpected concurrent error: %v", err)
		}
	}
	if winner < 0 || unauthorized != 1 {
		t.Fatalf("expected one success and one rejected replay, got %v", errs)
	}
	return winner
}

func TestAppTenantSelectionTokenIsConsumedAtomically(t *testing.T) {
	s := appAuthTestService(t)
	email, instance, tenant := seedAppMembership(t, s)
	challengeID, _ := uuid()
	selection, _ := token("app_tmp", challengeID)
	if _, err := s.db.Exec(`insert into app_auth_challenges(id,normalized_email,temporary_token_hash,otp_hash,status,expires_at,app_instance_id,verified_at) values($1,$2,$3,$4,'VERIFIED',$5,$6,now())`, challengeID, email, s.digest(selection), s.digest("123456"), time.Now().Add(time.Minute), instance); err != nil {
		t.Fatal(err)
	}
	call := func() (AppSession, error) {
		return s.SelectAppTenant(context.Background(), selection, tenant.TenantID, instance)
	}
	_, errs := concurrentResults(call, call)
	assertOneWinner(t, errs)
	var active int
	if err := s.db.QueryRow(`select count(*) from app_auth_sessions where normalized_email=$1 and status='ACTIVE'`, email).Scan(&active); err != nil || active != 1 {
		t.Fatalf("expected exactly one active session, count=%d err=%v", active, err)
	}
}

func TestAppRefreshRotationIsSingleUseAndAtomic(t *testing.T) {
	s := appAuthTestService(t)
	email, instance, tenant := seedAppMembership(t, s)
	original, err := s.createAppSession(context.Background(), email, instance, tenant)
	if err != nil {
		t.Fatal(err)
	}
	call := func() (AppSession, error) {
		return s.RotateAppSession(context.Background(), original.RefreshToken, instance, "")
	}
	values, errs := concurrentResults(call, call)
	winner := assertOneWinner(t, errs)
	if values[winner].RefreshToken == original.RefreshToken {
		t.Fatal("refresh credential was not rotated")
	}
	var active int
	if err = s.db.QueryRow(`select count(*) from app_auth_sessions where normalized_email=$1 and status='ACTIVE'`, email).Scan(&active); err != nil || active != 1 {
		t.Fatalf("expected exactly one active rotated session, count=%d err=%v", active, err)
	}

	// Failure to issue the replacement must roll back the old-session revocation.
	current := values[winner]
	s.SetAppSigningKey(nil)
	if _, err = s.RotateAppSession(context.Background(), current.RefreshToken, instance, ""); err == nil {
		t.Fatal("rotation unexpectedly succeeded without a signing key")
	}
	if err = s.db.QueryRow(`select count(*) from app_auth_sessions where normalized_email=$1 and status='ACTIVE'`, email).Scan(&active); err != nil || active != 1 {
		t.Fatalf("failed rotation revoked the current session, count=%d err=%v", active, err)
	}
}
