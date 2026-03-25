package karakeep

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Bookmark represents a Karakeep bookmark.
type Bookmark struct {
	ID          string    `json:"id"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Archived    bool      `json:"archived"`
	Favourited  bool      `json:"favourited"`
	Tags        []Tag     `json:"tags"`
	Content     Content   `json:"content"`
}

// Tag represents a Karakeep tag.
type Tag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Content is the polymorphic content of a bookmark.
type Content struct {
	Type        string `json:"type"`
	URL         string `json:"url,omitempty"`
	Text        string `json:"text,omitempty"`
	CrawlStatus string `json:"crawlStatus,omitempty"`
}

// ListResponse is the paginated response from GET /bookmarks.
type ListResponse struct {
	Bookmarks  []Bookmark `json:"bookmarks"`
	NextCursor string     `json:"nextCursor"`
}

// Client is a Karakeep API client.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient creates a new Karakeep API client.
func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ListBookmarks fetches one page of bookmarks.
func (c *Client) ListBookmarks(cursor string, limit int) (*ListResponse, error) {
	u, err := url.Parse(c.baseURL + "/api/v1/bookmarks")
	if err != nil {
		return nil, fmt.Errorf("parsing URL: %w", err)
	}

	q := u.Query()
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	if limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", limit))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	var result ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
}

// AllBookmarks fetches all bookmarks by paginating through cursor results.
func (c *Client) AllBookmarks() ([]Bookmark, error) {
	var all []Bookmark
	cursor := ""

	for {
		page, err := c.ListBookmarks(cursor, 100)
		if err != nil {
			return nil, err
		}

		all = append(all, page.Bookmarks...)

		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}

	return all, nil
}
