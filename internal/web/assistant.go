package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"mitte/internal/store"
)

const (
	maxAssistantMessages = 12
	maxAssistantMessage  = 1200
	maxAssistantRequest  = 16 << 10
	maxAssistantProducts = 8
	geminiDegradedTTL    = 10 * time.Minute
	maxGeminiToolRounds  = 4
)

type assistantMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type assistantChatInput struct {
	Messages []assistantMessage `json:"messages"`
}

type assistantChatOutput struct {
	Reply    string                 `json:"reply"`
	Products []store.ProductSummary `json:"products"`
}

type geminiAssistant struct {
	apiKey     string
	model      string
	endpoint   string
	httpClient *http.Client
	store      *store.Store

	mu            sync.Mutex
	degradedUntil time.Time
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string                  `json:"text,omitempty"`
	ThoughtSignature string                  `json:"thoughtSignature,omitempty"`
	FunctionCall     *geminiFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResponse `json:"functionResponse,omitempty"`
}

type geminiFunctionCall struct {
	Name string         `json:"name"`
	ID   string         `json:"id,omitempty"`
	Args map[string]any `json:"args"`
}

type geminiFunctionResponse struct {
	Name     string         `json:"name"`
	ID       string         `json:"id,omitempty"`
	Response map[string]any `json:"response"`
}

type geminiGenerateResponse struct {
	Candidates []struct {
		Content geminiContent `json:"content"`
	} `json:"candidates"`
}

type catalogIntent struct {
	kind         string
	query        string
	categorySlug string
	maxCents     int
	sort         string
	inStockOnly  bool
}

var (
	budgetEuroPattern = regexp.MustCompile(`(?i)(?:unter|bis(?:\s+zu)?|max(?:imal)?|höchstens|weniger als)\s*(\d{1,5})\s*(?:€|eur|euro)?`)
	budgetPlainEuro   = regexp.MustCompile(`(?i)\b(\d{1,5})\s*(?:€|eur|euro)\b`)
)

func newGeminiAssistant(apiKey, model string, repository *store.Store) *geminiAssistant {
	if model == "" {
		model = "gemini-3.8-flash"
	}
	return &geminiAssistant{
		apiKey:     apiKey,
		model:      model,
		endpoint:   "https://generativelanguage.googleapis.com",
		httpClient: &http.Client{Timeout: 20 * time.Second},
		store:      repository,
	}
}

func (a *geminiAssistant) aiAvailable() bool {
	if a == nil || strings.TrimSpace(a.apiKey) == "" {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return time.Now().After(a.degradedUntil)
}

func (a *geminiAssistant) markDegraded(err error) {
	if a == nil || err == nil {
		return
	}
	message := err.Error()
	if !(strings.Contains(message, "HTTP 401") || strings.Contains(message, "HTTP 403") ||
		strings.Contains(message, "HTTP 429") || strings.Contains(message, "HTTP 503")) {
		return
	}
	a.mu.Lock()
	a.degradedUntil = time.Now().Add(geminiDegradedTTL)
	a.mu.Unlock()
}

func (a *App) assistantStatus(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, map[string]bool{
		"enabled":   a.assistant != nil,
		"aiEnabled": a.assistant != nil && a.assistant.aiAvailable(),
	})
}

