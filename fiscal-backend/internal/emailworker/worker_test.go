package emailworker

import (
	"context"
	"errors"
	"strings"
	"testing"

	"fiscalisation/fiscal-backend/internal/config"
)

func TestComposeIncludesLocalizedSubjectAndCode(t *testing.T) {
	subject, body := compose(config.Config{SMTPFrom: "sender@example.com"}, message{
		To: "recipient@example.com", Code: "123456", Language: "ru",
	})
	if subject != "Код входа в BeeMiniPOS" {
		t.Fatalf("unexpected subject: %q", subject)
	}
	for _, expected := range []string{"To: recipient@example.com", "Subject: " + subject, "123456"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("message body does not contain %q", expected)
		}
	}
}

func TestMarkSentRetriesTransientJournalFailure(t *testing.T) {
	journal := &retryJournal{failures: 2}
	if err := markSent(context.Background(), journal, "message-id"); err != nil {
		t.Fatal(err)
	}
	if journal.attempts != 3 {
		t.Fatalf("got %d attempts, want 3", journal.attempts)
	}
}

type retryJournal struct {
	failures int
	attempts int
}

func (*retryJournal) BeginOutboundEmail(context.Context, string, string, string) (string, error) {
	return "message-id", nil
}

func (j *retryJournal) MarkOutboundEmailSent(context.Context, string) error {
	j.attempts++
	if j.attempts <= j.failures {
		return errors.New("temporary failure")
	}
	return nil
}

func (*retryJournal) MarkOutboundEmailFailed(context.Context, string, string) error { return nil }
