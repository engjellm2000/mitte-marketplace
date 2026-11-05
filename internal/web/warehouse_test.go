package web

import (
	"strings"
	"testing"
)

func TestParseWarehouseQueuePage(t *testing.T) {
	tests := []struct {
		name                string
		page, limit         string
		wantPage, wantLimit int
		wantOffset          int
		wantError           bool
	}{
		{name: "defaults", wantPage: 1, wantLimit: defaultWarehouseQueuePageSize},
		{name: "requested page", page: "3", limit: "20", wantPage: 3, wantLimit: 20, wantOffset: 40},
		{name: "invalid page", page: "0", wantError: true},
		{name: "invalid limit", limit: "101", wantError: true},
		{name: "offset bound", page: "2002", limit: "50", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page, limit, offset, err := parseWarehouseQueuePage(test.page, test.limit)
			if (err != nil) != test.wantError {
				t.Fatalf("parseWarehouseQueuePage() error = %v, wantError %v", err, test.wantError)
			}
			if err == nil && (page != test.wantPage || limit != test.wantLimit || offset != test.wantOffset) {
				t.Fatalf("parseWarehouseQueuePage() = (%d, %d, %d), want (%d, %d, %d)", page, limit, offset, test.wantPage, test.wantLimit, test.wantOffset)
			}
		})
	}
}

func TestValidWarehouseQueueStatus(t *testing.T) {
	for _, status := range []string{"", "all", "active", "ready_to_pick", "picking", "packed", "label_ready", "handed_over", "cancelled"} {
		if !validWarehouseQueueStatus(status) {
			t.Errorf("validWarehouseQueueStatus(%q) = false", status)
		}
	}
	for _, status := range []string{"pending", "unknown", "paid"} {
		if validWarehouseQueueStatus(status) {
			t.Errorf("validWarehouseQueueStatus(%q) = true", status)
		}
	}
}

func TestParseWarehouseOrderIDs(t *testing.T) {
	ids, err := parseWarehouseOrderIDs("order-1, order-2,order-1")
	if err != nil || len(ids) != 2 || ids[0] != "order-1" || ids[1] != "order-2" {
		t.Fatalf("parseWarehouseOrderIDs() = (%v, %v), want unique trimmed IDs", ids, err)
	}
	for _, value := range []string{"", "order-1,,order-2", strings.Repeat("x", maxWarehouseOrderIDLength+1)} {
		if _, err := parseWarehouseOrderIDs(value); err == nil {
			t.Errorf("parseWarehouseOrderIDs(%q) succeeded, want error", value)
		}
	}
}