func (a *App) chatAssistant(w http.ResponseWriter, r *http.Request) {
	if a.assistant == nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Der Shop-Assistent ist derzeit nicht verfügbar."})
		return
	}
	if !a.assistantLimiter.allow(a.clientIP(r)) {
		w.Header().Set("Retry-After", "60")
		respondJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Du hast den Shop-Assistenten gerade oft genutzt. Bitte warte kurz."})
		return
	}

	var input assistantChatInput
	if err := decodeJSON(w, r, &input, maxAssistantRequest); err != nil || !validAssistantMessages(input.Messages) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Bitte sende eine kurze Frage zum Sortiment."})
		return
	}

	var output assistantChatOutput
	var err error
	if a.assistant.aiAvailable() {
		providerContext, providerCancel := context.WithTimeout(r.Context(), 18*time.Second)
		output, err = a.assistant.chat(providerContext, input.Messages)
		providerCancel()
		if err != nil {
			a.assistant.markDegraded(err)
			a.logger.Warn("Gemini shopping assistant request failed; using catalog fallback", "request_id", requestID(r.Context()), "error", err)
		}
	}
	if !a.assistant.aiAvailable() || err != nil {
		catalogContext, catalogCancel := context.WithTimeout(r.Context(), 4*time.Second)
		output, err = a.assistant.catalogFallback(catalogContext, input.Messages)
		catalogCancel()
	}
	if err != nil {
		a.logger.Error("shopping assistant catalog fallback failed", "request_id", requestID(r.Context()), "error", err)
		respondJSON(w, http.StatusBadGateway, map[string]string{"error": "Der Shop-Assistent ist gerade nicht erreichbar. Bitte versuche es erneut."})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, output)
}

func (a *geminiAssistant) catalogFallback(ctx context.Context, messages []assistantMessage) (assistantChatOutput, error) {
	if a.store == nil {
		return assistantChatOutput{}, errors.New("shopping catalog is unavailable")
	}
	intent := parseCatalogIntent(messages)
	switch intent.kind {
	case "greeting":
		return assistantChatOutput{
			Reply:    "Hallo! Ich helfe dir beim Finden und Vergleichen im aktuellen MITTE-Sortiment. Sag mir zum Beispiel eine Produktart, eine Kategorie oder ein Budget.",
			Products: []store.ProductSummary{},
		}, nil
	case "help":
		return assistantChatOutput{
			Reply:    "Du kannst mich zum Beispiel fragen: „Finde kabellose Kopfhörer“, „Was gibt es unter 50 €?“, „Zeig Beauty“ oder „Vergleiche Lampen“. Ich antworte mit aktuellen Preisen und Verfügbarkeit aus dem Sortiment.",
			Products: []store.ProductSummary{},
		}, nil
	case "categories":
		return a.replyWithCategories(ctx)
	case "empty":
		return assistantChatOutput{
			Reply:    "Wonach suchst du? Nenne mir eine Produktart, ein Merkmal, eine Kategorie oder ein Budget — zum Beispiel „Kopfhörer unter 80 Euro“.",
			Products: []store.ProductSummary{},
		}, nil
	}

	intent, err := a.enrichIntentWithCategory(ctx, intent)
	if err != nil {
		return assistantChatOutput{}, fmt.Errorf("match assistant category: %w", err)
	}
	products, err := a.searchCatalogIntent(ctx, intent)
	if err != nil {
		return assistantChatOutput{}, err
	}
	if len(products) == 0 {
		if intent.categorySlug != "" {
			return assistantChatOutput{
				Reply:    "In dieser Kategorie habe ich gerade keine passenden Treffer gefunden. Versuch es mit einem anderen Suchbegriff oder ohne Budgetgrenze.",
				Products: []store.ProductSummary{},
			}, nil
		}
		return assistantChatOutput{
			Reply:    "Dazu habe ich im aktuellen Sortiment keine passenden Produkte gefunden. Formuliere kürzer — zum Beispiel nur die Produktart oder die Kategorie.",
			Products: []store.ProductSummary{},
		}, nil
	}

	reply := catalogReplyFor(intent, products)
	return assistantChatOutput{Reply: reply, Products: products}, nil
}

