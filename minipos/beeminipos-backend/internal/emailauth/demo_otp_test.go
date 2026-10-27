package emailauth

import (
	"context"
	"testing"
)

func TestDemoOTPEnvironmentCodeAndNonDemoEarlyReject(t *testing.T) {
	for _, tc := range []struct{ name, env, prod, want string }{{"demo", "demo", "", "demo"}, {"production", "production", "", "123456"}, {"develop", "develop", "", "123456"}, {"unset", "", "", "123456"}, {"uppercase", "DEMO", "", "123456"}, {"space", "demo ", "", "123456"}, {"production-override", "demo", "true", "123456"}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ENVIRONMENT", tc.env)
			t.Setenv("PRODUCTION", tc.prod)
			if got := emailOTPCode("123456"); got != tc.want {
				t.Fatalf("code=%q want=%q", got, tc.want)
			}
			if tc.want != "demo" {
				s := &Service{}
				for _, code := range []string{"demo", " demo "} {
					if _, e := s.VerifyCode(context.Background(), "qa@example.invalid", code); e == nil {
						t.Fatal("literal demo accepted outside demo")
					}
				}
			}
		})
	}
}
