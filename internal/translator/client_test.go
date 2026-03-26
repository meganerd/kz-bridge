package translator

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTranslateSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/web" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		items := []Item{
			{
				ItemType:     "webpage",
				Title:        "Enriched Title",
				URL:          "https://example.com/article",
				Abstract:     "This is the abstract.",
				WebsiteTitle: "Example News",
				Date:         "2025-01-15",
				Creators: []Creator{
					{CreatorType: "author", FirstName: "Jane", LastName: "Doe"},
				},
			},
		}
		json.NewEncoder(w).Encode(items)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	item, err := client.Translate("https://example.com/article")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item == nil {
		t.Fatal("expected item, got nil")
	}
	if item.Title != "Enriched Title" {
		t.Errorf("title = %q", item.Title)
	}
	if item.WebsiteTitle != "Example News" {
		t.Errorf("websiteTitle = %q", item.WebsiteTitle)
	}
	if len(item.Creators) != 1 || item.Creators[0].LastName != "Doe" {
		t.Errorf("creators = %v", item.Creators)
	}
}

func TestTranslateNoTranslator(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(501)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	item, err := client.Translate("https://example.com/nope")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item != nil {
		t.Errorf("expected nil for 501, got %+v", item)
	}
}

func TestTranslateMultipleChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(300)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	item, err := client.Translate("https://example.com/multi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item != nil {
		t.Errorf("expected nil for 300, got %+v", item)
	}
}

func TestTranslateServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Translate("https://example.com/error")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}
