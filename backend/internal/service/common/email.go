package common

import (
	"interview-agents/internal/config"
	"fmt"
	"log"

	"gopkg.in/gomail.v2"
)

// SendEmail 发送邮件
func SendEmail(to, subject, body string) error {
	cfg := config.Global.Email
	if cfg.SMTPHost == "" {
		log.Println("[Email] SMTP Host not configured, skipping email send")
		return fmt.Errorf("SMTP配置未找到")
	}

	m := gomail.NewMessage()
	m.SetHeader("From", cfg.FromEmail)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	d := gomail.NewDialer(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass)

	if err := d.DialAndSend(m); err != nil {
		log.Printf("[Email] Failed to send email to %s: %v", to, err)
		return err
	}

	log.Printf("[Email] Email sent to %s successfully", to)
	return nil
}
