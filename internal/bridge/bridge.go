package bridge

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/meganerd/kz-bridge/internal/config"
	"github.com/meganerd/kz-bridge/internal/karakeep"
	"github.com/meganerd/kz-bridge/internal/translator"
	"github.com/meganerd/kz-bridge/internal/zotero"
)

// Stats tracks sync progress.
type Stats struct {
	Total    int
	Synced   int
	Skipped  int
	Enriched int
	Failed   int
}

// Bridge orchestrates the Karakeep -> Zotero sync pipeline.
type Bridge struct {
	karakeep   *karakeep.Client
	translator *translator.Client
	zotero     *zotero.Client
	cfg        *config.Config
	log        *slog.Logger
}

// New creates a new Bridge.
func New(cfg *config.Config, log *slog.Logger) *Bridge {
	return &Bridge{
		karakeep:   karakeep.NewClient(cfg.Karakeep.URL, cfg.Karakeep.Token),
		translator: translator.NewClient(cfg.Translator.URL),
		zotero:     zotero.NewClient(cfg.Zotero.UserID, cfg.Zotero.APIKey),
		cfg:        cfg,
		log:        log,
	}
}

// Sync runs the full pipeline.
func (b *Bridge) Sync() (*Stats, error) {
	stats := &Stats{}

	// Step 1: Fetch all bookmarks from Karakeep
	b.log.Info("fetching bookmarks from Karakeep")
	bookmarks, err := b.karakeep.AllBookmarks()
	if err != nil {
		return stats, fmt.Errorf("fetching bookmarks: %w", err)
	}
	stats.Total = len(bookmarks)
	b.log.Info("fetched bookmarks", "count", stats.Total)

	// Filter to link-type bookmarks only
	var links []karakeep.Bookmark
	for _, bm := range bookmarks {
		if bm.Content.Type != "link" {
			stats.Skipped++
			continue
		}
		if b.cfg.Since != "" {
			since, err := time.Parse("2006-01-02", b.cfg.Since)
			if err != nil {
				return stats, fmt.Errorf("parsing --since date: %w", err)
			}
			if bm.CreatedAt.Before(since) {
				stats.Skipped++
				continue
			}
		}
		links = append(links, bm)
	}
	b.log.Info("link bookmarks to process", "count", len(links), "skipped", stats.Skipped)

	// Step 2: Fetch existing Zotero URLs for dedup
	b.log.Info("fetching existing Zotero URLs for deduplication")
	existingURLs, err := b.zotero.ExistingURLs()
	if err != nil {
		return stats, fmt.Errorf("fetching existing URLs: %w", err)
	}
	b.log.Info("existing Zotero URLs", "count", len(existingURLs))

	// Step 3: Build Zotero items
	var items []zotero.Item
	for _, bm := range links {
		if existingURLs[bm.Content.URL] {
			b.log.Debug("skipping duplicate", "url", bm.Content.URL)
			stats.Skipped++
			continue
		}

		item := b.buildItem(bm)

		// Try to enrich via Translation Server
		enriched, err := b.translator.Translate(bm.Content.URL)
		if err != nil {
			b.log.Warn("translation failed, using basic metadata", "url", bm.Content.URL, "error", err)
		} else if enriched != nil {
			item = b.mergeEnriched(item, enriched)
			stats.Enriched++
		}

		items = append(items, item)
	}

	b.log.Info("items to sync", "count", len(items), "enriched", stats.Enriched)

	if b.cfg.DryRun {
		for _, item := range items {
			b.log.Info("[dry-run] would create", "title", item.Title, "url", item.URL, "tags", len(item.Tags))
		}
		stats.Synced = len(items)
		return stats, nil
	}

	// Step 4: Create items in Zotero
	if len(items) > 0 {
		b.log.Info("creating items in Zotero", "count", len(items))
		created, failed, err := b.zotero.CreateItems(items)
		if err != nil {
			return stats, fmt.Errorf("creating items: %w", err)
		}
		stats.Synced = created
		stats.Failed = failed
	}

	return stats, nil
}

func (b *Bridge) buildItem(bm karakeep.Bookmark) zotero.Item {
	item := zotero.Item{
		ItemType:   "webpage",
		Title:      bm.Title,
		URL:        bm.Content.URL,
		AccessDate: bm.CreatedAt.Format("2006-01-02"),
		Tags:       b.mapTags(bm),
	}

	if bm.Description != "" {
		item.AbstractNote = bm.Description
	}

	if b.cfg.Collection != "" {
		item.Collections = []string{b.cfg.Collection}
	}

	return item
}

func (b *Bridge) mapTags(bm karakeep.Bookmark) []zotero.Tag {
	var tags []zotero.Tag

	for _, t := range bm.Tags {
		if !b.cfg.IncludeAITags {
			// Skip tags that look AI-generated (heuristic: we include all for now,
			// since Karakeep doesn't distinguish AI vs manual tags in the API)
		}
		tags = append(tags, zotero.Tag{Tag: t.Name})
	}

	// Add synthetic tags for metadata
	tags = append(tags, zotero.Tag{Tag: "karakeep-import"})
	if bm.Favourited {
		tags = append(tags, zotero.Tag{Tag: "starred"})
	}

	return tags
}

func (b *Bridge) mergeEnriched(base zotero.Item, enriched *translator.Item) zotero.Item {
	// Prefer enriched metadata where available
	if enriched.Title != "" {
		base.Title = enriched.Title
	}
	if enriched.Abstract != "" {
		base.AbstractNote = enriched.Abstract
	}
	if enriched.WebsiteTitle != "" {
		base.WebsiteTitle = enriched.WebsiteTitle
	}
	if enriched.Date != "" {
		base.Date = enriched.Date
	}
	if enriched.Language != "" {
		base.Language = enriched.Language
	}

	// Map creators
	for _, c := range enriched.Creators {
		base.Creators = append(base.Creators, zotero.Creator{
			CreatorType: c.CreatorType,
			FirstName:   c.FirstName,
			LastName:    c.LastName,
			Name:        c.Name,
		})
	}

	return base
}
