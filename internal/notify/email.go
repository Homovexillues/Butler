package notify

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/wneessen/go-mail"
)

type EmailNotifier struct {
	UpdatedAt time.Time `json:"updatedAt" gorm:"column:updated_at;autoUpdateTime"`
	ID        int64     `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	Host      string    `json:"host" gorm:"column:host;not null"`
	Port      int       `json:"port" gorm:"column:port;not null"`
	Username  string    `json:"username" gorm:"column:username;not null"`
	Authcode  string    `json:"authcode" gorm:"column:authcode;not null"`
	From      string    `json:"from" gorm:"column:from;not null"`
	To        []string  `json:"to" gorm:"column:to;serializer:json;not null"`
}

func NewEmailNotifier(host string, port int, username string, authcode string, from string, to []string) (Notifier, error) {
	var errs error
	required := []struct {
		name string
		ok   bool
	}{
		{"host", host != ""},
		{"port", port > 0},
		{"username", username != ""},
		{"authcode", authcode != ""},
		{"from", from != ""},
		{"to", len(to) > 0},
	}
	for _, r := range required {
		if !r.ok {
			errs = errors.Join(errs, fmt.Errorf("email: %s 不能为空", r.name))
		}
	}

	emailNotifier := EmailNotifier{
		Host:     host,
		Port:     port,
		Username: username,
		Authcode: authcode,
		From:     from,
		To:       to,
	}
	return emailNotifier, errs
}

func (email EmailNotifier) Name() string {
	return "email"
}

func (email EmailNotifier) Send(ctx context.Context, message Message) error {
	msg := mail.NewMsg()
	if err := msg.From(email.From); err != nil {
		return fmt.Errorf("fail to load email from:\n%w", err)
	}
	if err := msg.To(email.To...); err != nil {
		return fmt.Errorf("fail to load email to:\n%w", err)
	}
	msg.Subject(message.Title)
	msg.SetBodyString(mail.TypeTextPlain, message.Body)
	client, err := mail.NewClient(email.Host,
		mail.WithSSL(),
		mail.WithPort(email.Port),
		mail.WithSMTPAuth(mail.SMTPAuthLogin),
		mail.WithUsername(email.Username),
		mail.WithPassword(email.Authcode))
	if err != nil {
		return fmt.Errorf("fail to new email client:\n%w", err)
	}
	return client.DialAndSendWithContext(ctx, msg)
}
