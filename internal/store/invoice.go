package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"mitte/internal/invoiceprofile"
)

type InvoiceSeller = invoiceprofile.Profile

func validInvoiceSeller(seller InvoiceSeller) bool {
	return seller.Valid()
}

func issueInvoiceTx(ctx context.Context, tx executor, orderID, provider string, total int, seller InvoiceSeller) (Invoice, string, error) {
	if !validInvoiceSeller(seller) {
		return Invoice{}, "", fmt.Errorf("invoice seller profile is incomplete")
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return Invoice{}, "", fmt.Errorf("generate invoice access token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)
	hash := sha256.Sum256([]byte(token))
	sellerSnapshot, err := json.Marshal(seller)
	if err != nil {
		return Invoice{}, "", fmt.Errorf("encode invoice seller snapshot: %w", err)
	}
	number := "MF-" + time.Now().UTC().Format("2006") + "-" + orderID
	dueInterval := "0 days"
	if provider == "mitte_invoice" {
		dueInterval = "30 days"
	}
	invoice := Invoice{Number: number, TotalCents: total}
	err = tx.QueryRowContext(ctx, `INSERT INTO invoices
		(order_id,invoice_number,total_cents,due_at,access_token_hash,seller_snapshot)
		VALUES ($1,$2,$3,NOW() + $4::interval,$5,$6::jsonb)
		ON CONFLICT (order_id) DO UPDATE SET
			access_token_hash=EXCLUDED.access_token_hash,
			seller_snapshot=EXCLUDED.seller_snapshot
		RETURNING invoice_number,issued_at,due_at,total_cents`,
		orderID, number, total, dueInterval, hash[:], sellerSnapshot).
		Scan(&invoice.Number, &invoice.IssuedAt, &invoice.DueAt, &invoice.TotalCents)
	if err != nil {
		return Invoice{}, "", fmt.Errorf("persist shipment invoice: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO invoice_documents
		(order_id,document_type,document_number,issued_at,total_cents,currency,created_by)
		VALUES ($1,'invoice',$2,$3,$4,'EUR','system')
		ON CONFLICT (order_id,document_type) DO NOTHING`,
		orderID, invoice.Number, invoice.IssuedAt, invoice.TotalCents)
	if err != nil {
		return Invoice{}, "", fmt.Errorf("record issued invoice document: %w", err)
	}
	return invoice, token, nil
}

func (s *Store) InvoiceByAccessToken(ctx context.Context, token string) (Order, InvoiceSeller, error) {
	if len(token) != 64 {
		return Order{}, InvoiceSeller{}, ErrNotFound
	}
	if _, err := hex.DecodeString(token); err != nil {
		return Order{}, InvoiceSeller{}, ErrNotFound
	}
	hash := sha256.Sum256([]byte(token))
	var orderID string
	var sellerJSON []byte
	err := s.db.QueryRowContext(ctx, `SELECT order_id,seller_snapshot
		FROM invoices WHERE access_token_hash=$1`, hash[:]).Scan(&orderID, &sellerJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, InvoiceSeller{}, ErrNotFound
	}
	if err != nil {
		return Order{}, InvoiceSeller{}, err
	}
	seller := InvoiceSeller{}
	if len(sellerJSON) > 0 {
		if err := json.Unmarshal(sellerJSON, &seller); err != nil {
			return Order{}, InvoiceSeller{}, fmt.Errorf("decode invoice seller snapshot: %w", err)
		}
	}
	orders, err := s.ordersForOperations(ctx, "WHERE id = $1", orderID)
	if err != nil {
		return Order{}, InvoiceSeller{}, err
	}
	if len(orders) == 0 {
		return Order{}, InvoiceSeller{}, ErrNotFound
	}
	return orders[0], seller, nil
}

func (s *Store) InvoiceSellerForOrder(ctx context.Context, orderID string) (InvoiceSeller, error) {
	var snapshot []byte
	err := s.db.QueryRowContext(ctx, `SELECT seller_snapshot FROM invoices WHERE order_id=$1`, orderID).Scan(&snapshot)
	if errors.Is(err, sql.ErrNoRows) {
		return InvoiceSeller{}, ErrNotFound
	}
	if err != nil {
		return InvoiceSeller{}, err
	}
	seller := InvoiceSeller{}
	if len(snapshot) > 0 {
		if err := json.Unmarshal(snapshot, &seller); err != nil {
			return InvoiceSeller{}, fmt.Errorf("decode invoice seller snapshot: %w", err)
		}
	}
	return seller, nil
}

func invoiceAccessURL(baseURL, token string) string {
	return strings.TrimRight(baseURL, "/") + "/api/invoices/" + token
}
