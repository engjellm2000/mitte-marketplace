package web

import (
	"context"
	"slices"
	"strings"
	"time"
)

type operatorSessionStore interface {
	CreateOperatorSession(context.Context, string, string, time.Time) error
	OperatorSessionPrincipal(context.Context, string, time.Time) (string, error)
	RevokeOperatorSession(context.Context, string, time.Time) error
}

var staffPermissions = []string{"orders", "warehouse", "inventory", "catalog", "payments"}

func staffPermissionsForRole(role string) []string {
	switch role {
	case "admin":
		return slices.Clone(staffPermissions)
	case "cs":
		return []string{"orders"}
	case "operations":
		return []string{"orders", "warehouse"}
	case "orders", "warehouse", "inventory", "catalog", "payments":
		return []string{role}
	default:
		return nil
	}
}

func staffPermissionForPath(path string) string {
	switch {
	case strings.HasPrefix(path, "/api/admin/customer-fraud"):
		return "orders"
	case strings.HasPrefix(path, "/api/operations/orders/") && strings.HasSuffix(path, "/ship"):
		return "warehouse"
	case path == "/api/operations/backlog":
		return "orders"
	case strings.HasPrefix(path, "/api/operations/email-deliveries"):
		return "orders"
	case strings.HasPrefix(path, "/api/operations/return-requests"):
		return "orders"
	case strings.HasPrefix(path, "/api/warehouse/returns"):
		return "warehouse"
	case strings.HasPrefix(path, "/api/operations/shipment-delays"):
		return "orders"
	case strings.HasPrefix(path, "/api/operations/supplier-requests"):
		return "orders"
	case strings.HasPrefix(path, "/oms/categories"):
		return "catalog"
	case strings.HasPrefix(path, "/oms/pricing"), strings.HasPrefix(path, "/oms/queues/sellable"):
		return "catalog"
	case strings.HasPrefix(path, "/oms/queues/reorder"):
		return "inventory"
	case strings.HasPrefix(path, "/api/admin/catalog/products/") && strings.Contains(path, "/inventory"):
		return "inventory"
	case strings.HasPrefix(path, "/api/admin/promotions"), strings.HasPrefix(path, "/api/admin/marketing-emails"):
		return "catalog"
	case strings.HasPrefix(path, "/oms"), strings.HasPrefix(path, "/api/operations/orders"),
		strings.HasPrefix(path, "/api/operations/customers"), strings.HasPrefix(path, "/api/operations/order-items"):
		return "orders"
	case strings.HasPrefix(path, "/verwaltung"), strings.HasPrefix(path, "/api/warehouse/fulfillments"):
		return "warehouse"
	case strings.HasPrefix(path, "/bestand"), strings.HasPrefix(path, "/api/admin/inventory"):
		return "inventory"
	case strings.HasPrefix(path, "/zahlungen"),
		strings.HasPrefix(path, "/api/payments"),
		strings.HasPrefix(path, "/api/payment-receipts"),
		strings.HasPrefix(path, "/api/mitte-invoice"):
		return "payments"
	case strings.HasPrefix(path, "/admin/catalog"), strings.HasPrefix(path, "/api/admin/catalog"),
		strings.HasPrefix(path, "/admin/email-marketing"), strings.HasPrefix(path, "/api/admin/marketing-emails"):
		return "catalog"
	default:
		return "admin"
	}
}

func staffRoleAllows(role, permission string) bool {
	return slices.Contains(staffPermissionsForRole(role), permission)
}

// staffRoleAllowsFinancialReversal deliberately does not inherit the general
// orders permission. A paid-order cancellation and refund both move money and
// create accounting documents, so they require the explicit administrator
// role rather than everyday customer-service access.
func staffRoleAllowsFinancialReversal(role string) bool {
	return role == "admin"
}