func (a *geminiAssistant) replyWithCategories(ctx context.Context) (assistantChatOutput, error) {
	categories, err := a.store.Categories(ctx)
	if err != nil {
		return assistantChatOutput{}, fmt.Errorf("load assistant categories: %w", err)
	}
	if len(categories) == 0 {
		return assistantChatOutput{
			Reply:    "Im Sortiment sind gerade keine Kategorien hinterlegt. Schau bitte später noch einmal vorbei.",
			Products: []store.ProductSummary{},
		}, nil
	}
	roots := make([]string, 0, 10)
	for _, category := range categories {
		if category.ParentID != 0 {
			continue
		}
		roots = append(roots, category.Name)
		if len(roots) == 10 {
			break
		}
	}
	if len(roots) == 0 {
		for _, category := range categories[:min(len(categories), 8)] {
			roots = append(roots, category.Name)
		}
	}
	return assistantChatOutput{
		Reply:    "Im MITTE-Sortiment findest du unter anderem: " + strings.Join(roots, ", ") + ". Sag mir eine Kategorie oder ein Produkt, dann zeige ich passende Artikel.",
		Products: []store.ProductSummary{},
	}, nil
}

func (a *geminiAssistant) searchCatalogIntent(ctx context.Context, intent catalogIntent) ([]store.ProductSummary, error) {
	queries := intentSearchQueries(intent)
	products := make([]store.ProductSummary, 0, maxAssistantProducts)
	seen := make(map[string]struct{}, maxAssistantProducts)
	fetchLimit := maxAssistantProducts * 3
	if intent.maxCents > 0 || intent.inStockOnly {
		fetchLimit = 24
	}

	for _, query := range queries {
		matches, _, err := a.store.ProductsPage(ctx, intent.categorySlug, query, intent.kind == "featured", intent.sort, fetchLimit, 0)
		if err != nil {
			return nil, fmt.Errorf("search assistant catalog fallback: %w", err)
		}
		for _, product := range matches {
			if intent.maxCents > 0 {
				price := product.PriceCents
				if product.SalePriceCents > 0 {
					price = product.SalePriceCents
				}
				if price > intent.maxCents {
					continue
				}
			}
			if intent.inStockOnly && product.Inventory <= 0 {
				continue
			}
			if _, exists := seen[product.Slug]; exists {
				continue
			}
			seen[product.Slug] = struct{}{}
			products = append(products, product)
			if len(products) == maxAssistantProducts {
				return products, nil
			}
		}
	}
	return products, nil
}

func intentSearchQueries(intent catalogIntent) []string {
	queries := make([]string, 0, 4)
	seen := make(map[string]struct{}, 4)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			if _, exists := seen[""]; exists {
				return
			}
			seen[""] = struct{}{}
			queries = append(queries, "")
			return
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		queries = append(queries, value)
	}
	if intent.query != "" {
		add(intent.query)
		for _, term := range assistantSearchTerms(intent.query) {
			add(term)
		}
	}
	if intent.categorySlug != "" || intent.kind == "featured" || intent.kind == "bestsellers" || intent.maxCents > 0 {
		add("")
	}
	if len(queries) == 0 {
		add("")
	}
	return queries
}

func catalogReplyFor(intent catalogIntent, products []store.ProductSummary) string {
	names := make([]string, 0, min(len(products), 3))
	for _, product := range products[:min(len(products), 3)] {
		names = append(names, product.Name)
	}
	joined := strings.Join(names, ", ")
	switch intent.kind {
	case "compare":
		return "Hier sind passende Artikel zum Vergleichen — inkl. Preis und Verfügbarkeit. Besonders interessant: " + joined + "."
	case "budget":
		euros := intent.maxCents / 100
		return fmt.Sprintf("Diese Produkte passen zu deinem Budget bis %d €. Beispiele: %s.", euros, joined)
	case "bestsellers":
		return "Das sind aktuell gefragte Artikel aus dem Sortiment: " + joined + "."
	case "featured":
		return "Hier sind empfohlene Highlights aus dem Sortiment: " + joined + "."
	case "category":
		return "In dieser Kategorie könnte das passen: " + joined + ". Öffne eine Karte für Details oder lege direkt in den Warenkorb."
	default:
		return "Diese Produkte aus dem aktuellen MITTE-Sortiment könnten passen: " + joined + ". Öffne eine Produktkarte für Details und Verfügbarkeit."
	}
}

