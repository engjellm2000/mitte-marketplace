package events

import (
	"crypto/tls"
	"encoding/json"
	"testing"
	"time"

	"mitte/internal/store"

	"github.com/segmentio/kafka-go"
)

func TestKafkaPublisherConfiguresTLSAndSCRAM(t *testing.T) {
	publisher, err := NewKafkaPublisher([]string{"kafka.example.test:9093"}, "orders", KafkaSecurityConfig{
		TLSEnabled:    true,
		SASLMechanism: "scram-sha-512",
		SASLUsername:  "relay",
		SASLPassword:  "test-password",
	})
	if err != nil {
		t.Fatalf("NewKafkaPublisher() error = %v", err)
	}
	defer publisher.Close()

	transport, ok := publisher.writer.Transport.(*kafka.Transport)
	if !ok || transport.TLS == nil || transport.TLS.MinVersion != tls.VersionTLS12 {
		t.Fatalf("Kafka transport does not enforce TLS 1.2: %#v", publisher.writer.Transport)
	}
	if transport.SASL == nil || transport.SASL.Name() != "SCRAM-SHA-512" {
		t.Fatalf("Kafka SASL mechanism = %v, want SCRAM-SHA-512", transport.SASL)
	}
}

func TestKafkaPublisherRejectsSASLWithoutTLS(t *testing.T) {
	_, err := NewKafkaPublisher([]string{"kafka.example.test:9092"}, "orders", KafkaSecurityConfig{
		SASLMechanism: "plain",
		SASLUsername:  "relay",
		SASLPassword:  "test-password",
	})
	if err == nil {
		t.Fatal("NewKafkaPublisher() error = nil, want SASL over plaintext rejection")
	}
}

func TestKafkaPublisherRejectsInvalidCustomTLSCA(t *testing.T) {
	_, err := NewKafkaPublisher([]string{"kafka.example.test:9093"}, "orders", KafkaSecurityConfig{
		TLSEnabled: true,
		TLSCACert:  "not a PEM certificate",
	})
	if err == nil {
		t.Fatal("NewKafkaPublisher() error = nil, want invalid CA rejection")
	}
}

func TestMarshalOrderEventUsesVersionedEnvelope(t *testing.T) {
	occurredAt := time.Date(2026, 10, 10, 12, 0, 0, 0, time.FixedZone("CEST", 2*60*60))
	payload, err := marshalOrderEvent(store.OutboxEvent{ID: 42, OrderID: "ord_123", Type: "payment.confirmed", OccurredAt: occurredAt, Payload: json.RawMessage(`{"total_cents":4900}`)})
	if err != nil {
		t.Fatalf("marshalOrderEvent() error = %v", err)
	}
	var envelope OrderEventEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.SchemaVersion != 1 || envelope.EventID != 42 || envelope.OrderID != "ord_123" || envelope.EventType != "payment.confirmed" {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
	if got := envelope.OccurredAt.Location(); got != time.UTC {
		t.Fatalf("OccurredAt location = %v, want UTC", got)
	}
}
