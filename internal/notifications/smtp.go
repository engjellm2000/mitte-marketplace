package notifications

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"html"
	"io"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/url"
	"regexp"
	"strings"
	"time"

	"mitte/internal/store"
)

type SMTPConfig struct {
	Address, Username, Password, From string
	// RequireTLS prevents a production relay from silently delivering a
	// transactional message over an unencrypted SMTP connection.
	RequireTLS bool
}
type SMTPMailer struct {
	config        SMTPConfig
	envelopeFrom  string
	fromHeader    string
	messageIDHost string
}

// MirroringMailer delivers through Primary and, when configured, sends a copy
// to Mirror. A mirror is intended for observability (for example Mailpit) and
// must never turn a successful transactional delivery into a retry.
type MirroringMailer struct {
	Primary       store.EmailSender
	Mirror        store.EmailSender
	OnMirrorError func(error)
}

var emailURLPattern = regexp.MustCompile(`https?://[^\s<>"']+`)

func (m MirroringMailer) Send(ctx context.Context, message store.EmailMessage) error {
	if err := m.Primary.Send(ctx, message); err != nil {
		return err
	}
	if m.Mirror != nil {
		if err := m.Mirror.Send(ctx, message); err != nil && m.OnMirrorError != nil {
			m.OnMirrorError(err)
		}
	}
	return nil
}

func NewSMTPMailer(config SMTPConfig) (*SMTPMailer, error) {
	if config.Address == "" || config.From == "" {
		return nil, fmt.Errorf("SMTP address and sender are required")
	}
	from, err := mail.ParseAddress(config.From)
	if err != nil || from.Address == "" {
		return nil, fmt.Errorf("invalid SMTP sender %q", config.From)
	}
	_, host, ok := strings.Cut(from.Address, "@")
	if !ok || host == "" {
		return nil, fmt.Errorf("SMTP sender must contain a domain")
	}
	return &SMTPMailer{
		config:        config,
		envelopeFrom:  from.Address,
		fromHeader:    from.String(),
		messageIDHost: host,
	}, nil
}

func (m *SMTPMailer) Send(ctx context.Context, message store.EmailMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	body, err := m.buildMessage(message)
	if err != nil {
		return err
	}
	sendCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	host, _, err := net.SplitHostPort(m.config.Address)
	if err != nil {
		return fmt.Errorf("invalid SMTP address %q: %w", m.config.Address, err)
	}
	conn, err := (&net.Dialer{Timeout: 15 * time.Second}).DialContext(sendCtx, "tcp", m.config.Address)
	if err != nil {
		return fmt.Errorf("connect to SMTP server: %w", err)
	}
	defer conn.Close()
	if deadline, ok := sendCtx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return fmt.Errorf("set SMTP deadline: %w", err)
		}
	}
	stopCancel := context.AfterFunc(sendCtx, func() { _ = conn.Close() })
	defer stopCancel()
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("start SMTP session: %w", err)
	}
	defer client.Close()
	startTLS, _ := client.Extension("STARTTLS")
	if m.config.RequireTLS && !startTLS {
		return fmt.Errorf("SMTP server does not advertise STARTTLS")
	}
	if startTLS {
		if err := client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("start SMTP TLS: %w", err)
		}
	}
	if m.config.Username != "" {
		if err := client.Auth(smtp.PlainAuth("", m.config.Username, m.config.Password, host)); err != nil {
			return fmt.Errorf("authenticate with SMTP server: %w", err)
		}
	}
	if err := client.Mail(m.envelopeFrom); err != nil {
		return fmt.Errorf("set SMTP sender: %w", err)
	}
	if err := client.Rcpt(message.Recipient); err != nil {
		return fmt.Errorf("set SMTP recipient: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("start SMTP message: %w", err)
	}
	if _, err := io.WriteString(writer, body); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write SMTP message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("finish SMTP message: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("finish SMTP session: %w", err)
	}
	return nil
}

