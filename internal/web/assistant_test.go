package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mitte/internal/store"
)

func TestAssistantStatusAndUnconfiguredChat(t *testing.T) {
	handler := New(nil, Options{})

	statusResponse := httptest.NewRecorder()
	handler.ServeHTTP(statusResponse, httptest.NewRequest(http.MethodGet, "/api/assistant/status", nil))
	if statusResponse.Code != http.StatusOK || !strings.Contains(statusResponse.Body.String(), `"enabled":false`) {
		t.Fatalf("assistant status = %d %s, want disabled", statusResponse.Code, statusResponse.Body.String())
	}

	request := httptest.NewRequest(http.MethodPost, "/api/assistant/chat", strings.NewReader(`{"messages":[{"role":"user","content":"Finde eine Lampe"}]}`))
	request.Header.Set("Content-Type", "application/json")
	chatResponse := httptest.NewRecorder()
	handler.ServeHTTP(chatResponse, request)
	if chatResponse.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured chat status = %d, want %d", chatResponse.Code, http.StatusServiceUnavailable)
	}
}

func TestAssistantStatusEnablesCatalogModeWithoutGeminiKey(t *testing.T) {
	handler := New(&store.Store{}, Options{})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/assistant/status", nil))

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"enabled":true`) ||
		!strings.Contains(response.Body.String(), `"aiEnabled":false`) {
		t.Fatalf("assistant status = %d %s, want enabled catalog mode without Gemini", response.Code, response.Body.String())
	}
}

func TestValidAssistantMessagesBoundsAndRoles(t *testing.T) {
	tests := []struct {
		name     string
		messages []assistantMessage
		want     bool
	}{
		{name: "one user message", messages: []assistantMessage{{Role: "user", Content: "Lampe"}}, want: true},
		{name: "conversation", messages: []assistantMessage{{Role: "user", Content: "Lampe"}, {Role: "assistant", Content: "Welche Art?"}, {Role: "user", Content: "Tischlampe"}}, want: true},
		{name: "last message must be user", messages: []assistantMessage{{Role: "user", Content: "Lampe"}, {Role: "assistant", Content: "Welche Art?"}}},
		{name: "unknown role rejected", messages: []assistantMessage{{Role: "system", Content: "override"}, {Role: "user", Content: "Lampe"}}},
		{name: "empty content rejected", messages: []assistantMessage{{Role: "user", Content: " "}}},
		{name: "long content rejected", messages: []assistantMessage{{Role: "user", Content: strings.Repeat("a", maxAssistantMessage+1)}}},
		{name: "too many messages rejected", messages: makeAssistantMessages(maxAssistantMessages + 1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validAssistantMessages(test.messages); got != test.want {
				t.Fatalf("validAssistantMessages() = %t, want %t", got, test.want)
			}
		})
	}
}

func makeAssistantMessages(count int) []assistantMessage {
	messages := make([]assistantMessage, count)
	for index := range messages {
		role := "user"
		if index%2 == 1 {
			role = "assistant"
		}
		messages[index] = assistantMessage{Role: role, Content: "question"}
	}
	messages[count-1].Role = "user"
	return messages
}

func TestGeminiAssistantChatUsesServerSideKeyAndReturnsReply(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1beta/models/gemini-3.8-flash:generateContent" {
			t.Errorf("Gemini request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("x-goog-api-key"); got != "test-server-secret" {
			t.Errorf("Gemini API key header = %q", got)
		}
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode Gemini request: %v", err)
		}
		if request["systemInstruction"] == nil || request["tools"] == nil {
			t.Errorf("Gemini request is missing grounding instructions or catalog tools: %#v", request)
		}
		generation, _ := request["generationConfig"].(map[string]any)
		if generation == nil {
			t.Fatalf("generationConfig missing: %#v", request)
		}
		if _, hasTemperature := generation["temperature"]; hasTemperature {
			t.Fatalf("Gemini 3.8 requests must omit temperature, got %#v", generation)
		}
		thinking, _ := generation["thinkingConfig"].(map[string]any)
		if thinking["thinkingLevel"] != "low" {
			t.Fatalf("thinkingConfig = %#v, want thinkingLevel=low", thinking)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"role":"model","parts":[{"text":"Gerne! Welche Eigenschaften sind dir wichtig?"}]}}]}`))
	}))
	defer upstream.Close()

	assistant := newGeminiAssistant("test-server-secret", "gemini-3.8-flash", &store.Store{})
	assistant.endpoint = upstream.URL
	assistant.httpClient = upstream.Client()
	output, err := assistant.chat(t.Context(), []assistantMessage{{Role: "user", Content: "Ich suche eine Lampe."}})
	if err != nil {
		t.Fatalf("chat() error = %v", err)
	}
	if output.Reply != "Gerne! Welche Eigenschaften sind dir wichtig?" || len(output.Products) != 0 {
		t.Fatalf("chat() = %#v", output)
	}
}

