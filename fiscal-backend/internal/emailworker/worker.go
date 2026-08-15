package emailworker

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"strings"
	"time"

	"fiscalisation/fiscal-backend/internal/config"
	"github.com/rabbitmq/amqp091-go"
)

const queueName = "beeloy.email.otp"

type message struct{ To, Code, Language string }

func Run(ctx context.Context, cfg config.Config, logger *log.Logger) {
	if cfg.RabbitMQURL == "" || cfg.SMTPHost == "" {
		logger.Print("email worker disabled")
		return
	}
	for ctx.Err() == nil {
		if err := consume(ctx, cfg, logger); err != nil {
			logger.Printf("email worker: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}

func consume(ctx context.Context, cfg config.Config, logger *log.Logger) error {
	conn, err := amqp091.Dial(cfg.RabbitMQURL)
	if err != nil {
		return err
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()
	queue, err := ch.QueueDeclare(queueName, true, false, false, false, nil)
	if err != nil {
		return err
	}
	if err = ch.Qos(1, 0, false); err != nil {
		return err
	}
	deliveries, err := ch.Consume(queue.Name, "fiscal-email-worker", false, false, false, false, nil)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("delivery channel closed")
			}
			var value message
			if json.Unmarshal(delivery.Body, &value) != nil || value.To == "" || len(value.Code) != 6 {
				_ = delivery.Reject(false)
				continue
			}
			if err = send(cfg, value); err != nil {
				logger.Printf("OTP email to %s failed: %v", mask(value.To), err)
				_ = delivery.Nack(false, true)
				continue
			}
			_ = delivery.Ack(false)
		}
	}
}

func send(cfg config.Config, value message) error {
	subject, intro := "Your BeeMiniPOS sign-in code", "Your one-time sign-in code is"
	if strings.HasPrefix(strings.ToLower(value.Language), "bg") {
		subject, intro = "Код за вход в BeeMiniPOS", "Вашият еднократен код за вход е"
	}
	if strings.HasPrefix(strings.ToLower(value.Language), "ru") {
		subject, intro = "Код входа в BeeMiniPOS", "Ваш одноразовый код для входа"
	}
	body := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s: %s\r\n\r\nThe code expires in 10 minutes.\r\n", cfg.SMTPFrom, value.To, subject, intro, value.Code)
	address := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	connection, err := net.DialTimeout("tcp", address, 15*time.Second)
	if err != nil {
		return err
	}
	client, err := smtp.NewClient(connection, cfg.SMTPHost)
	if err != nil {
		_ = connection.Close()
		return err
	}
	defer client.Close()
	if cfg.SMTPMailDomain != "" {
		if err = client.Hello(cfg.SMTPMailDomain); err != nil {
			return err
		}
	}
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err = client.StartTLS(&tls.Config{ServerName: cfg.SMTPHost, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	if err = client.Auth(smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPHost)); err != nil {
		return err
	}
	if err = client.Mail(cfg.SMTPFrom); err != nil {
		return err
	}
	if err = client.Rcpt(value.To); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err = w.Write([]byte(body)); err != nil {
		_ = w.Close()
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func mask(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***"
	}
	return "***@" + parts[1]
}
