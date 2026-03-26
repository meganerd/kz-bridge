package bridge

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/meganerd/kz-bridge/internal/config"
	"github.com/meganerd/kz-bridge/internal/karakeep"
	"github.com/meganerd/kz-bridge/internal/translator"
	"github.com/meganerd/kz-bridge/internal/zotero"
)

func TestBuildItem(t *testing.T) {
	cfg := &config.Config{
		IncludeAITags: true,
		Collection:    "ABC123",
	}
	b := &Bridge{cfg: cfg, log: slog.Default()}

	bm := karakeep.Bookmark{
		Title:       "Test Article",
		Description: "A test description",
		Favourited:  true,
		Content: karakeep.Content{
			Type: "link",
			URL:  "https://example.com/article",
		},
		Tags: []karakeep.Tag{
			{Name: "research"},
			{Name: "ai"},
		},
	}

	item := b.buildItem(bm)

	if item.ItemType != "webpage" {
		t.Errorf("itemType = %q", item.ItemType)
	}
	if item.Title != "Test Article" {
		t.Errorf("title = %q", item.Title)
	}
	if item.URL != "https://example.com/article" {
		t.Errorf("url = %q", item.URL)
	}
	if item.AbstractNote != "A test description" {
		t.Errorf("abstractNote = %q", item.AbstractNote)
	}
	if len(item.Collections) != 1 || item.Collections[0] != "ABC123" {
		t.Errorf("collections = %v", item.Collections)
	}

	// Should have: research, ai, karakeep-import, starred
	if len(item.Tags) != 4 {
		t.Errorf("expected 4 tags, got %d: %v", len(item.Tags), item.Tags)
	}

	tagNames := make(map[string]bool)
	for _, tag := range item.Tags {
		tagNames[tag.Tag] = true
	}
	for _, expected := range []string{"research", "ai", "karakeep-import", "starred"} {
		if !tagNames[expected] {
			t.Errorf("missing tag %q", expected)
		}
	}
}

func TestMergeEnriched(t *testing.T) {
	b := &Bridge{cfg: &config.Config{}, log: slog.Default()}

	base := zotero.Item{
		ItemType: "webpage",
		Title:    "Original Title",
		URL:      "https://example.com",
		Tags:     []zotero.Tag{{Tag: "test"}},
	}

	enriched := &translator.Item{
		Title:        "Better Title From Translator",
		Abstract:     "Detailed abstract.",
		WebsiteTitle: "Example News",
		Date:         "2025-06-15",
		Language:     "en",
		Creators: []translator.Creator{
			{CreatorType: "author", FirstName: "Jane", LastName: "Doe"},
		},
	}

	merged := b.mergeEnriched(base, enriched)

	if merged.Title != "Better Title From Translator" {
		t.Errorf("title = %q", merged.Title)
	}
	if merged.AbstractNote != "Detailed abstract." {
		t.Errorf("abstractNote = %q", merged.AbstractNote)
	}
	if merged.WebsiteTitle != "Example News" {
		t.Errorf("websiteTitle = %q", merged.WebsiteTitle)
	}
	if merged.Date != "2025-06-15" {
		t.Errorf("date = %q", merged.Date)
	}
	if merged.Language != "en" {
		t.Errorf("language = %q", merged.Language)
	}
	if len(merged.Creators) != 1 || merged.Creators[0].LastName != "Doe" {
		t.Errorf("creators = %v", merged.Creators)
	}
	// Original tags preserved
	if len(merged.Tags) != 1 || merged.Tags[0].Tag != "test" {
		t.Errorf("tags = %v", merged.Tags)
	}
}

func TestSyncEndToEnd(t *testing.T) {
	// Mock Karakeep
	kkServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := karakeep.ListResponse{
			Bookmarks: []karakeep.Bookmark{
				{
					ID:    "1",
					Title: "Article One",
					Content: karakeep.Content{
						Type: "link",
						URL:  "https://example.com/one",
					},
					Tags: []karakeep.Tag{{Name: "tag1"}},
				},
				{
					ID:    "2",
					Title: "Note",
					Content: karakeep.Content{
						Type: "text",
						Text: "just a note",
					},
				},
			},
			NextCursor: "",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer kkServer.Close()

	// Mock Translation Server
	tsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		items := []translator.Item{
			{
				Title:        "Enriched: " + string(body),
				WebsiteTitle: "Example Site",
			},
		}
		json.NewEncoder(w).Encode(items)
	}))
	defer tsServer.Close()

	// Mock Zotero
	var createdItems []zotero.Item
	zotServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// ExistingURLs — return empty
			json.NewEncoder(w).Encode([]struct{}{})
			return
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &createdItems)

		resp := zotero.WriteResponse{
			Successful: make(map[string]json.RawMessage),
			Failed:     make(map[string]zotero.FailedItem),
		}
		for i := range createdItems {
			resp.Successful[fmt.Sprintf("%d", i)] = json.RawMessage(`{}`)
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer zotServer.Close()

	cfg := &config.Config{
		Karakeep:      config.KarakeepConfig{URL: kkServer.URL, Token: "test"},
		Zotero:        config.ZoteroConfig{UserID: "123", APIKey: "key"},
		Translator:    config.TranslatorConfig{URL: tsServer.URL},
		IncludeAITags: true,
	}

	b := &Bridge{
		karakeep:   karakeep.NewClient(cfg.Karakeep.URL, cfg.Karakeep.Token),
		translator: translator.NewClient(cfg.Translator.URL),
		zotero:     zotero.NewClientWithBase(cfg.Zotero.UserID, cfg.Zotero.APIKey, zotServer.URL),
		cfg:        cfg,
		log:        slog.Default(),
	}

	stats, err := b.Sync()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.Total != 2 {
		t.Errorf("total = %d, want 2", stats.Total)
	}
	if stats.Skipped != 1 {
		t.Errorf("skipped = %d, want 1 (text bookmark)", stats.Skipped)
	}
	if stats.Synced != 1 {
		t.Errorf("synced = %d, want 1", stats.Synced)
	}
	if stats.Enriched != 1 {
		t.Errorf("enriched = %d, want 1", stats.Enriched)
	}
}

func TestSyncDryRun(t *testing.T) {
	kkServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := karakeep.ListResponse{
			Bookmarks: []karakeep.Bookmark{
				{
					ID:    "1",
					Title: "Article",
					Content: karakeep.Content{
						Type: "link",
						URL:  "https://example.com",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer kkServer.Close()

	tsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(501)
	}))
	defer tsServer.Close()

	zotServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode([]struct{}{})
			return
		}
		t.Error("should not POST in dry-run mode")
	}))
	defer zotServer.Close()

	cfg := &config.Config{
		Karakeep:      config.KarakeepConfig{URL: kkServer.URL, Token: "test"},
		Zotero:        config.ZoteroConfig{UserID: "123", APIKey: "key"},
		Translator:    config.TranslatorConfig{URL: tsServer.URL},
		DryRun:        true,
		IncludeAITags: true,
	}

	b := &Bridge{
		karakeep:   karakeep.NewClient(cfg.Karakeep.URL, cfg.Karakeep.Token),
		translator: translator.NewClient(cfg.Translator.URL),
		zotero:     zotero.NewClientWithBase(cfg.Zotero.UserID, cfg.Zotero.APIKey, zotServer.URL),
		cfg:        cfg,
		log:        slog.Default(),
	}

	stats, err := b.Sync()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.Synced != 1 {
		t.Errorf("synced = %d, want 1 (dry-run counts planned)", stats.Synced)
	}
}