func parseCatalogIntent(messages []assistantMessage) catalogIntent {
	query := strings.TrimSpace(messages[len(messages)-1].Content)
	lower := strings.ToLower(query)
	intent := catalogIntent{kind: "search", query: query, sort: "featured"}

	if isAssistantGreeting(lower) {
		intent.kind = "greeting"
		return intent
	}
	if isAssistantHelp(lower) {
		intent.kind = "help"
		return intent
	}
	if asksForCategories(lower) {
		intent.kind = "categories"
		return intent
	}
	if asksForBestsellers(lower) {
		intent.kind = "bestsellers"
		intent.sort = "bestselling"
		intent.query = strings.TrimSpace(stripIntentNoise(query))
		return intent
	}
	if asksForFeatured(lower) {
		intent.kind = "featured"
		intent.query = strings.TrimSpace(stripIntentNoise(query))
		return intent
	}
	if asksForCompare(lower) {
		intent.kind = "compare"
	}
	if asksForInStock(lower) {
		intent.inStockOnly = true
	}
	if maxCents := parseBudgetCents(lower); maxCents > 0 {
		intent.maxCents = maxCents
		if intent.kind == "search" {
			intent.kind = "budget"
		}
		intent.sort = "price-low"
	}

	cleaned := strings.TrimSpace(stripIntentNoise(query))
	terms := assistantSearchTerms(cleaned)
	if len(terms) == 0 {
		// Pull useful nouns from earlier user turns so follow-ups like "unter 40 Euro" still search.
		for index := len(messages) - 2; index >= 0; index-- {
			if messages[index].Role != "user" {
				continue
			}
			previous := assistantSearchTerms(stripIntentNoise(messages[index].Content))
			if len(previous) == 0 {
				continue
			}
			terms = previous
			cleaned = strings.Join(previous, " ")
			break
		}
	}
	if len(terms) == 0 && intent.maxCents == 0 && intent.kind == "search" {
		intent.kind = "empty"
		return intent
	}
	intent.query = cleaned
	if intent.query == "" && len(terms) > 0 {
		intent.query = strings.Join(terms, " ")
	}
	return intent
}

func (a *geminiAssistant) enrichIntentWithCategory(ctx context.Context, intent catalogIntent) (catalogIntent, error) {
	if intent.categorySlug != "" || a.store == nil {
		return intent, nil
	}
	haystack := strings.ToLower(intent.query)
	if haystack == "" {
		return intent, nil
	}
	categories, err := a.store.Categories(ctx)
	if err != nil {
		return intent, err
	}
	bestLen := 0
	for _, category := range categories {
		name := strings.ToLower(strings.TrimSpace(category.Name))
		slug := strings.ToLower(strings.TrimSpace(category.Slug))
		if name == "" {
			continue
		}
		if strings.Contains(haystack, name) || (slug != "" && strings.Contains(haystack, strings.ReplaceAll(slug, "-", " "))) {
			if len(name) >= bestLen {
				bestLen = len(name)
				intent.categorySlug = category.Slug
				if intent.kind == "search" || intent.kind == "compare" || intent.kind == "budget" {
					intent.kind = "category"
				}
			}
		}
	}
	return intent, nil
}

func parseBudgetCents(query string) int {
	if match := budgetEuroPattern.FindStringSubmatch(query); len(match) == 2 {
		if euros, err := strconv.Atoi(match[1]); err == nil && euros > 0 && euros <= 100000 {
			return euros * 100
		}
	}
	if strings.Contains(query, "günstig") || strings.Contains(query, "billig") || strings.Contains(query, "preiswert") {
		return 5000
	}
	if match := budgetPlainEuro.FindStringSubmatch(query); len(match) == 2 &&
		(strings.Contains(query, "budget") || strings.Contains(query, "maximal") || strings.Contains(query, "max.")) {
		if euros, err := strconv.Atoi(match[1]); err == nil && euros > 0 && euros <= 100000 {
			return euros * 100
		}
	}
	return 0
}

