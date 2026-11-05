package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrInvalidFulfillmentTransition = errors.New("invalid fulfillment transition")
var ErrInvalidFulfillmentHold = errors.New("invalid fulfillment hold")
var ErrInvalidWarehouseQueueFilter = errors.New("invalid warehouse queue filter")
var ErrInvalidWarehouseOrderIDs = errors.New("invalid warehouse order IDs")

const warehouseEventHistoryLimit = 100
const warehouseOrderIDsLimit = 100

type WarehouseFulfillment struct {
	OrderID, Status, AssignedTo, Carrier, TrackingNumber, TrackingURL, LabelURL string
	HoldReason, HeldBy, ResumeStatus                                            string
	PickedAt, PackedAt, HandedOverAt                                            *time.Time
	HeldAt                                                                      *time.Time
	UpdatedAt                                                                   time.Time
	FraudBlocked                                                                bool
}

type WarehouseQueueItem struct {
	WarehouseFulfillment
	CustomerName string
	TotalCents   int
	CreatedAt    time.Time
}

type WarehouseQueueStats struct {
	Active, HandedOver, Processing int
}

type WarehouseEvent struct {
	ActorEmail, EventType, Detail string
	CreatedAt                     time.Time
}

func (s *Store) WarehouseFulfillmentsByOrderIDs(ctx context.Context, orderIDs []string) ([]WarehouseFulfillment, error) {
	if len(orderIDs) > warehouseOrderIDsLimit {
		return nil, ErrInvalidWarehouseOrderIDs
	}
	if len(orderIDs) == 0 {
		return []WarehouseFulfillment{}, nil
	}
	placeholders := make([]string, len(orderIDs))
	args := make([]any, len(orderIDs))
	for index, orderID := range orderIDs {
		placeholders[index] = fmt.Sprintf("$%d", index+1)
		args[index] = orderID
	}
	query := `SELECT order_id, status, COALESCE(assigned_to,''), COALESCE(carrier,''), COALESCE(tracking_number,''), COALESCE(tracking_url,''), COALESCE(label_url,''), COALESCE(hold_reason,''), COALESCE(held_by,''), COALESCE(resume_status,''), picked_at, packed_at, handed_over_at, held_at, updated_at FROM warehouse_fulfillments WHERE order_id IN (` + strings.Join(placeholders, ",") + `) ORDER BY updated_at ASC, order_id ASC`
	return scanWarehouseQueue(ctx, s.db, query, args...)
}

func (s *Store) WarehouseQueuePage(ctx context.Context, status, search string, limit, offset int) ([]WarehouseQueueItem, int, WarehouseQueueStats, error) {
	where := []string{}
	args := []any{}
	if status != "" && status != "all" {
		switch status {
		case "active":
			where = append(where, "f.status NOT IN ('handed_over','cancelled')")
		case "ready_to_pick", "picking", "packed", "label_ready", "held", "handed_over", "cancelled":
			args = append(args, status)
			where = append(where, fmt.Sprintf("f.status = $%d", len(args)))
		default:
			return nil, 0, WarehouseQueueStats{}, ErrInvalidWarehouseQueueFilter
		}
	}
	if search != "" {
		args = append(args, "%"+search+"%")
		searchParameter := fmt.Sprintf("$%d", len(args))
		where = append(where, "(f.order_id ILIKE "+searchParameter+" OR o.customer_name ILIKE "+searchParameter+" OR o.customer_email ILIKE "+searchParameter+" OR COALESCE(f.carrier,'') ILIKE "+searchParameter+" OR COALESCE(f.tracking_number,'') ILIKE "+searchParameter+")")
	}
	filter := ""
	if len(where) > 0 {
		filter = " WHERE " + strings.Join(where, " AND ")
	}
	var total int
	queueFrom := ` FROM warehouse_fulfillments f JOIN orders o ON o.id=f.order_id
		LEFT JOIN users u ON u.id=o.customer_id OR (o.customer_id IS NULL AND LOWER(u.email)=LOWER(o.customer_email))`
	queueFilter := filter
	if queueFilter == "" {
		queueFilter = " WHERE COALESCE(u.fraud_flagged,FALSE)=FALSE"
	} else {
		queueFilter += " AND COALESCE(u.fraud_flagged,FALSE)=FALSE"
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*)`+queueFrom+queueFilter, args...).Scan(&total); err != nil {
		return nil, 0, WarehouseQueueStats{}, err
	}
	var stats WarehouseQueueStats
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FILTER (WHERE f.status NOT IN ('handed_over','cancelled')), COUNT(*) FILTER (WHERE f.status = 'handed_over'), COUNT(*) FILTER (WHERE f.status IN ('picking','packed','label_ready'))`+queueFrom+` WHERE COALESCE(u.fraud_flagged,FALSE)=FALSE`).Scan(&stats.Active, &stats.HandedOver, &stats.Processing); err != nil {
		return nil, 0, WarehouseQueueStats{}, err
	}
	pageArgs := append([]any(nil), args...)
	pageArgs = append(pageArgs, limit, offset)
	query := `SELECT f.order_id, f.status, COALESCE(f.assigned_to,''), COALESCE(f.carrier,''), COALESCE(f.tracking_number,''), COALESCE(f.tracking_url,''), COALESCE(f.label_url,''), COALESCE(f.hold_reason,''), COALESCE(f.held_by,''), COALESCE(f.resume_status,''), f.picked_at, f.packed_at, f.handed_over_at, f.held_at, f.updated_at, o.customer_name, o.total_cents, o.created_at
		` + queueFrom + queueFilter +
		fmt.Sprintf(" ORDER BY f.updated_at ASC, f.order_id ASC LIMIT $%d OFFSET $%d", len(pageArgs)-1, len(pageArgs))
	rows, err := s.db.QueryContext(ctx, query, pageArgs...)
	if err != nil {
		return nil, 0, WarehouseQueueStats{}, err
	}
	defer rows.Close()
	items := []WarehouseQueueItem{}
	for rows.Next() {
		var item WarehouseQueueItem
		if err := rows.Scan(&item.OrderID, &item.Status, &item.AssignedTo, &item.Carrier, &item.TrackingNumber, &item.TrackingURL, &item.LabelURL, &item.HoldReason, &item.HeldBy, &item.ResumeStatus, &item.PickedAt, &item.PackedAt, &item.HandedOverAt, &item.HeldAt, &item.UpdatedAt, &item.CustomerName, &item.TotalCents, &item.CreatedAt); err != nil {
			return nil, 0, WarehouseQueueStats{}, err
		}
		items = append(items, item)
	}
	return items, total, stats, rows.Err()
}

