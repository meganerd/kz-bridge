package translator

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Item represents an enriched Zotero item from the Translation Server.
type Item struct {
	ItemType    string   `json:"itemType"`
	Title       string   `json:"title"`
	URL         string   `json:"url"`
	Abstract    string   `json:"abstractNote"`
	Date        string   `json:"date"`
	WebsiteTitle string  `json:"websiteTitle"`
	Language    string   `json:"language"`
	AccessDate  string   `json:"accessDate"`
	Creators    []Creator `json:"creators"`
	Tags        []ItemTag `json:"tags"`
}

// Creator represents an author or contributor.
type Creator struct {
	CreatorType string `json:"creatorType"`
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	Name        string `json:"name,omitempty"`
}

// ItemTag represents a tag from the Translation Server.
type ItemTag struct {
	Tag string `json:"tag"`
}

// Client is a Zotero Translation Server client.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new Translation Server client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Translate sends a URL to the /web endpoint and returns enriched metadata.
// Returns nil (no error) if the server cannot translate the URL (501, 300).
func (c *Client) Translate(rawURL string) (*Item, error) {
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/web", strings.NewReader(rawURL))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// Success — parse the array and return the first item
	case 300:
		// Multiple choices — we can't pick automatically, skip
		return nil, nil
	case 501:
		// No translator available for this URL
		return nil, nil
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("translation server returned %d: %s", resp.StatusCode, string(body))
	}

	var items []Item
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if len(items) == 0 {
		return nil, nil
	}

	return &items[0], nil
}
