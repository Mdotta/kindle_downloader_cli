package email

import (
	"fmt"
	"os"
	"time"

	"kindle_cli/internal/config"

	"github.com/go-mail/mail"
)

func SendToKindle(cfg *config.Config, subject, epubPath string) error {
	if cfg.SMTPHost == "" || cfg.SMTPUsername == "" || cfg.SMTPPassword == "" || cfg.KindleEmail == "" {
		return fmt.Errorf("SMTP settings not configured")
	}

	fi, err := os.Stat(epubPath)
	fileSize := int64(0)
	if err == nil {
		fileSize = fi.Size()
	}

	// Gmail limit is 25MB; with base64 overhead ~33%, effective limit ~18MB raw
	const maxSize = 20 * 1024 * 1024 // 20MB
	if fileSize > maxSize {
		return fmt.Errorf("EPUB too large (%dMB) — Gmail limit is 25MB. Try selecting fewer chapters per volume.",
			fileSize/(1024*1024))
	}

	m := mail.NewMessage()
	m.SetHeader("From", cfg.SMTPUsername)
	m.SetHeader("To", cfg.KindleEmail)
	m.SetHeader("Subject", subject)
	m.SetBody("text/plain", "Sent by kindle_cli")

	m.Attach(epubPath,
		mail.SetHeader(map[string][]string{
			"Content-Type":              {"application/epub+zip"},
			"Content-Transfer-Encoding": {"base64"},
		}),
	)

	d := mail.NewDialer(cfg.SMTPHost, 587, cfg.SMTPUsername, cfg.SMTPPassword)
	d.StartTLSPolicy = mail.MandatoryStartTLS
	d.Timeout = 5 * time.Minute

	start := time.Now()
	err = d.DialAndSend(m)
	elapsed := time.Since(start)

	if err != nil {
		return fmt.Errorf("sending email (%.1fs, %dKB): %w", elapsed.Seconds(), fileSize/1024, err)
	}
	return nil
}