func (s *Store) WarehouseFulfillment(ctx context.Context, orderID string) (WarehouseFulfillment, []WarehouseEvent, int, error) {
	items, err := scanWarehouseQueue(ctx, s.db, `SELECT order_id, status, COALESCE(assigned_to,''), COALESCE(carrier,''), COALESCE(tracking_number,''), COALESCE(tracking_url,''), COALESCE(label_url,''), COALESCE(hold_reason,''), COALESCE(held_by,''), COALESCE(resume_status,''), picked_at, packed_at, handed_over_at, held_at, updated_at FROM warehouse_fulfillments WHERE order_id = $1`, orderID)
	if err != nil || len(items) == 0 {
		if err == nil {
			err = ErrNotFound
		}
		return WarehouseFulfillment{}, nil, 0, err
	}
	var eventCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM warehouse_events WHERE order_id=$1`, orderID).Scan(&eventCount); err != nil {
		return WarehouseFulfillment{}, nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT actor_email, event_type, detail, created_at FROM warehouse_events WHERE order_id=$1 ORDER BY id DESC LIMIT $2`, orderID, warehouseEventHistoryLimit)
	if err != nil {
		return WarehouseFulfillment{}, nil, 0, err
	}
	defer rows.Close()
	events := []WarehouseEvent{}
	for rows.Next() {
		var event WarehouseEvent
		if err := rows.Scan(&event.ActorEmail, &event.EventType, &event.Detail, &event.CreatedAt); err != nil {
			return WarehouseFulfillment{}, nil, 0, err
		}
		events = append(events, event)
	}
	return items[0], events, eventCount, rows.Err()
}

func (s *Store) HoldWarehouseFulfillment(ctx context.Context, orderID, actor, reason string) error {
	actor, reason = strings.TrimSpace(actor), strings.TrimSpace(reason)
	if actor == "" || len([]rune(reason)) < 3 || len([]rune(reason)) > 500 {
		return ErrInvalidFulfillmentHold
	}
	return s.withTransaction(ctx, func(tx *sql.Tx) error {
		var current string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM warehouse_fulfillments WHERE order_id=$1 FOR UPDATE`, orderID).Scan(&current); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if current == "held" || current == "handed_over" || current == "cancelled" {
			return ErrInvalidFulfillmentHold
		}
		if _, err := tx.ExecContext(ctx, `UPDATE warehouse_fulfillments SET status='held', resume_status=$2, hold_reason=$3, held_by=$4, held_at=NOW(), updated_at=NOW() WHERE order_id=$1`, orderID, current, reason, actor); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO warehouse_events (order_id, actor_email, event_type, detail) VALUES ($1,$2,'fulfillment.held',$3)`, orderID, actor, reason)
		return err
	})
}

