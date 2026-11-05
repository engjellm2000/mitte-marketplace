package oms

import (
	"context"
	"database/sql"
	"time"

	"mitte/internal/store"
)

type healthRepository interface {
	DatabaseStats() sql.DBStats
	Ping(context.Context) error
}

type sessionRepository interface {
	CreateOperatorSession(context.Context, string, string, time.Time) error
	OperatorSessionPrincipal(context.Context, string, time.Time) (string, error)
	RevokeOperatorSession(context.Context, string, time.Time) error
}

type orderRepository interface {
	AddInternalOrderNote(context.Context, string, string, string) (store.InternalOrderNote, error)
	CancelPaidOrderForOperations(context.Context, string, string, string) error
	CreateInvoiceDocument(context.Context, string, string, string, string) (store.InvoiceDocument, error)
	InvoiceSellerForOrder(context.Context, string) (store.InvoiceSeller, error)
	MarkOrderShippedBy(context.Context, string, string, string, string, string, string, store.InvoiceSeller) error
	MarkWarehouseShipmentBy(context.Context, string, string, string, string, string, string, store.InvoiceSeller) error
	OrderManagementDetail(context.Context, string) (store.OrderManagementDetail, error)
	OrdersForOperations(context.Context) ([]store.Order, error)
}

type fulfillmentRepository interface {
	AttachWarehouseLabel(context.Context, string, string, string, string, string, string) error
	HoldWarehouseFulfillment(context.Context, string, string, string) error
	ReleaseWarehouseFulfillment(context.Context, string, string) error
	TransitionWarehouseFulfillment(context.Context, string, string, string, string) error
	WarehouseFulfillment(context.Context, string) (store.WarehouseFulfillment, []store.WarehouseEvent, int, error)
	WarehouseFulfillmentsByOrderIDs(context.Context, []string) ([]store.WarehouseFulfillment, error)
	WarehouseQueuePage(context.Context, string, string, int, int) ([]store.WarehouseQueueItem, int, store.WarehouseQueueStats, error)
}

type Repository interface {
	healthRepository
	sessionRepository
	orderRepository
	fulfillmentRepository
}
