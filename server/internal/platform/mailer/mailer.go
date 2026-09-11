// Package mailer defines the transactional-email boundary. No provider is
// configured yet (brief §18 lists "transactional email provider and
// sending domain" as an owner-supplied input this repo cannot invent), so
// every call site depends on the Sender interface rather than a concrete
// vendor — swapping in a real provider later touches only this package's
// implementation, per ADR 0005 (docs/adr/0005-provider-choices.md).
package mailer

import "log/slog"

// Sender delivers transactional emails. LogSender is the only
// implementation until a provider is configured.
type Sender interface {
	SendVerificationEmail(to, token string) error
}

// LogSender never claims to have delivered anything to a real inbox — it
// logs the token so local development and testing can proceed without a
// configured provider. Do not use it past local/test environments.
type LogSender struct{}

func (LogSender) SendVerificationEmail(to, token string) error {
	slog.Info("email verification token (no email provider configured)", "to", to, "token", token)
	return nil
}
