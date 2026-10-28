package integration

import (
	"context"
	"testing"
)

func TestDemoOTPEnvironmentCodeAndNonDemoEarlyReject(t *testing.T) {
	for _, tc := range []struct{ name, env, prod, want string }{{"demo", "demo", "", "demo"}, {"production", "production", "", "123456"}, {"develop", "develop", "", "123456"}, {"unset", "", "", "123456"}, {"uppercase", "DEMO", "", "123456"}, {"space", " demo", "", "123456"}, {"production-override", "demo", "true", "123456"}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ENVIRONMENT", tc.env)
			t.Setenv("PRODUCTION", tc.prod)
			if got := appOTPCode("123456"); got != tc.want {
				t.Fatalf("code=%q want=%q", got, tc.want)
			}
			if tc.want != "demo" {
				s := &Service{}
				if _, e := s.VerifyAppChallenge(context.Background(), "app_tmp_fixture.secret", "demo", "instance"); e != ErrUnauthorized {
					t.Fatal("literal demo must be rejected before DB")
				}
			}
		})
	}
}
func TestDemoOTPStillRequiresTemporaryToken(t *testing.T) {
	t.Setenv("ENVIRONMENT", "demo")
	t.Setenv("PRODUCTION", "")
	s := &Service{}
	if _, e := s.VerifyAppChallenge(context.Background(), "invalid", "demo", "instance"); e != ErrUnauthorized {
		t.Fatal("demo bypassed temporary token")
	}
}
