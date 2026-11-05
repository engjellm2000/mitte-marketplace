package notifications

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"

	"mitte/internal/store"
)

func TestNewSMTPMailerParsesDisplayNameForEnvelopeAndHeaders(t *testing.T) {
	mailer, err := NewSMTPMailer(SMTPConfig{Address: "smtp.example.test:587", From: "Mitte <shop@example.test>"})
	if err != nil {
		t.Fatalf("NewSMTPMailer returned an error: %v", err)
	}
	if mailer.envelopeFrom != "shop@example.test" || mailer.fromHeader != "\"Mitte\" <shop@example.test>" || mailer.messageIDHost != "example.test" {
		t.Fatalf("mailer sender fields = %#v", mailer)
	}
}

func TestSMTPMailerRejectsInvalidAddressesBeforeDialing(t *testing.T) {
	mailer, err := NewSMTPMailer(SMTPConfig{Address: "smtp.example.test:587", From: "shop@example.test"})
	if err != nil {
		t.Fatalf("NewSMTPMailer returned an error: %v", err)
	}
	err = mailer.Send(context.Background(), store.EmailMessage{Recipient: "Customer <customer@example.test>", Subject: "Bestellung bestätigt"})
	if err == nil || !strings.Contains(err.Error(), "invalid email recipient") {
		t.Fatalf("Send error = %v, want invalid recipient error", err)
	}
}

func TestSMTPMailerRejectsUnsafeUnsubscribeURLBeforeDialing(t *testing.T) {
	mailer, err := NewSMTPMailer(SMTPConfig{Address: "smtp.example.test:587", From: "shop@example.test"})
	if err != nil {
		t.Fatalf("NewSMTPMailer returned an error: %v", err)
	}
	err = mailer.Send(context.Background(), store.EmailMessage{
		Recipient: "customer@example.test", Subject: "Neuigkeiten", UnsubscribeURL: "https://shop.example.test/\r\nBcc:other@example.test",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid email unsubscribe URL") {
		t.Fatalf("Send error = %v, want invalid unsubscribe URL error", err)
	}
}

func TestSMTPMailerBuildsBrandedMultipartMessage(t *testing.T) {
	mailer, err := NewSMTPMailer(SMTPConfig{Address: "smtp.example.test:587", From: "MITTE <shop@example.test>"})
	if err != nil {
		t.Fatalf("NewSMTPMailer returned an error: %v", err)
	}
	message, err := mailer.buildMessage(store.EmailMessage{
		ID: 42, Recipient: "customer@example.test", Subject: "Neuigkeiten aus MITTE",
		Preheader: "Ein kurzer Überblick", Body: "Entdecke unsere Neuigkeiten.",
		UnsubscribeURL: "https://shop.example.test/api/email/unsubscribe?token=abc",
	})
	if err != nil {
		t.Fatalf("buildMessage() error = %v", err)
	}
	for _, expected := range []string{
		"Content-Type: multipart/alternative;",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Type: text/html; charset=UTF-8",
		"List-Unsubscribe: <https://shop.example.test/api/email/unsubscribe?token=abc>",
		"List-Unsubscribe-Post: List-Unsubscribe=One-Click",
		"Message-ID: <mitte-42@example.test>",
		"MITTE",
	} {
		if !strings.Contains(message, expected) {
			t.Errorf("buildMessage() missing %q", expected)
		}
	}
}

func TestRenderEmailHTMLUsesBrandAndEscapesCustomerContent(t *testing.T) {
	result := renderEmailHTML("Neuigkeiten & Angebote", "Ein kurzer Vorschautext", "Hallo <script>alert(1)</script>\nhttps://shop.example.test/angebote?a=1&b=2")
	for _, expected := range []string{
		"MITTE",
		"Neuigkeiten &amp; Angebote",
		"Ein kurzer Vorschautext",
		"&lt;script&gt;alert(1)&lt;/script&gt;",
		`href="https://shop.example.test/angebote?a=1&amp;b=2"`,
	} {
		if !strings.Contains(result, expected) {
			t.Errorf("renderEmailHTML() missing %q", expected)
		}
	}
	if strings.Contains(result, "<script>") {
		t.Fatal("renderEmailHTML() emitted unescaped customer HTML")
	}
}

func TestSMTPMailerRequiresSTARTTLSWhenConfigured(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		_, _ = connection.Write([]byte("220 smtp.example.test ESMTP\r\n"))
		reader := bufio.NewReader(connection)
		if _, err := reader.ReadString('\n'); err == nil {
			// Deliberately omit STARTTLS: this simulates a downgrade-capable relay.
			_, _ = connection.Write([]byte("250 smtp.example.test\r\n"))
		}
	}()

	mailer, err := NewSMTPMailer(SMTPConfig{Address: listener.Addr().String(), From: "shop@example.test", RequireTLS: true})
	if err != nil {
		t.Fatalf("NewSMTPMailer returned an error: %v", err)
	}
	err = mailer.Send(context.Background(), store.EmailMessage{Recipient: "customer@example.test", Subject: "Bestellung bestätigt"})
	if err == nil || !strings.Contains(err.Error(), "does not advertise STARTTLS") {
		t.Fatalf("Send error = %v, want STARTTLS rejection", err)
	}
	<-done
}
