package web

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"mitte/internal/store"
)

func (a *App) catalogProducts(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentStaff(w, r); !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	query := r.URL.Query()
	search := strings.TrimSpace(query.Get("search"))
	status := query.Get("status")
	if utf8.RuneCountInString(search) > maxOperationsCatalogSearchLength ||
		(status != "" && status != "draft" && status != "active" && status != "scheduled" && status != "archived") {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Katalogabfrage."})
		return
	}
	page, limit, offset, err := parseCatalogProductsPage(query.Get("page"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Katalogseite."})
		return
	}
	products, total, err := a.store.CatalogProducts(r.Context(), search, status, limit, offset)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	pages := (total + limit - 1) / limit
	if pages == 0 {
		pages = 1
	}
	w.Header().Set("Cache-Control", "private, no-store")
	respondJSON(w, http.StatusOK, map[string]any{
		"products": products, "total": total, "page": page, "pageSize": limit, "pages": pages,
	})
}

const (
	defaultCatalogProductsPageSize = 50
	maxCatalogProductsPage         = maxOperationsCatalogOffset/defaultCatalogProductsPageSize + 1
)

func parseCatalogProductsPage(pageValue string) (page, limit, offset int, err error) {
	page = 1
	if pageValue != "" {
		page, err = strconv.Atoi(pageValue)
		if err != nil || page < 1 || page > maxCatalogProductsPage {
			return 0, 0, 0, errors.New("invalid catalog page")
		}
	}
	limit = defaultCatalogProductsPageSize
	offset = (page - 1) * limit
	if offset > maxOperationsCatalogOffset {
		return 0, 0, 0, errors.New("catalog page offset exceeds maximum")
	}
	return page, limit, offset, nil
}

func (a *App) catalogPrices(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentStaff(w, r); !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	query := r.URL.Query()
	search := strings.TrimSpace(query.Get("search"))
	status := query.Get("status")
	sellableOnly := query.Get("sellable") == "true"
	if utf8.RuneCountInString(search) > maxOperationsCatalogSearchLength ||
		(status != "" && status != "draft" && status != "active" && status != "scheduled" && status != "archived") ||
		(query.Has("sellable") && query.Get("sellable") != "true" && query.Get("sellable") != "false") ||
		(sellableOnly && status != "" && status != "active") {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Preisabfrage."})
		return
	}
	page, limit, offset, err := parseOperationsCatalogPage(query.Get("page"))
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Preisseite."})
		return
	}
	prices, total, err := a.store.CatalogPricesPage(r.Context(), search, status, sellableOnly, limit, offset)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	pages := (total + limit - 1) / limit
	if pages == 0 {
		pages = 1
	}
	w.Header().Set("Cache-Control", "private, no-store")
	respondJSON(w, http.StatusOK, map[string]any{
		"prices": prices, "total": total, "page": page, "pageSize": limit, "pages": pages,
	})
}

const (
	defaultOperationsCatalogPageSize = 25
	maxOperationsCatalogPage         = 4001
	maxOperationsCatalogOffset       = 100000
	maxOperationsCatalogSearchLength = 120
)

func parseOperationsCatalogPage(pageValue string) (page, limit, offset int, err error) {
	page = 1
	if pageValue != "" {
		page, err = strconv.Atoi(pageValue)
		if err != nil || page < 1 || page > maxOperationsCatalogPage {
			return 0, 0, 0, errors.New("invalid operations catalog page")
		}
	}
	limit = defaultOperationsCatalogPageSize
	offset = (page - 1) * limit
	if offset > maxOperationsCatalogOffset {
		return 0, 0, 0, errors.New("operations catalog offset exceeds maximum")
	}
	return page, limit, offset, nil
}

func (a *App) inventoryOverview(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentStaff(w, r); !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	page := 1
	if value := r.URL.Query().Get("page"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Seite."})
			return
		}
		page = parsed
	}
	stock := r.URL.Query().Get("stock")
	if stock != "" && stock != "low" && stock != "out" && stock != "available" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültiger Bestandsfilter."})
		return
	}
	const pageSize = 50
	summary, products, total, err := a.store.InventoryOverview(r.Context(), r.URL.Query().Get("search"), stock, pageSize, (page-1)*pageSize)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"summary": summary, "products": products, "total": total, "page": page,
		"pageSize": pageSize, "pages": (total + pageSize - 1) / pageSize,
	})
}

func (a *App) inventoryMovements(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentStaff(w, r); !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	productID := int64(0)
	if value := r.URL.Query().Get("productId"); value != "" {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Produkt-ID."})
			return
		}
		productID = id
	}
	movements, err := a.store.InventoryMovements(r.Context(), productID, 50)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"movements": movements})
}