func stripIntentNoise(query string) string {
	replacements := []string{
		"vergleiche", "vergleichen", "vergleich", "bitte", "zeig mir", "zeige mir", "zeig", "zeige",
		"finde", "suche", "such", "empfehle", "empfehlung", "empfehlungen", "brauch", "brauche",
		"möchte", "moechte", "hätte gern", "haette gern", "unter", "bis zu", "bis", "maximal",
		"max", "höchstens", "weniger als", "euro", "eur", "€", "günstig", "guenstig", "billig",
		"preiswert", "verfügbar", "verfuegbar", "auf lager", "bestseller", "meistverkauft",
		"highlights", "empfohlen", "featured", "passende", "produkte", "produkt", "artikel",
	}
	cleaned := strings.ToLower(query)
	for _, token := range replacements {
		cleaned = strings.ReplaceAll(cleaned, token, " ")
	}
	cleaned = budgetEuroPattern.ReplaceAllString(cleaned, " ")
	cleaned = budgetPlainEuro.ReplaceAllString(cleaned, " ")
	return strings.Join(strings.Fields(cleaned), " ")
}

func isAssistantGreeting(query string) bool {
	switch strings.TrimSpace(query) {
	case "hallo", "hi", "hey", "servus", "moin", "guten tag", "guten morgen", "guten abend":
		return true
	default:
		return false
	}
}

func isAssistantHelp(query string) bool {
	return strings.Contains(query, "hilfe") || strings.Contains(query, "was kannst du") ||
		strings.Contains(query, "wie funktioniert") || query == "help"
}

func asksForBestsellers(query string) bool {
	return strings.Contains(query, "bestseller") || strings.Contains(query, "meistverkauft") ||
		strings.Contains(query, "beliebtest") || strings.Contains(query, "am beliebtesten")
}

func asksForFeatured(query string) bool {
	return strings.Contains(query, "highlight") || strings.Contains(query, "empfohlen") ||
		strings.Contains(query, "empfehlung") || strings.Contains(query, "featured")
}

func asksForCompare(query string) bool {
	return strings.Contains(query, "vergleich") || strings.Contains(query, "versus") ||
		strings.Contains(query, " vs ") || strings.Contains(query, "unterschied")
}

func asksForInStock(query string) bool {
	return strings.Contains(query, "verfügbar") || strings.Contains(query, "verfuegbar") ||
		strings.Contains(query, "auf lager") || strings.Contains(query, "lieferbar")
}

func assistantSearchTerms(query string) []string {
	stopWords := map[string]struct{}{
		"aber": {}, "als": {}, "am": {}, "an": {}, "auch": {}, "auf": {}, "aus": {},
		"bitte": {}, "brauche": {}, "das": {}, "dass": {}, "dem": {}, "den": {}, "der": {},
		"des": {}, "die": {}, "ein": {}, "eine": {}, "einer": {}, "eines": {}, "einen": {},
		"er": {}, "es": {}, "etwas": {}, "finde": {}, "fuer": {}, "für": {}, "gibt": {},
		"haben": {}, "ich": {}, "im": {}, "in": {}, "ist": {}, "kann": {}, "koennt": {},
		"könnt": {}, "mir": {}, "mit": {}, "moechte": {}, "möchte": {}, "nach": {},
		"mein": {}, "meine": {}, "meiner": {}, "meines": {},
		"oder": {}, "passende": {}, "passt": {}, "produkt": {}, "produkte": {}, "such": {},
		"suche": {}, "und": {}, "was": {}, "welche": {}, "welcher": {}, "welches": {},
		"vergleiche": {}, "vergleichen": {}, "vergleich": {}, "zeige": {}, "zeig": {},
		"zum": {}, "zur": {}, "hier": {}, "dort": {}, "noch": {}, "mal": {}, "gern": {},
		"gerne": {}, "mehr": {}, "ohne": {}, "sehr": {}, "ganz": {}, "echt": {},
		"richtig": {}, "aktuell": {}, "aktuellen": {},
	}
	terms := make([]string, 0, 5)
	seen := make(map[string]struct{}, 5)
	for _, term := range strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !('a' <= r && r <= 'z' || '0' <= r && r <= '9' || r == 'ä' || r == 'ö' || r == 'ü' || r == 'ß')
	}) {
		if len([]rune(term)) < 3 {
			continue
		}
		if _, exists := stopWords[term]; exists {
			continue
		}
		if _, exists := seen[term]; exists {
			continue
		}
		seen[term] = struct{}{}
		terms = append(terms, term)
		if len(terms) == 5 {
			break
		}
	}
	return terms
}

