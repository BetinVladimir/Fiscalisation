package integration

import (
	"strings"
	"testing"
)

func TestAppOTPEmailHasAllSelectableLanguagesAndKeepsCodeUnchanged(t *testing.T) {
	languages := []string{"bg", "hr", "cs", "da", "nl", "en", "et", "fi", "fr", "de", "el", "hu", "ga", "it", "lv", "lt", "mt", "pl", "pt", "ro", "sk", "sl", "es", "sv", "ru"}
	if len(appOTPCopies) != len(languages) {
		t.Fatalf("expected %d languages, got %d", len(languages), len(appOTPCopies))
	}
	for _, language := range languages {
		subject, body := appOTPEmail(language, "123456")
		if !strings.Contains(subject, "BeeFiscal") || !strings.Contains(body, "BeeFiscal") || !strings.HasSuffix(body, "123456") || strings.Count(body, "123456") != 1 {
			t.Errorf("invalid OTP email for %s: subject=%q body=%q", language, subject, body)
		}
	}
	englishSubject, englishBody := appOTPEmail("en", "123456")
	unknownSubject, unknownBody := appOTPEmail("unexpected", "123456")
	if unknownSubject != englishSubject || unknownBody != englishBody {
		t.Fatal("unknown language must use English")
	}
	russianSubject, russianBody := appOTPEmail("RU", "123456")
	if russianSubject == englishSubject || russianBody == englishBody {
		t.Fatal("requested Russian language was not selected")
	}
}
