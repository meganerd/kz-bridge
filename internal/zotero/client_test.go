package zotero

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateItems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Zotero-API-Key") != "test-key" {
			t.Errorf("missing API key header")
		}

		body, _ := io.ReadAll(r.Body)
		var items []Item
		json.Unmarshal(body, &items)

		resp := WriteResponse{
			Successful: make(map[string]json.RawMessage),
			Failed:     make(map[string]FailedItem),
		}
		for i := range items {
			key := fmt.Sprintf("%d", i)
			resp.Successful[key] = json.RawMessage(`{}`)
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClientWithBase("12345", "test-key", server.URL)
	items := []Item{
		{ItemType: "webpage", Title: "Test", URL: "https://example.com", Tags: []Tag{{Tag: "test"}}},
	}

	created, failed, err := client.CreateItems(items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created != 1 {
		t.Errorf("created = %d", created)
	}
	if failed != 0 {
		t.Errorf("failed = %d", failed)
	}
}

func TestCreateItemsBatching(t *testing.T) {
	batchCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		batchCount++
		body, _ := io.ReadAll(r.Body)
		var items []Item
		json.Unmarshal(body, &items)

		if len(items) > maxBatch {
			t.Errorf("batch too large: %d items", len(items))
		}

		resp := WriteResponse{
			Successful: make(map[string]json.RawMessage),
			Failed:     make(map[string]FailedItem),
		}
		for i := range items {
			resp.Successful[fmt.Sprintf("%d", i)] = json.RawMessage(`{}`)
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClientWithBase("12345", "test-key", server.URL)

	// Create 75 items — should result in 2 batches (50 + 25)
	items := make([]Item, 75)
	for i := range items {
		items[i] = Item{ItemType: "webpage", Title: fmt.Sprintf("Item %d", i), Tags: []Tag{}}
	}

	created, failed, err := client.CreateItems(items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created != 75 {
		t.Errorf("created = %d, want 75", created)
	}
	if failed != 0 {
		t.Errorf("failed = %d", failed)
	}
	if batchCount != 2 {
		t.Errorf("batchCount = %d, want 2", batchCount)
	}
}

func TestExistingURLs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		items := []struct {
			Data struct {
				URL string `json:"url"`
			} `json:"data"`
		}{
			{Data: struct {
				URL string `json:"url"`
			}{URL: "https://example.com/existing"}},
		}
		json.NewEncoder(w).Encode(items)
	}))
	defer server.Close()

	client := NewClientWithBase("12345", "test-key", server.URL)
	urls, err := client.ExistingURLs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !urls["https://example.com/existing"] {
		t.Error("expected URL to be in set")
	}
	if urls["https://example.com/missing"] {
		t.Error("unexpected URL in set")
	}
}

func TestRateLimiting(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := NewClientWithBase("12345", "test-key", server.URL)
	_, _, err := client.CreateItems([]Item{{ItemType: "webpage", Tags: []Tag{}}})
	if err == nil {
		t.Fatal("expected error for 429")
	}
}
