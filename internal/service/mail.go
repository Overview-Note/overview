package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Overview-Note/overview/internal/core"
)

// ErrMailDisabled is returned when no SMTP transport is configured. Callers map
// it to a 503 so the reason never leaks as an internal error.
var ErrMailDisabled = errors.New("mail is not configured")

// ErrMailDelivery is returned when a configured transport fails to deliver a
// message. Callers map it to a 503 so an unreachable SMTP server reads as
// temporarily unavailable rather than an internal error.
var ErrMailDelivery = errors.New("mail delivery failed")

// Message is an outbound email. When HTML is set the message is sent as a
// multipart/alternative body.
type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

// MailConfig is the SMTP configuration reserved for email-based user
// management (invitations, verification, password reset).
type MailConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	StartTLS bool   `json:"starttls"`
}

// settingsStore persists string settings (implemented by the index).
type settingsStore interface {
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
}

const (
	settingMailHost     = "mail_host"
	settingMailPort     = "mail_port"
	settingMailUsername = "mail_username"
	settingMailPassword = "mail_password"
	settingMailFrom     = "mail_from"
	settingMailStartTLS = "mail_starttls"
)

// DefaultMailPort is assumed when no port is configured.
const DefaultMailPort = 587

// MailService stores and exposes the SMTP configuration.
type MailService struct {
	mu       sync.RWMutex
	store    settingsStore
	defaults MailConfig
	current  MailConfig
	sendHook func(ctx context.Context, msg Message) error
}

// NewMail constructs a MailService. store may be nil to disable persistence.
func NewMail(store settingsStore, defaults MailConfig) *MailService {
	if defaults.Port == 0 {
		defaults.Port = DefaultMailPort
	}
	return &MailService{store: store, defaults: defaults, current: defaults}
}

// Load refreshes the effective config from the settings store.
func (s *MailService) Load(ctx context.Context) {
	if s.store == nil {
		return
	}
	cfg := s.defaults
	if cfg.Port == 0 {
		cfg.Port = DefaultMailPort
	}
	if v, err := s.store.GetSetting(ctx, settingMailHost); err == nil && v != "" {
		cfg.Host = v
	}
	if v, err := s.store.GetSetting(ctx, settingMailPort); err == nil && v != "" {
		if p, perr := strconv.Atoi(v); perr == nil && p > 0 {
			cfg.Port = p
		}
	}
	if v, err := s.store.GetSetting(ctx, settingMailUsername); err == nil && v != "" {
		cfg.Username = v
	}
	if v, err := s.store.GetSetting(ctx, settingMailPassword); err == nil && v != "" {
		cfg.Password = v
	}
	if v, err := s.store.GetSetting(ctx, settingMailFrom); err == nil && v != "" {
		cfg.From = v
	}
	if v, err := s.store.GetSetting(ctx, settingMailStartTLS); err == nil && v != "" {
		cfg.StartTLS = v == "true" || v == "on" || v == "1"
	}
	s.mu.Lock()
	s.current = cfg
	s.mu.Unlock()
}

func (s *MailService) config() MailConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current
}

// Config returns the effective configuration with the password masked.
func (s *MailService) Config() MailConfig {
	cfg := s.config()
	cfg.Password = ""
	return cfg
}

// HasPassword reports whether a password is stored.
func (s *MailService) HasPassword() bool { return s.config().Password != "" }

// Enabled reports whether enough is configured to attempt sending mail.
func (s *MailService) Enabled() bool {
	cfg := s.config()
	return cfg.Host != "" && cfg.From != ""
}

// SaveConfig persists the SMTP configuration. The password is only overwritten
// when a non-empty value is provided, mirroring the AI key behaviour.
func (s *MailService) SaveConfig(ctx context.Context, cfg MailConfig) error {
	if s.store == nil {
		return core.Forbiddenf("runtime mail configuration is not available")
	}
	cfg.Host = strings.TrimSpace(cfg.Host)
	cfg.Username = strings.TrimSpace(cfg.Username)
	cfg.From = strings.TrimSpace(cfg.From)
	cfg.Password = strings.TrimSpace(cfg.Password)

	if cfg.Port == 0 {
		cfg.Port = DefaultMailPort
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return core.Invalidf("invalid SMTP port")
	}

	if err := s.store.SetSetting(ctx, settingMailHost, cfg.Host); err != nil {
		return err
	}
	if err := s.store.SetSetting(ctx, settingMailPort, strconv.Itoa(cfg.Port)); err != nil {
		return err
	}
	if err := s.store.SetSetting(ctx, settingMailUsername, cfg.Username); err != nil {
		return err
	}
	if err := s.store.SetSetting(ctx, settingMailFrom, cfg.From); err != nil {
		return err
	}
	if err := s.store.SetSetting(ctx, settingMailStartTLS, strconv.FormatBool(cfg.StartTLS)); err != nil {
		return err
	}
	if cfg.Password != "" {
		if err := s.store.SetSetting(ctx, settingMailPassword, cfg.Password); err != nil {
			return err
		}
	}
	s.Load(ctx)
	return nil
}

// SetSendHook installs a test seam that intercepts outbound messages. When set,
// Send never dials SMTP. It is intended for tests and must not be used in
// production; SetSendHook(nil) restores normal delivery.
func (s *MailService) SetSendHook(fn func(ctx context.Context, msg Message) error) {
	s.mu.Lock()
	s.sendHook = fn
	s.mu.Unlock()
}

func (s *MailService) hook() func(context.Context, Message) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sendHook
}

