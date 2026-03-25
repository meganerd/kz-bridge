package karakeep

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestListBookmarks(t *testing.T) {
	now := time.Now().Truncate(time.Second)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/bookmarks" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("auth header = %q", got)
		}

		resp := ListResponse{
			Bookmarks: []Bookmark{
				{
					ID:        "bm-1",
					CreatedAt: now,
					Title:     "Test Bookmark",
					Tags:      []Tag{{ID: "t1", Name: "research"}},
					Content: Content{
						Type: "link",
						URL:  "https://example.com/paper",
					},
				},
			},
			NextCursor: "",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	result, err := client.ListBookmarks("", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Bookmarks) != 1 {
		t.Fatalf("expected 1 bookmark, got %d", len(result.Bookmarks))
	}

	bm := result.Bookmarks[0]
	if bm.Title != "Test Bookmark" {
		t.Errorf("title = %q", bm.Title)
	}
	if bm.Content.URL != "https://example.com/paper" {
		t.Errorf("url = %q", bm.Content.URL)
	}
	if len(bm.Tags) != 1 || bm.Tags[0].Name != "research" {
		t.Errorf("tags = %v", bm.Tags)
	}
}

func TestAllBookmarksPagination(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		cursor := r.URL.Query().Get("cursor")

		var resp ListResponse
		switch cursor {
		case "":
			resp = ListResponse{
				Bookmarks:  []Bookmark{{ID: "1", Title: "First"}},
				NextCursor: "page2",
			}
		case "page2":
			resp = ListResponse{
				Bookmarks:  []Bookmark{{ID: "2", Title: "Second"}},
				NextCursor: "",
			}
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "token")
	all, err := client.AllBookmarks()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(all) != 2 {
		t.Fatalf("expected 2 bookmarks, got %d", len(all))
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls, got %d", callCount)
	}
}

func TestListBookmarksAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "invalid token"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "bad-token")
	_, err := client.ListBookmarks("", 0)
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}
