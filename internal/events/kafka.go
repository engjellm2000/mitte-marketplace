package events

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"mitte/internal/store"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
)

// KafkaPublisher uses the order ID as the message key, so Kafka assigns every
// event for an order to the same partition and consumers see that key in order.
type KafkaPublisher struct {
	writer *kafka.Writer
}

type KafkaSecurityConfig struct {
	TLSEnabled    bool
	TLSCACert     string
	SASLMechanism string
	SASLUsername  string
	SASLPassword  string
}

const OrderEventSchemaVersion = 1

// OrderEventEnvelope is the stable wire contract for downstream consumers.
// The outbox ID is the idempotency key; new incompatible fields require a new
// schema version rather than silently changing consumer behaviour.
type OrderEventEnvelope struct {
	SchemaVersion int             `json:"schema_version"`
	EventID       int64           `json:"event_id"`
	EventType     string          `json:"event_type"`
	OrderID       string          `json:"order_id"`
	OccurredAt    time.Time       `json:"occurred_at"`
	Data          json.RawMessage `json:"data"`
}

func NewKafkaPublisher(brokers []string, topic string, security KafkaSecurityConfig) (*KafkaPublisher, error) {
	transport, err := newKafkaTransport(security)
	if err != nil {
		return nil, err
	}
	return &KafkaPublisher{writer: &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.Hash{},
		RequiredAcks: kafka.RequireAll,
		Async:        false,
		BatchTimeout: 10 * time.Millisecond,
		Transport:    transport,
	}}, nil
}

func newKafkaTransport(security KafkaSecurityConfig) (*kafka.Transport, error) {
	transport := &kafka.Transport{}
	if security.TLSEnabled {
		tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
		if strings.TrimSpace(security.TLSCACert) != "" {
			roots, err := x509.SystemCertPool()
			if err != nil {
				return nil, fmt.Errorf("load system certificate pool for Kafka TLS: %w", err)
			}
			if roots == nil {
				roots = x509.NewCertPool()
			}
			if !roots.AppendCertsFromPEM([]byte(security.TLSCACert)) {
				return nil, fmt.Errorf("KAFKA_TLS_CA_CERT contains no valid PEM certificates")
			}
			tlsConfig.RootCAs = roots
		}
		transport.TLS = tlsConfig
	}

	var mechanism sasl.Mechanism
	switch security.SASLMechanism {
	case "":
	case "plain":
		mechanism = plain.Mechanism{Username: security.SASLUsername, Password: security.SASLPassword}
	case "scram-sha-256":
		var err error
		mechanism, err = scram.Mechanism(scram.SHA256, security.SASLUsername, security.SASLPassword)
		if err != nil {
			return nil, fmt.Errorf("configure Kafka SCRAM-SHA-256: %w", err)
		}
	case "scram-sha-512":
		var err error
		mechanism, err = scram.Mechanism(scram.SHA512, security.SASLUsername, security.SASLPassword)
		if err != nil {
			return nil, fmt.Errorf("configure Kafka SCRAM-SHA-512: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported Kafka SASL mechanism %q", security.SASLMechanism)
	}
	if mechanism != nil && !security.TLSEnabled {
		return nil, fmt.Errorf("Kafka SASL authentication requires TLS")
	}
	transport.SASL = mechanism
	return transport, nil
}

func (p *KafkaPublisher) Publish(ctx context.Context, event store.OutboxEvent) error {
	value, err := marshalOrderEvent(event)
	if err != nil {
		return err
	}
	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(event.OrderID),
		Value: value,
		Headers: []kafka.Header{
			{Key: "schema_version", Value: []byte(strconv.Itoa(OrderEventSchemaVersion))},
			{Key: "event_id", Value: []byte(strconv.FormatInt(event.ID, 10))},
			{Key: "event_type", Value: []byte(event.Type)},
			{Key: "occurred_at", Value: []byte(event.OccurredAt.UTC().Format(time.RFC3339Nano))},
		},
	})
}

func marshalOrderEvent(event store.OutboxEvent) ([]byte, error) {
	return json.Marshal(OrderEventEnvelope{
		SchemaVersion: OrderEventSchemaVersion,
		EventID:       event.ID,
		EventType:     event.Type,
		OrderID:       event.OrderID,
		OccurredAt:    event.OccurredAt.UTC(),
		Data:          event.Payload,
	})
}

func (p *KafkaPublisher) Close() error { return p.writer.Close() }