func asksForCategories(query string) bool {
	query = strings.ToLower(query)
	if strings.Contains(query, "kategorie") || strings.Contains(query, "sortiment") {
		return true
	}
	if !strings.Contains(query, "was gibt es") {
		return false
	}
	if parseBudgetCents(query) > 0 {
		return false
	}
	for _, term := range assistantSearchTerms(stripIntentNoise(query)) {
		switch term {
		case "mitte", "shop", "store", "bei", "hier":
			continue
		default:
			return false
		}
	}
	return true
}

func validAssistantMessages(messages []assistantMessage) bool {
	if len(messages) == 0 || len(messages) > maxAssistantMessages || messages[len(messages)-1].Role != "user" {
		return false
	}
	total := 0
	for _, message := range messages {
		if message.Role != "user" && message.Role != "assistant" {
			return false
		}
		content := strings.TrimSpace(message.Content)
		if content == "" || !utf8.ValidString(content) || utf8.RuneCountInString(content) > maxAssistantMessage {
			return false
		}
		total += len(content)
		if total > 9000 {
			return false
		}
	}
	return true
}

func (a *geminiAssistant) chat(ctx context.Context, messages []assistantMessage) (assistantChatOutput, error) {
	if a.store == nil {
		return assistantChatOutput{}, errors.New("shopping catalog is unavailable")
	}
	contents := make([]geminiContent, 0, len(messages)+4)
	for _, message := range messages {
		role := message.Role
		if role == "assistant" {
			role = "model"
		}
		contents = append(contents, geminiContent{Role: role, Parts: []geminiPart{{Text: message.Content}}})
	}

	output := assistantChatOutput{Products: make([]store.ProductSummary, 0)}
	productIndexes := make(map[string]int)
	for attempt := 0; attempt < maxGeminiToolRounds; attempt++ {
		response, err := a.generate(ctx, contents)
		if err != nil {
			return assistantChatOutput{}, err
		}
		if len(response.Candidates) == 0 {
			return assistantChatOutput{}, errors.New("Gemini returned no response candidates")
		}
		candidate := response.Candidates[0].Content
		calls := make([]geminiFunctionCall, 0)
		var text strings.Builder
		for _, part := range candidate.Parts {
			if part.Text != "" {
				text.WriteString(part.Text)
			}
			if part.FunctionCall != nil {
				calls = append(calls, *part.FunctionCall)
			}
		}
		if len(calls) == 0 {
			output.Reply = strings.TrimSpace(text.String())
			if output.Reply == "" || utf8.RuneCountInString(output.Reply) > 4000 {
				return assistantChatOutput{}, errors.New("Gemini returned an invalid response")
			}
			return output, nil
		}
		if attempt == maxGeminiToolRounds-1 {
			return assistantChatOutput{}, errors.New("Gemini exceeded the shopping tool-call limit")
		}

		// Preserve the model turn exactly, including thought signatures required by Gemini 3.x.
		contents = append(contents, candidate)
		toolResults := geminiContent{Role: "user", Parts: make([]geminiPart, 0, len(calls))}
		for _, call := range calls {
			result, found, err := a.runTool(ctx, call)
			if err != nil {
				return assistantChatOutput{}, err
			}
			for _, product := range found {
				if index, exists := productIndexes[product.Slug]; exists {
					output.Products[index] = product
					continue
				}
				if len(output.Products) < maxAssistantProducts {
					productIndexes[product.Slug] = len(output.Products)
					output.Products = append(output.Products, product)
				}
			}
			toolResults.Parts = append(toolResults.Parts, geminiPart{FunctionResponse: &geminiFunctionResponse{
				Name: call.Name, ID: call.ID, Response: map[string]any{"result": result},
			}})
		}
		contents = append(contents, toolResults)
	}
	return assistantChatOutput{}, errors.New("Gemini did not return a final response")
}

