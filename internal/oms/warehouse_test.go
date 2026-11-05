package oms

import "testing"

func TestParseFulfillmentPage(t *testing.T) {
	tests := []struct {
		name                string
		page, limit         string
		wantPage, wantLimit int
		wantOffset          int
		wantError           bool
	}{
		{name: "defaults", wantPage: 1, wantLimit: defaultFulfillmentPageSize},
		{name: "requested page", page: "4", limit: "25", wantPage: 4, wantLimit: 25, wantOffset: 75},
		{name: "invalid page", page: "0", wantError: true},
		{name: "invalid limit", limit: "101", wantError: true},
		{name: "offset bound", page: "2002", limit: "50", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page, limit, offset, err := parseFulfillmentPage(test.page, test.limit)
			if (err != nil) != test.wantError {
				t.Fatalf("parseFulfillmentPage() error = %v, wantError %v", err, test.wantError)
			}
			if err == nil && (page != test.wantPage || limit != test.wantLimit || offset != test.wantOffset) {
				t.Fatalf("parseFulfillmentPage() = (%d, %d, %d), want (%d, %d, %d)", page, limit, offset, test.wantPage, test.wantLimit, test.wantOffset)
			}
		})
	}
}

func TestParseFulfillmentOrderIDs(t *testing.T) {
	ids, err := parseFulfillmentOrderIDs("order-1, order-2,order-1")
	if err != nil || len(ids) != 2 || ids[0] != "order-1" || ids[1] != "order-2" {
		t.Fatalf("parseFulfillmentOrderIDs() = (%v, %v), want unique trimmed IDs", ids, err)
	}
	if _, err := parseFulfillmentOrderIDs("order-1,,order-2"); err == nil {
		t.Fatal("parseFulfillmentOrderIDs() accepted an empty ID")
	}
}