func (s *Store) ReleaseWarehouseFulfillment(ctx context.Context, orderID, actor string) error {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return ErrInvalidFulfillmentHold
	}
	return s.withTransaction(ctx, func(tx *sql.Tx) error {
		var current, resume string
		if err := tx.QueryRowContext(ctx, `SELECT status,resume_status FROM warehouse_fulfillments WHERE order_id=$1 FOR UPDATE`, orderID).Scan(&current, &resume); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if current != "held" || resume == "" {
			return ErrInvalidFulfillmentHold
		}
		if _, err := tx.ExecContext(ctx, `UPDATE warehouse_fulfillments SET status=$2, resume_status='', hold_reason='', held_by='', held_at=NULL, updated_at=NOW() WHERE order_id=$1`, orderID, resume); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO warehouse_events (order_id, actor_email, event_type, detail) VALUES ($1,$2,'fulfillment.released',$3)`, orderID, actor, resume)
		return err
	})
}

// ensureOrderFulfillmentNotHeldTx protects shipment paths that do not first
// transition the warehouse record (for example a vetted OMS integration).
// The row lock keeps a concurrent hold from being lost between this check and
// the shipment write in the same transaction.
func ensureOrderFulfillmentNotHeldTx(ctx context.Context, tx *sql.Tx, orderID string) error {
	var status string
	err := tx.QueryRowContext(ctx, `SELECT status FROM warehouse_fulfillments WHERE order_id=$1 FOR UPDATE`, orderID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if status == "held" {
		return ErrInvalidFulfillmentHold
	}
	return nil
}

func (s *Store) TransitionWarehouseFulfillment(ctx context.Context, orderID, actor, next, detail string) error {
	return s.withTransaction(ctx, func(tx *sql.Tx) error {
		if err := ensureOrderNotFraudBlockedTx(ctx, tx, orderID); err != nil {
			return err
		}
		var current string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM warehouse_fulfillments WHERE order_id=$1 FOR UPDATE`, orderID).Scan(&current); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		allowed := map[string][]string{"ready_to_pick": {"picking"}, "picking": {"packed"}, "packed": {"label_ready"}, "label_ready": {"handed_over"}}
		valid := false
		for _, candidate := range allowed[current] {
			if candidate == next {
				valid = true
			}
		}
		if !valid {
			return ErrInvalidFulfillmentTransition
		}
		set := "status=$2, updated_at=NOW()"
		switch next {
		case "picking":
			set += ", assigned_to=$3, picked_at=NOW()"
		case "packed":
			set += ", packed_at=NOW()"
		}
		var updateErr error
		if next == "picking" {
			_, updateErr = tx.ExecContext(ctx, `UPDATE warehouse_fulfillments SET `+set+` WHERE order_id=$1`, orderID, next, actor)
		} else {
			_, updateErr = tx.ExecContext(ctx, `UPDATE warehouse_fulfillments SET `+set+` WHERE order_id=$1`, orderID, next)
		}
		if updateErr != nil {
			return updateErr
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO warehouse_events (order_id, actor_email, event_type, detail) VALUES ($1,$2,$3,$4)`, orderID, actor, "fulfillment."+next, detail)
		return err
	})
}

func (s *Store) AttachWarehouseLabel(ctx context.Context, orderID, actor, carrier, trackingNumber, trackingURL, labelURL string) error {
	return s.withTransaction(ctx, func(tx *sql.Tx) error {
		if err := ensureOrderNotFraudBlockedTx(ctx, tx, orderID); err != nil {
			return err
		}
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM warehouse_fulfillments WHERE order_id=$1 FOR UPDATE`, orderID).Scan(&status); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if status != "packed" {
			return ErrInvalidFulfillmentTransition
		}
		_, err := tx.ExecContext(ctx, `UPDATE warehouse_fulfillments SET status='label_ready',carrier=$2,tracking_number=$3,tracking_url=NULLIF($4,''),label_url=NULLIF($5,''),updated_at=NOW() WHERE order_id=$1`, orderID, carrier, trackingNumber, trackingURL, labelURL)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO warehouse_events (order_id, actor_email, event_type, detail) VALUES ($1,$2,'fulfillment.label_ready',$3)`, orderID, actor, fmt.Sprintf("%s %s", carrier, trackingNumber))
		return err
	})
}

func scanWarehouseQueue(ctx context.Context, q queryer, query string, args ...any) ([]WarehouseFulfillment, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []WarehouseFulfillment{}
	for rows.Next() {
		var item WarehouseFulfillment
		if err := rows.Scan(&item.OrderID, &item.Status, &item.AssignedTo, &item.Carrier, &item.TrackingNumber, &item.TrackingURL, &item.LabelURL, &item.HoldReason, &item.HeldBy, &item.ResumeStatus, &item.PickedAt, &item.PackedAt, &item.HandedOverAt, &item.HeldAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func validWarehouseURL(value string) bool {
	return value == "" || (strings.HasPrefix(value, "https://") && len(value) <= 2048)
}
