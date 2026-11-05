package store

import "testing"

func TestValidOrderStatusTransition(t *testing.T) {
	tests := []struct {
		from string
		to   string
		want bool
	}{
		{"pending_payment", "paid", true},
		{"pending_payment", "cancelled", true},
		{"pending_payment", "fulfilled", false},
		{"paid", "fulfilled", true},
		{"paid", "cancelled", true},
		{"fulfilled", "cancelled", false},
		{"cancelled", "paid", false},
	}

	for _, test := range tests {
		if got := ValidOrderStatusTransition(test.from, test.to); got != test.want {
			t.Fatalf("ValidOrderStatusTransition(%q, %q) = %v, want %v", test.from, test.to, got, test.want)
		}
	}
}

func TestOrderNumberIsEightNumericDigits(t *testing.T) {
	tests := []struct {
		number int64
		want   string
	}{
		{number: 10000000, want: "10000000"},
		{number: 99999999, want: "99999999"},
	}

	for _, test := range tests {
		got, err := formatOrderNumber(test.number)
		if err != nil {
			t.Fatalf("formatOrderNumber(%d) error: %v", test.number, err)
		}
		if got != test.want {
			t.Errorf("formatOrderNumber(%d) = %q, want %q", test.number, got, test.want)
		}
	}

	for _, number := range []int64{9999999, 100000000} {
		if got, err := formatOrderNumber(number); err == nil {
			t.Errorf("formatOrderNumber(%d) = %q, want an error", number, got)
		}
	}
}