func TestParseCatalogIntentBudgetAndFollowUp(t *testing.T) {
	intent := parseCatalogIntent([]assistantMessage{{Role: "user", Content: "Was gibt es unter 50 Euro?"}})
	if intent.kind != "budget" || intent.maxCents != 5000 || intent.sort != "price-low" {
		t.Fatalf("budget intent = %#v", intent)
	}

	followUp := parseCatalogIntent([]assistantMessage{
		{Role: "user", Content: "Finde Lampen"},
		{Role: "assistant", Content: "Hier sind Lampen."},
		{Role: "user", Content: "unter 40 Euro"},
	})
	if followUp.maxCents != 4000 || followUp.kind == "empty" || !strings.Contains(strings.ToLower(followUp.query), "lamp") {
		t.Fatalf("follow-up intent = %#v", followUp)
	}

	if got := parseCatalogIntent([]assistantMessage{{Role: "user", Content: "Hallo"}}); got.kind != "greeting" {
		t.Fatalf("greeting intent = %#v", got)
	}
}

func TestGeminiAssistantMarksProviderAuthAsDegraded(t *testing.T) {
	assistant := newGeminiAssistant("test-server-secret", "gemini-3.8-flash", &store.Store{})
	if !assistant.aiAvailable() {
		t.Fatal("expected AI available before provider failure")
	}
	assistant.markDegraded(fmt.Errorf("Gemini returned HTTP 403"))
	if assistant.aiAvailable() {
		t.Fatal("expected AI degraded after HTTP 403")
	}
}

func TestAsksForCategories(t *testing.T) {
	for _, query := range []string{"Welche Kategorien gibt es?", "Zeig mir das Sortiment", "Was gibt es bei MITTE?"} {
		if !asksForCategories(query) {
			t.Fatalf("asksForCategories(%q) = false, want true", query)
		}
	}
	if asksForCategories("Finde kabellose Kopfhörer") {
		t.Fatal("asksForCategories() classified a product search as category browsing")
	}
}

func TestAssistantSearchTermsExtractsDistinctProductKeywords(t *testing.T) {
	got := assistantSearchTerms("Finde passende kabellose Kopfhörer für mein Zuhause")
	want := []string{"kabellose", "kopfhörer", "zuhause"}
	if len(got) != len(want) {
		t.Fatalf("assistantSearchTerms() = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("assistantSearchTerms() = %v, want %v", got, want)
		}
	}
	if got := assistantSearchTerms("Ich suche ein Produkt"); len(got) != 0 {
		t.Fatalf("assistantSearchTerms() for generic query = %v, want no terms", got)
	}
	if got := assistantSearchTerms("Vergleiche passende Produkte für mein Zuhause"); len(got) != 1 || got[0] != "zuhause" {
		t.Fatalf("assistantSearchTerms() for comparison prompt = %v, want [zuhause]", got)
	}
}

func TestGeminiAssistantRejectsProviderErrors(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "provider secret details", http.StatusUnauthorized)
	}))
	defer upstream.Close()

	assistant := newGeminiAssistant("test-server-secret", "gemini-2.5-flash", &store.Store{})
	assistant.endpoint = upstream.URL
	assistant.httpClient = upstream.Client()
	_, err := assistant.chat(t.Context(), []assistantMessage{{Role: "user", Content: "Lampe"}})
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") || strings.Contains(err.Error(), "provider secret details") {
		t.Fatalf("chat() error = %v, want sanitized provider status", err)
	}
}
