package email_sender

import (
	"context"
	"log"
)

type LogEmailSender struct{}

func NewLogEmailSender() *LogEmailSender {
	return &LogEmailSender{}
}

func (s *LogEmailSender) SendEmail(ctx context.Context, recipient, subject, body string) error {
	log.Printf("[EMAIL] TO: %s | SUBJECT: %s | BODY: %s", recipient, subject, body)
	return nil
}
