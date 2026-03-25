package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.IncludeAITags {
		t.Error("expected IncludeAITags default to be true")
	}
	if cfg.Translator.URL != "http://localhost:1969" {
		t.Errorf("expected default translator URL, got %q", cfg.Translator.URL)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `
karakeep:
  url: https://karakeep.example.com
  token: kk-test-token
zotero:
  user_id: "12345"
  api_key: zot-test-key
translator:
  url: http://translator:1969
include_ai_tags: false
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Karakeep.URL != "https://karakeep.example.com" {
		t.Errorf("karakeep URL = %q", cfg.Karakeep.URL)
	}
	if cfg.Karakeep.Token != "kk-test-token" {
		t.Errorf("karakeep token = %q", cfg.Karakeep.Token)
	}
	if cfg.Zotero.UserID != "12345" {
		t.Errorf("zotero user_id = %q", cfg.Zotero.UserID)
	}
	if cfg.Zotero.APIKey != "zot-test-key" {
		t.Errorf("zotero api_key = %q", cfg.Zotero.APIKey)
	}
	if cfg.Translator.URL != "http://translator:1969" {
		t.Errorf("translator URL = %q", cfg.Translator.URL)
	}
	if cfg.IncludeAITags {
		t.Error("expected IncludeAITags to be false")
	}
}
