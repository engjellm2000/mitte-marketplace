package web

import (
	"testing"

	"mitte/internal/store"
)

func TestValidSupplierAction(t *testing.T) {
	tests := []struct {
		name  string
		input supplierOrderActionInput
		want  bool
	}{
		{name: "accept", input: supplierOrderActionInput{Action: "accept"}, want: true},
		{name: "delay without reason", input: supplierOrderActionInput{Action: "delay", DelayedDate: "2099-01-01"}},
		{name: "legacy shipment action", input: supplierOrderActionInput{Action: "shipment", Carrier: "DHL"}},
		{name: "cancellation needs a reason", input: supplierOrderActionInput{Action: "cancellation_request", Note: "No"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validSupplierAction(&test.input); got != test.want {
				t.Fatalf("validSupplierAction() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestValidSupplierShipment(t *testing.T) {
	tests := []struct {
		name  string
		input supplierShipmentInput
		want  bool
	}{
		{
			name: "multiple distinct positions",
			input: supplierShipmentInput{
				Carrier: "DHL", TrackingNumber: "123",
				Items: []store.SupplierShipmentLine{{ProductID: 1, Quantity: 2}, {ProductID: 2, Quantity: 1}},
			},
			want: true,
		},
		{
			name: "duplicate position",
			input: supplierShipmentInput{
				Carrier: "DHL", TrackingNumber: "123",
				Items: []store.SupplierShipmentLine{{ProductID: 1, Quantity: 1}, {ProductID: 1, Quantity: 1}},
			},
		},
		{
			name: "zero quantity",
			input: supplierShipmentInput{
				Carrier: "DHL", TrackingNumber: "123",
				Items: []store.SupplierShipmentLine{{ProductID: 1}},
			},
		},
		{
			name: "unsafe tracking URL",
			input: supplierShipmentInput{
				Carrier: "DHL", TrackingNumber: "123", TrackingURL: "http://carrier.example/123",
				Items: []store.SupplierShipmentLine{{ProductID: 1, Quantity: 1}},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validSupplierShipment(&test.input); got != test.want {
				t.Fatalf("validSupplierShipment() = %v, want %v", got, test.want)
			}
		})
	}
}