// Ready reports whether a message can be delivered, either through a configured
// SMTP transport or an installed test hook.
func (s *MailService) Ready() bool {
	if s.hook() != nil {
		return true
	}
	return s.Enabled()
}

// Send delivers msg over SMTP. It returns ErrMailDisabled when no transport is
// available. Recipients, subjects, bodies and tokens are never logged.
func (s *MailService) Send(ctx context.Context, msg Message) error {
	if h := s.hook(); h != nil {
		return h(ctx, msg)
	}
	cfg := s.config()
	if cfg.Host == "" || cfg.From == "" {
		return ErrMailDisabled
	}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	dialer := net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	c, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer func() { _ = c.Close() }()

	if cfg.StartTLS {
		if err := c.StartTLS(&tls.Config{ServerName: cfg.Host}); err != nil {
			return err
		}
	}
	if cfg.Username != "" {
		if err := c.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
			return err
		}
	}
	if err := c.Mail(cfg.From); err != nil {
		return err
	}
	if err := c.Rcpt(msg.To); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(buildMessage(cfg.From, msg)); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// buildMessage assembles an RFC 5322 message. A plain-text body is used unless
// an HTML alternative is provided.
func buildMessage(from string, msg Message) []byte {
	var body bytes.Buffer
	headers := textproto.MIMEHeader{}
	headers.Set("From", from)
	headers.Set("To", msg.To)
	headers.Set("Subject", mime.QEncoding.Encode("utf-8", msg.Subject))
	headers.Set("Date", time.Now().UTC().Format(time.RFC1123Z))
	headers.Set("Message-ID", newMessageID(from))
	headers.Set("MIME-Version", "1.0")

	if msg.HTML == "" {
		headers.Set("Content-Type", "text/plain; charset=utf-8")
		headers.Set("Content-Transfer-Encoding", "quoted-printable")
		writeQuotedPrintable(&body, msg.Text)
	} else {
		mw := multipart.NewWriter(&body)
		headers.Set("Content-Type", "multipart/alternative; boundary="+mw.Boundary())
		writePart(mw, "text/plain; charset=utf-8", msg.Text)
		writePart(mw, "text/html; charset=utf-8", msg.HTML)
		_ = mw.Close()
	}

	var out bytes.Buffer
	for k, values := range headers {
		for _, v := range values {
			fmt.Fprintf(&out, "%s: %s\r\n", k, v)
		}
	}
	out.WriteString("\r\n")
	out.Write(body.Bytes())
	return out.Bytes()
}

func writePart(mw *multipart.Writer, contentType, content string) {
	h := textproto.MIMEHeader{}
	h.Set("Content-Type", contentType)
	h.Set("Content-Transfer-Encoding", "quoted-printable")
	w, err := mw.CreatePart(h)
	if err != nil {
		return
	}
	writeQuotedPrintable(w, content)
}

func writeQuotedPrintable(w io.Writer, content string) {
	qp := quotedprintable.NewWriter(w)
	_, _ = qp.Write([]byte(content))
	_ = qp.Close()
}

func newMessageID(from string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("<%d@overview>", time.Now().UnixNano())
	}
	domain := "overview"
	if i := strings.LastIndexByte(from, '@'); i >= 0 && i+1 < len(from) {
		domain = from[i+1:]
	}
	return fmt.Sprintf("<%s.%d@%s>", hex.EncodeToString(buf), time.Now().UnixNano(), domain)
}

// inviteMessage builds the invitation email. The token lives in the URL
// fragment so it is not sent to the server in access logs.
func inviteMessage(to, baseURL, token string) Message {
	link := strings.TrimRight(baseURL, "/") + "/accept-invite#token=" + token
	return Message{
		To:      to,
		Subject: "You have been invited to Overview",
		Text: "Hi,\n\nYou have been invited to Overview. Accept the invitation and " +
			"choose a password using the link below:\n\n" + link + "\n\n" +
			"This link expires in 7 days. If you were not expecting it, ignore this email.\n",
		HTML: "<p>Hi,</p><p>You have been invited to Overview. Accept the invitation and " +
			"choose a password using the link below:</p><p><a href=\"" + html.EscapeString(link) +
			"\">Accept invitation</a></p><p>This link expires in 7 days. If you were not " +
			"expecting it, ignore this email.</p>",
	}
}

// resetMessage builds the password reset email.
func resetMessage(to, baseURL, token string) Message {
	link := strings.TrimRight(baseURL, "/") + "/reset-password#token=" + token
	return Message{
		To:      to,
		Subject: "Reset your Overview password",
		Text: "Hi,\n\nUse the link below to choose a new password:\n\n" + link + "\n\n" +
			"This link expires in 2 hours. If you did not request a reset, ignore this email.\n",
		HTML: "<p>Hi,</p><p>Use the link below to choose a new password:</p><p><a href=\"" +
			html.EscapeString(link) + "\">Reset password</a></p><p>This link expires in 2 hours. " +
			"If you did not request a reset, ignore this email.</p>",
	}
}

// verifyMessage builds the email verification email.
func verifyMessage(to, baseURL, token string) Message {
	link := strings.TrimRight(baseURL, "/") + "/verify-email#token=" + token
	return Message{
		To:      to,
		Subject: "Verify your Overview email",
		Text: "Hi,\n\nConfirm your email address using the link below:\n\n" + link + "\n\n" +
			"This link expires in 24 hours.\n",
		HTML: "<p>Hi,</p><p>Confirm your email address using the link below:</p><p><a href=\"" +
			html.EscapeString(link) + "\">Verify email</a></p><p>This link expires in 24 hours.</p>",
	}
}
