package web

import "testing"

func TestParseCatalogProductsPage(t *testing.T) {
	tests := []struct {
		name       string
		page       string
		wantPage   int
		wantLimit  int
		wantOffset int
		wantError  bool
	}{
		{name: "defaults", wantPage: 1, wantLimit: defaultCatalogProductsPageSize},
		{name: "valid page", page: "3", wantPage: 3, wantLimit: defaultCatalogProductsPageSize, wantOffset: 100},
		{name: "last allowed page", page: "2001", wantPage: 2001, wantLimit: defaultCatalogProductsPageSize, wantOffset: 100000},
		{name: "page zero", page: "0", wantError: true},
		{name: "invalid page", page: "x", wantError: true},
		{name: "offset too large", page: "2002", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page, limit, offset, err := parseCatalogProductsPage(test.page)
			if (err != nil) != test.wantError {
				t.Fatalf("parseCatalogProductsPage() error = %v, wantError %v", err, test.wantError)
			}
			if err != nil {
				return
			}
			if page != test.wantPage || limit != test.wantLimit || offset != test.wantOffset {
				t.Fatalf("parseCatalogProductsPage() = (%d, %d, %d), want (%d, %d, %d)", page, limit, offset, test.wantPage, test.wantLimit, test.wantOffset)
			}
		})
	}
}
