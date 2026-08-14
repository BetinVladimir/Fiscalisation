package domain

import "testing"

func TestOperatorSecurityLifecycleIsDurableAndFailClosed(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(repo, NewSimulator(true))
	svc.SetBLESigningKey("01234567890123456789012345678901")
	register, _ := prepareBLERegister(t, svc, "tenant-security")
	if _, err := svc.OpenWorkstationSession(register, "BAD", "app", "subject", "tenant-security"); err == nil {
		t.Fatal("invalid login accepted")
	}
	session, err := svc.OpenWorkstationSession(register, "A001", "app", "subject", "tenant-security")
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.LogoutWorkstationSession(session.SessionID, register, "subject", "tenant-security"); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.WorkstationSession(session.SessionID, register, "tenant-security"); err == nil {
		t.Fatal("logged-out session remained active")
	}
	counts := map[string]int{}
	for _, event := range repo.AuditEvents("tenant-security") {
		counts[event.Action]++
	}
	for _, action := range []string{"LOGIN_FAILED", "LOGIN_SUCCEEDED", "WORKSTATION_STARTED", "LOGOUT"} {
		if counts[action] != 1 {
			t.Fatalf("%s events=%d", action, counts[action])
		}
	}
}