func (a *geminiAssistant) generate(ctx context.Context, contents []geminiContent) (geminiGenerateResponse, error) {
	requestBody := map[string]any{
		"systemInstruction": map[string]any{"parts": []map[string]string{{"text": geminiSystemPrompt}}},
		"contents":          contents,
		"tools":             []any{geminiShoppingTools},
		"toolConfig":        map[string]any{"functionCallingConfig": map[string]string{"mode": "AUTO"}},
		"generationConfig": map[string]any{
			"maxOutputTokens": 900,
			"thinkingConfig":  map[string]any{"thinkingLevel": "low"},
		},
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return geminiGenerateResponse{}, fmt.Errorf("encode Gemini request: %w", err)
	}
	endpoint := strings.TrimRight(a.endpoint, "/") + "/v1beta/models/" + url.PathEscape(a.model) + ":generateContent"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return geminiGenerateResponse{}, fmt.Errorf("create Gemini request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("x-goog-api-key", a.apiKey)
	response, err := a.httpClient.Do(request)
	if err != nil {
		return geminiGenerateResponse{}, fmt.Errorf("send Gemini request: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return geminiGenerateResponse{}, fmt.Errorf("read Gemini response: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return geminiGenerateResponse{}, fmt.Errorf("Gemini returned HTTP %d", response.StatusCode)
	}
	var result geminiGenerateResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return geminiGenerateResponse{}, fmt.Errorf("decode Gemini response: %w", err)
	}
	return result, nil
}

const geminiSystemPrompt = `You are MITTE's German-language shopping assistant. Help shoppers find and compare products in the live MITTE catalog. Use search_catalog before naming, recommending, comparing, pricing, or stating availability for products. Use get_product_details when technical specifications or an exact comparison are requested. Use list_categories to help shoppers browse.
Only state product names, prices, stock, and specifications present in tool results. Prices are gross EUR amounts in cents; convert them accurately to euros. Inventory above zero means available; zero means unavailable. If the catalog does not support a claim or no matching products are found, say so plainly. Never invent discounts, reviews, delivery dates, guarantees, or product facts. Ask a concise clarification when the shopper's constraints are unclear.
Be proactive: suggest 2-4 concrete next filters (budget, material, category, use case) when results are broad. Prefer currently available products when the shopper does not ask for unavailable items.
Do not request or repeat payment details, passwords, or other sensitive information. You cannot place orders or change the cart; shoppers can choose a product card to view or add it themselves. Keep replies helpful, concise, and in German.`

var geminiShoppingTools = map[string]any{
	"functionDeclarations": []map[string]any{
		{
			"name":        "search_catalog",
			"description": "Search active MITTE products by name or description and return live prices, stock and product cards.",
			"parameters": map[string]any{
				"type": "OBJECT", "required": []string{"query"},
				"properties": map[string]any{
					"query":        map[string]string{"type": "STRING", "description": "Short product search, up to 100 characters"},
					"categorySlug": map[string]string{"type": "STRING", "description": "Optional exact category slug"},
					"sort":         map[string]any{"type": "STRING", "enum": []string{"featured", "price-low", "bestselling"}},
				},
			},
		},
		{
			"name":        "get_product_details",
			"description": "Get current description, specifications, gross price and inventory for one active product slug.",
			"parameters": map[string]any{
				"type": "OBJECT", "required": []string{"slug"},
				"properties": map[string]any{"slug": map[string]string{"type": "STRING", "description": "Exact product slug returned by catalog search"}},
			},
		},
		{
			"name":        "list_categories",
			"description": "List the current MITTE category tree for browsing.",
			"parameters":  map[string]any{"type": "OBJECT", "properties": map[string]any{}},
		},
	},
}

func (a *geminiAssistant) runTool(ctx context.Context, call geminiFunctionCall) (map[string]any, []store.ProductSummary, error) {
	switch call.Name {
	case "search_catalog":
		var input struct {
			Query        string `json:"query"`
			CategorySlug string `json:"categorySlug"`
			Sort         string `json:"sort"`
		}
		if err := decodeToolArgs(call.Args, &input); err != nil || strings.TrimSpace(input.Query) == "" || utf8.RuneCountInString(input.Query) > 100 || len(input.CategorySlug) > 64 {
			return map[string]any{"error": "Ungültige Produktsuche."}, nil, nil
		}
		sortOrder := input.Sort
		if sortOrder == "" {
			sortOrder = "featured"
		}
		if !slices.Contains([]string{"featured", "price-low", "bestselling"}, sortOrder) {
			sortOrder = "featured"
		}
		if input.CategorySlug != "" {
			categories, err := a.store.Categories(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("load categories for assistant: %w", err)
			}
			found := false
			for _, category := range categories {
				found = found || category.Slug == input.CategorySlug
			}
			if !found {
				return map[string]any{"products": []store.ProductSummary{}}, nil, nil
			}
		}
		products, _, err := a.store.ProductsPage(ctx, input.CategorySlug, strings.TrimSpace(input.Query), false, sortOrder, 6, 0)
		if err != nil {
			return nil, nil, fmt.Errorf("search assistant catalog: %w", err)
		}
		return map[string]any{"products": products}, products, nil
	case "get_product_details":
		var input struct {
			Slug string `json:"slug"`
		}
		if err := decodeToolArgs(call.Args, &input); err != nil || input.Slug == "" || len(input.Slug) > 120 {
			return map[string]any{"error": "Ungültiger Produktlink."}, nil, nil
		}
		product, err := a.store.ProductBySlug(ctx, input.Slug)
		if errors.Is(err, store.ErrNotFound) {
			return map[string]any{"error": "Produkt nicht gefunden."}, nil, nil
		}
		if err != nil {
			return nil, nil, fmt.Errorf("load assistant product details: %w", err)
		}
		summary := store.ProductSummary{
			ID: product.ID, Slug: product.Slug, Name: product.Name, ShortDescription: product.ShortDescription,
			PriceCents: product.PriceCents, PriceGrossCents: product.PriceCents, ImageURL: product.ImageURL,
			ImageAlt: product.ImageAlt, Inventory: product.Inventory, Category: product.Category,
		}
		details := map[string]any{
			"name": product.Name, "slug": product.Slug, "description": product.Description,
			"shortDescription": product.ShortDescription, "priceGrossCents": product.PriceCents,
			"inventory": product.Inventory, "category": product.Category.Name, "manufacturer": product.Manufacturer,
			"material": product.Material, "dimensions": product.Dimensions, "weightGrams": product.WeightGrams,
			"technicalDetails": product.TechnicalDetails,
		}
		return map[string]any{"product": details}, []store.ProductSummary{summary}, nil
	case "list_categories":
		categories, err := a.store.Categories(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("load assistant categories: %w", err)
		}
		if len(categories) > 80 {
			categories = categories[:80]
		}
		return map[string]any{"categories": categories}, nil, nil
	default:
		return map[string]any{"error": "Unbekanntes Shop-Werkzeug."}, nil, nil
	}
}

func decodeToolArgs(arguments map[string]any, destination any) error {
	data, err := json.Marshal(arguments)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(destination)
}