func catalogID(r *http.Request) (int64, error) { return strconv.ParseInt(r.PathValue("id"), 10, 64) }

func (a *App) catalogProduct(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentStaff(w, r); !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Nicht autorisiert."})
		return
	}
	id, err := catalogID(r)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Produkt-ID."})
		return
	}
	p, err := a.store.CatalogProduct(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Produkt nicht gefunden."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"product": p})
}

func (a *App) createCatalogProduct(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	user, ok := a.currentStaff(w, r)
	if !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Nicht autorisiert."})
		return
	}
	var in store.CatalogProductInput
	if err := decodeJSON(w, r, &in, 16384); err != nil || !validCatalogInput(&in) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe die Produktdaten."})
		return
	}
	p, err := a.store.CreateCatalogProduct(r.Context(), in, user.Email)
	if err != nil {
		if isCatalogConflict(err) {
			respondJSON(w, http.StatusConflict, map[string]string{"error": "Slug, SKU, Barcode oder GTIN ist bereits vergeben."})
			return
		}
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{"product": p})
}

func (a *App) updateCatalogProduct(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	user, ok := a.currentStaff(w, r)
	if !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Nicht autorisiert."})
		return
	}
	id, err := catalogID(r)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Produkt-ID."})
		return
	}
	var in store.CatalogProductInput
	if err := decodeJSON(w, r, &in, 16384); err != nil || !validCatalogInput(&in) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte prüfe die Produktdaten."})
		return
	}
	p, err := a.store.UpdateCatalogProduct(r.Context(), id, in, user.Email)
	if errors.Is(err, store.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Produkt nicht gefunden oder archiviert."})
		return
	}
	if err != nil {
		if isCatalogConflict(err) {
			respondJSON(w, http.StatusConflict, map[string]string{"error": "Slug, Barcode oder GTIN ist bereits vergeben."})
			return
		}
		a.serverError(w, r, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"product": p})
}

func isCatalogConflict(err error) bool {
	return strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique")
}

func (a *App) catalogStatus(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	user, ok := a.currentStaff(w, r)
	if !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Nicht autorisiert."})
		return
	}
	id, err := catalogID(r)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Produkt-ID."})
		return
	}
	var in struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(w, r, &in, 1024); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültiger Status."})
		return
	}
	if err := a.store.SetCatalogStatus(r.Context(), id, in.Status, user.Email); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "Produkt nicht gefunden."})
			return
		}
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültiger Statuswechsel."})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) catalogInventory(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	user, ok := a.currentStaff(w, r)
	if !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Nicht autorisiert."})
		return
	}
	id, err := catalogID(r)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Produkt-ID."})
		return
	}
	var in struct {
		Delta     int    `json:"delta"`
		Reason    string `json:"reason"`
		Reference string `json:"reference"`
	}
	if err := decodeJSON(w, r, &in, 2048); err != nil || in.Delta == 0 || in.Delta < -1000000 || in.Delta > 1000000 ||
		utf8.RuneCountInString(strings.TrimSpace(in.Reason)) < 3 || utf8.RuneCountInString(in.Reason) > 240 || utf8.RuneCountInString(in.Reference) > 120 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Gib eine Bestandsänderung und einen Grund an."})
		return
	}
	p, err := a.store.AdjustCatalogInventory(r.Context(), id, in.Delta, in.Reason, in.Reference, user.Email)
	if errors.Is(err, store.ErrOutOfStock) {
		respondJSON(w, http.StatusConflict, map[string]string{"error": "Der Bestand kann nicht negativ werden."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"product": p})
}

func (a *App) receiveInventory(w http.ResponseWriter, r *http.Request) {
	if !a.allowMutation(w, r) {
		return
	}
	user, ok := a.currentStaff(w, r)
	if !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Dieser Bereich ist nur für autorisierte Mitarbeitende verfügbar."})
		return
	}
	var in struct {
		ProductID int64  `json:"productId"`
		Quantity  int    `json:"quantity"`
		Reason    string `json:"reason"`
		Reference string `json:"reference"`
	}
	if err := decodeJSON(w, r, &in, 4096); err != nil || in.ProductID <= 0 || in.Quantity <= 0 || in.Quantity > 1000000 ||
		utf8.RuneCountInString(strings.TrimSpace(in.Reason)) < 3 || utf8.RuneCountInString(in.Reason) > 240 ||
		utf8.RuneCountInString(in.Reference) > 120 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Gib Produkt, positive Eingangsmenge und einen Grund an."})
		return
	}
	product, err := a.store.ReceiveCatalogInventory(r.Context(), in.ProductID, in.Quantity, strings.TrimSpace(in.Reason), strings.TrimSpace(in.Reference), user.Email)
	if errors.Is(err, store.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Produkt nicht gefunden."})
		return
	}
	if errors.Is(err, store.ErrInvalidQuantity) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Die Eingangsmenge ist ungültig."})
		return
	}
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"product": product})
}

