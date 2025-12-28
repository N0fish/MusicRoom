package main

import (
	"errors"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"strings"
)

type EmailSender interface {
	Send(to, subject, body string) error
}

type LogEmailSender struct{}

func (LogEmailSender) Send(to, subject, body string) error {
	log.Printf("[dev email] To: %s | Subject: %s\n%s", to, subject, body)
	return nil
}

type SMTPSender struct {
	host     string
	port     string
	username string
	password string
	from     string
}

func NewSMTPSenderFromEnv() (EmailSender, error) {
	host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	port := strings.TrimSpace(os.Getenv("SMTP_PORT"))
	user := strings.TrimSpace(os.Getenv("SMTP_USER"))
	pass := strings.TrimSpace(os.Getenv("SMTP_PASS"))
	from := strings.TrimSpace(os.Getenv("SMTP_FROM"))

	if host == "" || port == "" || user == "" || pass == "" || from == "" {
		return nil, errors.New("smtp not fully configured")
	}

	return &SMTPSender{
		host:     host,
		port:     port,
		username: user,
		password: pass,
		from:     from,
	}, nil
}

func (s *SMTPSender) Send(to, subject, body string) error {
	addr := s.host + ":" + s.port

	auth := smtp.PlainAuth("", s.username, s.password, s.host)

	msg := []byte(
		fmt.Sprintf("From: %s\r\n", s.from) +
			fmt.Sprintf("To: %s\r\n", to) +
			fmt.Sprintf("Subject: %s\r\n", subject) +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/plain; charset=UTF-8\r\n" +
			"\r\n" +
			body + "\r\n",
	)

	return smtp.SendMail(addr, auth, s.from, []string{to}, msg)
}
