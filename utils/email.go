package utils

import (
	"gopkg.in/gomail.v2"
)

func SendEmail(to, subject, body, smtpHost, smtpPort, smtpUser, smtpPass string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", smtpUser)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/plain", body)

	port := 587
	// Можно парсить smtpPort, если нужно
	d := gomail.NewDialer(smtpHost, port, smtpUser, smtpPass)
	return d.DialAndSend(m)
}