func (a *App) catalogAudit(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentStaff(w, r); !ok {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "Nicht autorisiert."})
		return
	}
	id, err := catalogID(r)
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Ungültige Produkt-ID."})
		return
	}
	audit, err := a.store.CatalogAudit(r.Context(), id)
	if err != nil {
		a.serverError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	respondJSON(w, http.StatusOK, map[string]any{"audit": audit})
}

func validCatalogInput(in *store.CatalogProductInput) bool {
	if in.TechnicalDetails == nil {
		in.TechnicalDetails = map[string]string{}
	}
	in.Slug = strings.TrimSpace(strings.ToLower(in.Slug))
	in.Name = strings.TrimSpace(in.Name)
	in.ShortDescription = strings.TrimSpace(in.ShortDescription)
	in.Description = strings.TrimSpace(in.Description)
	in.ImageURL = strings.TrimSpace(in.ImageURL)
	in.ImageAlt = strings.TrimSpace(in.ImageAlt)
	in.SEOTitle = strings.TrimSpace(in.SEOTitle)
	in.SEODescription = strings.TrimSpace(in.SEODescription)
	in.SKU = strings.TrimSpace(strings.ToUpper(in.SKU))
	in.Barcode = strings.TrimSpace(in.Barcode)
	in.SupplierName = strings.TrimSpace(in.SupplierName)
	in.Manufacturer = strings.TrimSpace(in.Manufacturer)
	in.ManufacturerSKU = strings.TrimSpace(in.ManufacturerSKU)
	in.GTIN = strings.TrimSpace(in.GTIN)
	in.Material = strings.TrimSpace(in.Material)
	in.Dimensions = strings.TrimSpace(in.Dimensions)
	if in.GTIN != "" && !validGTIN(in.GTIN) {
		return false
	}
	detailsSize := 0
	for key, value := range in.TechnicalDetails {
		detailsSize += len(key) + len(value)
		if strings.TrimSpace(key) == "" || utf8.RuneCountInString(key) > 80 || utf8.RuneCountInString(value) > 500 {
			return false
		}
	}
	return in.CategoryID > 0 && in.PriceCents >= 0 && in.PriceCents <= 100000000 && in.Slug != "" && in.Name != "" &&
		in.ShortDescription != "" && in.Description != "" && in.ImageURL != "" && in.ImageAlt != "" &&
		in.UnitCostCents >= 0 && in.UnitCostCents <= 100000000 && in.ReorderPoint >= 0 && in.ReorderPoint <= 1000000 &&
		in.LeadTimeDays >= 0 && in.LeadTimeDays <= 3650 && in.InitialStock >= 0 && in.InitialStock <= 1000000 &&
		utf8.RuneCountInString(in.SKU) <= 80 && utf8.RuneCountInString(in.Barcode) <= 80 &&
		utf8.RuneCountInString(in.Manufacturer) <= 200 && utf8.RuneCountInString(in.ManufacturerSKU) <= 100 &&
		utf8.RuneCountInString(in.Material) <= 200 && utf8.RuneCountInString(in.Dimensions) <= 200 &&
		in.WeightGrams >= 0 && in.WeightGrams <= 1000000000 && len(in.TechnicalDetails) <= 40 && detailsSize <= 8000 &&
		utf8.RuneCountInString(in.SupplierName) <= 200 && utf8.RuneCountInString(in.Slug) <= 160 &&
		utf8.RuneCountInString(in.Name) <= 200 && utf8.RuneCountInString(in.Description) <= 20000
}

func validGTIN(value string) bool {
	if len(value) != 8 && len(value) != 12 && len(value) != 13 && len(value) != 14 {
		return false
	}
	sum, weight := 0, 3
	for index := len(value) - 2; index >= 0; index-- {
		digit := value[index] - '0'
		if digit > 9 {
			return false
		}
		sum += int(digit) * weight
		if weight == 3 {
			weight = 1
		} else {
			weight = 3
		}
	}
	checkDigit := (10 - sum%10) % 10
	return value[len(value)-1] >= '0' && value[len(value)-1] <= '9' &&
		int(value[len(value)-1]-'0') == checkDigit
}
