package web

import "testing"

func TestParseProductListOptions(t *testing.T) {
	tests := []struct {
		name     string
		page     string
		sort     string
		wantPage int
		wantSort string
		wantErr  bool
	}{
		{name: "defaults", wantPage: 1, wantSort: "featured"},
		{name: "valid page and price sort", page: "4", sort: "price-low", wantPage: 4, wantSort: "price-low"},
		{name: "maximum page", page: "10000", sort: "price-high", wantPage: 10000, wantSort: "price-high"},
		{name: "best-selling sort", sort: "bestselling", wantPage: 1, wantSort: "bestselling"},
		{name: "non-numeric page", page: "many", wantErr: true},
		{name: "zero page", page: "0", wantErr: true},
		{name: "oversized page", page: "10001", wantErr: true},
		{name: "unknown sort", sort: "random", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page, sort, err := parseProductListOptions(test.page, test.sort)
			if (err != nil) != test.wantErr {
				t.Fatalf("parseProductListOptions() error = %v, wantErr %t", err, test.wantErr)
			}
			if err == nil && (page != test.wantPage || sort != test.wantSort) {
				t.Fatalf("parseProductListOptions() = (%d, %q), want (%d, %q)", page, sort, test.wantPage, test.wantSort)
			}
		})
	}
}
