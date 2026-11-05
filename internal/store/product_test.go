package store

import (
	"encoding/json"
	"testing"
)

func TestProductSpecificationsUseStorefrontJSONFields(t *testing.T) {
	product := Product{
		Manufacturer: "Example maker", ManufacturerSKU: "M-123", GTIN: "4006381333931",
		Material: "Steel", Dimensions: "10 x 20 cm", WeightGrams: 1250,
		TechnicalDetails: map[string]string{"Finish": "Matte"},
		PriceGrossCents:  11900, PriceNetCents: 10000,
	}
	encoded, err := json.Marshal(product)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		"manufacturer", "manufacturerSku", "gtin", "material", "dimensions",
		"weightGrams", "technicalDetails", "priceGrossCents", "priceNetCents",
	} {
		if _, ok := fields[key]; !ok {
			t.Errorf("product JSON is missing %q: %s", key, encoded)
		}
	}
	if _, ok := fields["Manufacturer"]; ok {
		t.Errorf("product JSON unexpectedly contains capitalized manufacturer field: %s", encoded)
	}
}
