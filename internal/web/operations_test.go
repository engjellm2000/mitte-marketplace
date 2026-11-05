package web

import "testing"

func TestParseOperationsOrderPage(t *testing.T) {
	tests := []struct {
		name       string
		page       string
		limit      string
		wantPage   int
		wantLimit  int
		wantOffset int
		wantError  bool
	}{
		{name: "defaults", wantPage: 1, wantLimit: defaultOperationsOrderPageSize},
		{name: "valid page", page: "3", limit: "20", wantPage: 3, wantLimit: 20, wantOffset: 40},
		{name: "page zero", page: "0", wantError: true},
		{name: "invalid page", page: "x", wantError: true},
		{name: "limit too large", limit: "101", wantError: true},
		{name: "offset too large", page: "10000", limit: "25", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page, limit, offset, err := parseOperationsOrderPage(test.page, test.limit)
			if (err != nil) != test.wantError {
				t.Fatalf("parseOperationsOrderPage() error = %v, wantError %v", err, test.wantError)
			}
			if err != nil {
				return
			}
			if page != test.wantPage || limit != test.wantLimit || offset != test.wantOffset {
				t.Fatalf("parseOperationsOrderPage() = (%d, %d, %d), want (%d, %d, %d)", page, limit, offset, test.wantPage, test.wantLimit, test.wantOffset)
			}
		})
	}
}

func TestValidOperationsOrderStatus(t *testing.T) {
	for _, status := range []string{"all", "open", "paid", "pending_payment", "invoice_approved", "fulfilled", "cancelled"} {
		if !validOperationsOrderStatus(status) {
			t.Errorf("validOperationsOrderStatus(%q) = false", status)
		}
	}
	if validOperationsOrderStatus("unknown") {
		t.Fatal("validOperationsOrderStatus(unknown) = true")
	}
}

func TestValidOperationsBacklogStage(t *testing.T) {
	for _, stage := range []string{"all", "payment", "needs_fulfillment", "in_progress", "held"} {
		if !validOperationsBacklogStage(stage) {
			t.Errorf("validOperationsBacklogStage(%q) = false", stage)
		}
	}
	if validOperationsBacklogStage("overdue") {
		t.Fatal("validOperationsBacklogStage(overdue) = true")
	}
}

func TestValidOperationsMatrixKind(t *testing.T) {
	for _, kind := range []string{"fulfillment", "fulfillment-clarify", "payment", "payment-clarify", "old-orders", "old-ops-clarify"} {
		if !validOperationsMatrixKind(kind) {
			t.Errorf("validOperationsMatrixKind(%q) = false", kind)
		}
	}
	if validOperationsMatrixKind("unknown") {
		t.Fatal("validOperationsMatrixKind(unknown) = true")
	}
}

func TestValidOperationsReturnRequestStatus(t *testing.T) {
	for _, status := range []string{"all", "return_open", "replacement_requested"} {
		if !validOperationsReturnRequestStatus(status) {
			t.Errorf("validOperationsReturnRequestStatus(%q) = false", status)
		}
	}
	if validOperationsReturnRequestStatus("fulfilled") {
		t.Fatal("validOperationsReturnRequestStatus(fulfilled) = true")
	}
}

func TestValidOperationsEmailDeliveryStatus(t *testing.T) {
	for _, status := range []string{"all", "pending", "sending", "retrying", "sent", "cancelled"} {
		if !validOperationsEmailDeliveryStatus(status) {
			t.Errorf("validOperationsEmailDeliveryStatus(%q) = false", status)
		}
	}
	if validOperationsEmailDeliveryStatus("failed") {
		t.Fatal("validOperationsEmailDeliveryStatus(failed) = true")
	}
}

func TestParseOperationsCustomerPage(t *testing.T) {
	tests := []struct {
		name       string
		page       string
		limit      string
		wantPage   int
		wantLimit  int
		wantOffset int
		wantError  bool
	}{
		{name: "defaults", wantPage: 1, wantLimit: defaultOperationsCustomerPageSize},
		{name: "valid page", page: "3", limit: "20", wantPage: 3, wantLimit: 20, wantOffset: 40},
		{name: "page zero", page: "0", wantError: true},
		{name: "invalid page", page: "x", wantError: true},
		{name: "limit too large", limit: "101", wantError: true},
		{name: "offset too large", page: "10000", limit: "25", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page, limit, offset, err := parseOperationsCustomerPage(test.page, test.limit)
			if (err != nil) != test.wantError {
				t.Fatalf("parseOperationsCustomerPage() error = %v, wantError %v", err, test.wantError)
			}
			if err != nil {
				return
			}
			if page != test.wantPage || limit != test.wantLimit || offset != test.wantOffset {
				t.Fatalf("parseOperationsCustomerPage() = (%d, %d, %d), want (%d, %d, %d)", page, limit, offset, test.wantPage, test.wantLimit, test.wantOffset)
			}
		})
	}
}

func TestParseOperationsCatalogPage(t *testing.T) {
	tests := []struct {
		name       string
		page       string
		wantPage   int
		wantLimit  int
		wantOffset int
		wantError  bool
	}{
		{name: "defaults", wantPage: 1, wantLimit: defaultOperationsCatalogPageSize},
		{name: "valid page", page: "5", wantPage: 5, wantLimit: 25, wantOffset: 100},
		{name: "page zero", page: "0", wantError: true},
		{name: "invalid page", page: "x", wantError: true},
		{name: "offset too large", page: "4002", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page, limit, offset, err := parseOperationsCatalogPage(test.page)
			if (err != nil) != test.wantError {
				t.Fatalf("parseOperationsCatalogPage() error = %v, wantError %v", err, test.wantError)
			}
			if err != nil {
				return
			}
			if page != test.wantPage || limit != test.wantLimit || offset != test.wantOffset {
				t.Fatalf("parseOperationsCatalogPage() = (%d, %d, %d), want (%d, %d, %d)", page, limit, offset, test.wantPage, test.wantLimit, test.wantOffset)
			}
		})
	}
}
