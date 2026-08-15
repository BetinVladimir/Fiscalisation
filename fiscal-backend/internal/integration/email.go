package integration

import (
	"context"
	"database/sql"
	"errors"
	"net/smtp"
	"strconv"
	"time"
)

type SMTPConfig struct {
	Host                 string
	Port                 int
	User, Password, From string
}

func (s *Service) RunEmailWorker(ctx context.Context, c SMTPConfig) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.deliverEmail(ctx, c)
		}
	}
}
func (s *Service) deliverEmail(ctx context.Context, c SMTPConfig) error {
	if c.Host == "" || c.From == "" || c.Port < 1 {
		return nil
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var id, to, subject, body string
	e = tx.QueryRowContext(ctx, `with picked as (select id from fiscal_email_outbox where status in ('PENDING','FAILED') and available_at<=now() order by available_at,id for update skip locked limit 1) update fiscal_email_outbox o set status='SENDING',updated_at=now() from picked where o.id=picked.id returning o.id::text,o.recipient,o.subject,o.body_text`).Scan(&id, &to, &subject, &body)
	if errors.Is(e, sql.ErrNoRows) {
		return tx.Commit()
	}
	if e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	message := []byte("From: " + c.From + "\r\nTo: " + to + "\r\nSubject: " + subject + "\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + body)
	var auth smtp.Auth
	if c.User != "" {
		auth = smtp.PlainAuth("", c.User, c.Password, c.Host)
	}
	e = smtp.SendMail(c.Host+":"+strconv.Itoa(c.Port), auth, c.From, []string{to}, message)
	if e != nil {
		_, _ = s.db.ExecContext(ctx, `update fiscal_email_outbox set status='FAILED',attempts=attempts+1,available_at=now()+least(interval '1 hour',interval '30 seconds'*(attempts+1)),last_error=$2,updated_at=now() where id=$1`, id, e.Error())
		return e
	}
	_, e = s.db.ExecContext(ctx, `update fiscal_email_outbox set status='SENT',sent_at=now(),last_error=null,updated_at=now() where id=$1`, id)
	return e
}
