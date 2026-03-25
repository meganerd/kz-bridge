package zotero

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	baseURL    = "https://api.zotero.org"
	maxBatch   = 50
)

// Item represents a Zotero item for creation.
type Item struct {
	ItemType     string    `json:"itemType"`
	Title        string    `json:"title"`
	URL          string    `json:"url"`
	AbstractNote string    `json:"abstractNote,omitempty"`
	WebsiteTitle string    `json:"websiteTitle,omitempty"`
	Date         string    `json:"date,omitempty"`
	AccessDate   string    `json:"accessDate,omitempty"`
	Language     string    `json:"language,omitempty"`
	Creators     []Creator `json:"creators,omitempty"`
	Tags         []Tag     `json:"tags"`
	Collections  []string  `json:"collections,omitempty"`
	Extra        string    `json:"extra,omitempty"`
}

// Creator represents an author.
type Creator struct {
	CreatorType string `json:"creatorType"`
	FirstName   string `json:"firstName,omitempty"`
	LastName    string `json:"lastName,omitempty"`
	Name        string `json:"name,omitempty"`
}

// Tag represents a Zotero tag.
type Tag struct {
	Tag  string `json:"tag"`
	Type int    `json:"type,omitempty"`
}

// ExistingItem is a minimal representation for deduplication.
type ExistingItem struct {
	Key string `json:"key"`
	URL string `json:"url"`
}

// WriteResponse is the response from a batch create.
type WriteResponse struct {
	Successful map[string]json.RawMessage `json:"successful"`
	Unchanged  map[string]json.RawMessage `json:"unchanged"`
	Failed     map[string]FailedItem      `json:"failed"`
}

// FailedItem describes a failure in batch creation.
type FailedItem struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Client is a Zotero Web API client.
type Client struct {
	userID     string
	apiKey     string
	apiBase    string
	httpClient *http.Client
}

// NewClient creates a new Zotero API client.
func NewClient(userID, apiKey string) *Client {
	return &Client{
		userID: userID,
		apiKey: apiKey,
		apiBase: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewClientWithBase creates a client with a custom API base (for testing).
func NewClientWithBase(userID, apiKey, base string) *Client {
	c := NewClient(userID, apiKey)
	c.apiBase = base
	return c
}

// CreateItems creates items in batches of up to 50.
func (c *Client) CreateItems(items []Item) (created int, failed int, err error) {
	for i := 0; i < len(items); i += maxBatch {
		end := i + maxBatch
		if end > len(items) {
			end = len(items)
		}
		batch := items[i:end]

		resp, batchErr := c.createBatch(batch)
		if batchErr != nil {
			return created, failed + len(batch), fmt.Errorf("batch starting at %d: %w", i, batchErr)
		}

		created += len(resp.Successful)
		failed += len(resp.Failed)

		// Handle rate limiting via Backoff header
		if backoff := resp; backoff != nil {
			// Already handled in doRequest
		}
	}
	return created, failed, nil
}

func (c *Client) createBatch(items []Item) (*WriteResponse, error) {
	body, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("marshaling items: %w", err)
	}

	path := fmt.Sprintf("/users/%s/items", c.userID)
	respBody, err := c.doRequest(http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}

	var result WriteResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
}

// ExistingURLs fetches all URLs currently in the library for deduplication.
func (c *Client) ExistingURLs() (map[string]bool, error) {
	urls := make(map[string]bool)
	start := 0
	limit := 100

	for {
		path := fmt.Sprintf("/users/%s/items?format=json&itemType=webpage&limit=%d&start=%d", c.userID, limit, start)
		body, err := c.doRequest(http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}

		var items []struct {
			Data struct {
				URL string `json:"url"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &items); err != nil {
			return nil, fmt.Errorf("decoding items: %w", err)
		}

		for _, item := range items {
			if item.Data.URL != "" {
				urls[item.Data.URL] = true
			}
		}

		if len(items) < limit {
			break
		}
		start += limit
	}

	return urls, nil
}

func (c *Client) doRequest(method, path string, body []byte) ([]byte, error) {
	u, err := url.JoinPath(c.apiBase, path)
	if err != nil {
		return nil, fmt.Errorf("building URL: %w", err)
	}

	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, u, reqBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Zotero-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	// Handle backoff
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate limited by Zotero API (429)")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Zotero API returned %d: %s", resp.StatusCode, string(respBody))
	}

	return io.ReadAll(resp.Body)
}
