package notify

import (
	"context"
	"errors"
	"fmt"

	"github.com/wneessen/go-mail"
)

type emailNotifier struct {
	host     string
	port     int
	username string
	authcode string
	from     string
	to       []string
}

func NewEmailNotifier(host string, port int, username string, authcode string, from string, to []string) (Notifier, error) {
	if host == "" || port == 0 || username == "" || authcode == "" || from == "" || len(to) == 0 {
		return nil, errors.New("invalid argument")
	}
	emailNotifier := emailNotifier{
		host:     host,
		port:     port,
		username: username,
		authcode: authcode,
		from:     from,
		to:       to,
	}
	return emailNotifier, nil
}

func (email emailNotifier) Name() string {
	return "email"
}

func (email emailNotifier) Send(ctx context.Context, message Message) error {
	msg := mail.NewMsg()
	if err := msg.From(email.from); err != nil {
		return fmt.Errorf("fail to load email from:\n%w", err)
	}
	if err := msg.To(email.to...); err != nil {
		return fmt.Errorf("fail to load email to:\n%w", err)
	}
	msg.Subject(message.Title)
	msg.SetBodyString(mail.TypeTextPlain, message.Body)
	client, err := mail.NewClient(email.host,
		mail.WithSSL(),
		mail.WithPort(email.port),
		mail.WithSMTPAuth(mail.SMTPAuthLogin),
		mail.WithUsername(email.username),
		mail.WithPassword(email.authcode))
	if err != nil {
		return fmt.Errorf("fail to new email client:\n%w", err)
	}
	return client.DialAndSendWithContext(ctx, msg)
}
