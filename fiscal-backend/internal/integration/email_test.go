package integration

import (
	"io"
	"mime/quotedprintable"
	"strings"
	"testing"
)

func TestEmailMessageEncodesLocalizedOTP(t *testing.T) {
	subject, body := appOTPEmail("bg", "123456")
	message, err := emailMessage("sender@example.test", "recipient@example.test", subject, body)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitN(string(message), "\r\n\r\n", 2)
	if len(parts) != 2 || !strings.Contains(parts[0], "Subject: =?utf-8?") || !strings.Contains(parts[0], "Content-Transfer-Encoding: quoted-printable") {
		t.Fatalf("localized email headers are not MIME encoded: %q", message)
	}
	decoded, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(parts[1])))
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != body {
		t.Fatalf("decoded OTP body mismatch: %q", decoded)
	}
}
