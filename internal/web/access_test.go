package web

import "testing"

func TestStaffRolePermissions(t *testing.T) {
	for _, test := range []struct {
		role       string
		permission string
		want       bool
	}{
		{role: "admin", permission: "payments", want: true},
		{role: "admin", permission: "orders", want: true},
		{role: "cs", permission: "orders", want: true},
		{role: "cs", permission: "warehouse"},
		{role: "cs", permission: "payments"},
		{role: "operations", permission: "orders", want: true},
		{role: "operations", permission: "warehouse", want: true},
		{role: "warehouse", permission: "inventory"},
		{role: "catalog", permission: "catalog", want: true},
		{role: "unknown", permission: "orders"},
	} {
		if got := staffRoleAllows(test.role, test.permission); got != test.want {
			t.Errorf("staffRoleAllows(%q, %q) = %t, want %t", test.role, test.permission, got, test.want)
		}
	}
}

func TestStaffRoleAllowsFinancialReversal(t *testing.T) {
	for _, test := range []struct {
		role string
		want bool
	}{
		{role: "admin", want: true},
		{role: "payments", want: false},
		{role: "operations", want: false},
		{role: "cs", want: false},
	} {
		if got := staffRoleAllowsFinancialReversal(test.role); got != test.want {
			t.Errorf("staffRoleAllowsFinancialReversal(%q) = %t, want %t", test.role, got, test.want)
		}
	}
}

func TestStaffPermissionForPath(t *testing.T) {
	for path, want := range map[string]string{
		"/api/operations/orders":                    "orders",
		"/api/operations/backlog":                   "orders",
		"/api/operations/order-items":               "orders",
		"/api/operations/return-requests":           "orders",
		"/api/operations/shipment-delays":           "orders",
		"/api/operations/orders/123/ship":           "warehouse",
		"/api/operations/customers":                 "orders",
		"/api/operations/customers/123/orders":      "orders",
		"/api/admin/customer-fraud/123":             "orders",
		"/api/operations/email-deliveries":          "orders",
		"/oms/customers":                            "orders",
		"/oms/categories":                           "catalog",
		"/oms/pricing":                              "catalog",
		"/oms/queues/sellable":                      "catalog",
		"/oms/queues/reorder":                       "inventory",
		"/api/warehouse/fulfillments":               "warehouse",
		"/api/warehouse/returns":                    "warehouse",
		"/api/warehouse/returns/12/inspect":         "warehouse",
		"/api/admin/catalog/products/123/inventory": "inventory",
		"/api/admin/catalog/products/123":           "catalog",
		"/api/admin/catalog/prices":                 "catalog",
		"/api/admin/promotions":                     "catalog",
		"/api/admin/marketing-emails":               "catalog",
		"/api/payment-receipts":                     "payments",
		"/api/unknown-operation":                    "admin",
	} {
		if got := staffPermissionForPath(path); got != want {
			t.Errorf("staffPermissionForPath(%q) = %q, want %q", path, got, want)
		}
	}
}