func (m *SMTPMailer) buildMessage(message store.EmailMessage) (string, error) {
	if strings.ContainsAny(message.Subject, "\r\n") {
		return "", errors.New("invalid email subject")
	}
	recipient, err := mail.ParseAddress(message.Recipient)
	if err != nil || recipient.Address != message.Recipient {
		return "", fmt.Errorf("invalid email recipient %q", message.Recipient)
	}
	var headers strings.Builder
	fmt.Fprintf(&headers, "From: %s\r\n", m.fromHeader)
	fmt.Fprintf(&headers, "To: %s\r\n", recipient.String())
	fmt.Fprintf(&headers, "Subject: %s\r\n", mime.QEncoding.Encode("UTF-8", message.Subject))
	fmt.Fprintf(&headers, "Message-ID: <mitte-%d@%s>\r\n", message.ID, m.messageIDHost)
	fmt.Fprintf(&headers, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	headers.WriteString("X-Mailer: MITTE\r\n")
	headers.WriteString("MIME-Version: 1.0\r\n")
	if message.UnsubscribeURL != "" {
		unsubscribeURL, err := url.ParseRequestURI(message.UnsubscribeURL)
		isLocalHTTP := err == nil && unsubscribeURL.Scheme == "http" &&
			(strings.EqualFold(unsubscribeURL.Hostname(), "localhost") ||
				net.ParseIP(unsubscribeURL.Hostname()) != nil && net.ParseIP(unsubscribeURL.Hostname()).IsLoopback())
		if err != nil || unsubscribeURL.Scheme != "https" && !isLocalHTTP || unsubscribeURL.Host == "" ||
			unsubscribeURL.User != nil || strings.ContainsAny(message.UnsubscribeURL, "\r\n<>") {
			return "", errors.New("invalid email unsubscribe URL")
		}
		fmt.Fprintf(&headers, "List-Unsubscribe: <%s>\r\n", message.UnsubscribeURL)
		headers.WriteString("List-Unsubscribe-Post: List-Unsubscribe=One-Click\r\n")
	}
	boundary := fmt.Sprintf("=_MITTE_%d", message.ID)
	fmt.Fprintf(&headers, "Content-Type: multipart/alternative; boundary=%q\r\n\r\n", boundary)
	textPart, err := quotedPrintable(message.Body)
	if err != nil {
		return "", fmt.Errorf("encode plain-text email: %w", err)
	}
	htmlPart, err := quotedPrintable(renderEmailHTML(message.Subject, message.Preheader, message.Body))
	if err != nil {
		return "", fmt.Errorf("encode HTML email: %w", err)
	}
	body := headers.String() +
		"--" + boundary + "\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"Content-Transfer-Encoding: quoted-printable\r\n\r\n" + textPart + "\r\n" +
		"--" + boundary + "\r\n" +
		"Content-Type: text/html; charset=UTF-8\r\n" +
		"Content-Transfer-Encoding: quoted-printable\r\n\r\n" + htmlPart + "\r\n" +
		"--" + boundary + "--\r\n"
	return body, nil
}

func quotedPrintable(value string) (string, error) {
	var encoded strings.Builder
	writer := quotedprintable.NewWriter(&encoded)
	if _, err := io.WriteString(writer, value); err != nil {
		_ = writer.Close()
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}
	return encoded.String(), nil
}

func renderEmailHTML(subject, preheader, body string) string {
	var content strings.Builder
	offset := 0
	for _, match := range emailURLPattern.FindAllStringIndex(body, -1) {
		start, end := match[0], match[1]
		rawURL := body[start:end]
		cleanURL := strings.TrimRight(rawURL, ".,!?;:)]}")
		if cleanURL == "" {
			continue
		}
		parsedURL, err := url.ParseRequestURI(cleanURL)
		if err != nil || (parsedURL.Scheme != "https" && parsedURL.Scheme != "http") ||
			parsedURL.Host == "" || parsedURL.User != nil {
			continue
		}
		content.WriteString(html.EscapeString(body[offset:start]))
		escapedURL := html.EscapeString(cleanURL)
		fmt.Fprintf(&content, `<a href="%s" style="color:#193d68;text-decoration:underline;overflow-wrap:anywhere">%s</a>`,
			escapedURL, escapedURL)
		content.WriteString(html.EscapeString(rawURL[len(cleanURL):]))
		offset = end
	}
	content.WriteString(html.EscapeString(body[offset:]))
	message := strings.ReplaceAll(content.String(), "\n", "<br>\n")
	return `<!doctype html><html lang="de"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head>` +
		`<body style="margin:0;background:#f5f6f7;color:#142e50;font-family:Arial,Helvetica,sans-serif">` +
		`<div style="display:none;max-height:0;overflow:hidden;opacity:0;color:transparent">` +
		html.EscapeString(preheader) + `</div>` +
		`<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background:#f5f6f7;padding:28px 12px">` +
		`<tr><td align="center"><table role="presentation" width="600" cellspacing="0" cellpadding="0" style="width:100%;max-width:600px;background:#fff;border:1px solid #e1e5e9">` +
		`<tr><td style="background:#193d68;border-bottom:4px solid #ee5b32;padding:22px 28px;color:#fff">` +
		`<div style="font-size:23px;font-weight:800;letter-spacing:2px">MITTE</div>` +
		`<div style="margin-top:3px;font-size:10px;letter-spacing:1.3px">ALLES FÜR JEDEN TAG</div></td></tr>` +
		`<tr><td style="padding:30px 28px 24px"><h1 style="margin:0 0 22px;color:#142e50;font-size:24px;line-height:1.25">` +
		html.EscapeString(subject) + `</h1><div style="font-size:15px;line-height:1.7;color:#142e50;overflow-wrap:anywhere">` +
		message + `</div></td></tr>` +
		`<tr><td style="border-top:1px solid #e1e5e9;padding:18px 28px;color:#5f6b7b;font-size:12px;line-height:1.6">` +
		`Viele Grüße,<br><strong style="color:#142e50">Dein MITTE-Team</strong><br>` +
		`<span>MITTE · Alles für jeden Tag</span></td></tr>` +
		`</table></td></tr></table></body></html>`
}
