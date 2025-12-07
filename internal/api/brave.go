package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/quocvuong92/azure-ai-cli/internal/config"
)

const BraveAPIURL = "https://api.search.brave.com/res/v1/web/search"

// BraveResponse represents the Brave search response
type BraveResponse struct {
	Web BraveWebResults `json:"web"`
}

// BraveWebResults contains the web search results
type BraveWebResults struct {
	Results []BraveResult `json:"results"`
}

// BraveResult represents a single search result
type BraveResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

// BraveClient is the Brave Search API client
type BraveClient struct {
	httpClient    *http.Client
	config        *config.Config
	onKeyRotation KeyRotationCallback
}

// Ensure BraveClient implements SearchClient
var _ SearchClient = (*BraveClient)(nil)

// NewBraveClient creates a new Brave Search client
func NewBraveClient(cfg *config.Config) *BraveClient {
	return &BraveClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		config: cfg,
	}
}

// SetKeyRotationCallback sets a callback function for key rotation events
func (c *BraveClient) SetKeyRotationCallback(callback func(fromIndex, toIndex, totalKeys int)) {
	c.onKeyRotation = callback
}

// Search performs a web search using Brave Search (implements SearchClient interface)
func (c *BraveClient) Search(ctx context.Context, query string) (*SearchResponse, error) {
	resp, err := c.searchWithRetry(ctx, query)
	if err != nil {
		return nil, err
	}
	return resp.ToSearchResponse(), nil
}

// SearchLegacy performs a web search using Brave Search (legacy method for backward compatibility)
func (c *BraveClient) SearchLegacy(query string) (*BraveResponse, error) {
	return c.searchWithRetry(context.Background(), query)
}

// searchWithRetry performs search with automatic key rotation on failure
func (c *BraveClient) searchWithRetry(ctx context.Context, query string) (*BraveResponse, error) {
	if c.config.GetBraveKeyCount() <= 1 {
		return c.doSearch(ctx, query)
	}

	var lastErr error
	for attempt := 0; attempt < MaxRetryAttempts; attempt++ {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("search cancelled: %w", err)
		}

		resp, err := c.doSearch(ctx, query)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		apiErr, ok := err.(*APIError)
		if !ok || !ShouldRotateKey(apiErr.StatusCode) {
			return nil, err
		}

		if rotateErr := c.rotateKey(); rotateErr != nil {
			return nil, fmt.Errorf("%v (no more Brave API keys available)", err)
		}

		// Apply backoff before retry
		if attempt < MaxRetryAttempts-1 {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("search cancelled: %w", ctx.Err())
			case <-time.After(CalculateBackoff(attempt)):
			}
		}
	}

	return nil, fmt.Errorf("max retry attempts (%d) exceeded: %v", MaxRetryAttempts, lastErr)
}

// doSearch performs a single search attempt
func (c *BraveClient) doSearch(ctx context.Context, query string) (*BraveResponse, error) {
	// Build URL with query parameters
	reqURL, err := url.Parse(BraveAPIURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	params := url.Values{}
	params.Set("q", query)
	params.Set("count", "5")
	reqURL.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", c.config.BraveAPIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("Brave API error: status code %d", resp.StatusCode),
		}
	}

	var braveResp BraveResponse
	if err := json.Unmarshal(body, &braveResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &braveResp, nil
}

// rotateKey attempts to switch to the next available API key
func (c *BraveClient) rotateKey() error {
	oldIndex := c.config.BraveCurrentKeyIdx
	_, err := c.config.RotateBraveKey()
	if err != nil {
		return err
	}

	if c.onKeyRotation != nil {
		c.onKeyRotation(oldIndex+1, c.config.BraveCurrentKeyIdx+1, c.config.GetBraveKeyCount())
	}

	return nil
}

// ToSearchResponse converts BraveResponse to unified SearchResponse
func (r *BraveResponse) ToSearchResponse() *SearchResponse {
	results := make([]SearchResult, len(r.Web.Results))
	for i, res := range r.Web.Results {
		results[i] = SearchResult{
			Title:   res.Title,
			URL:     res.URL,
			Content: res.Description,
		}
	}
	return &SearchResponse{
		Results: results,
	}
}

// FormatResultsAsContext formats search results for use as LLM context
func (r *BraveResponse) FormatResultsAsContext() string {
	if len(r.Web.Results) == 0 {
		return ""
	}

	var result string
	for i, res := range r.Web.Results {
		result += fmt.Sprintf("[%d] %s\nURL: %s\n%s\n\n", i+1, res.Title, res.URL, res.Description)
	}
	return result
}

// ToTavilyResponse converts BraveResponse to TavilyResponse for compatibility
func (r *BraveResponse) ToTavilyResponse() *TavilyResponse {
	results := make([]TavilyResult, len(r.Web.Results))
	for i, res := range r.Web.Results {
		results[i] = TavilyResult{
			Title:   res.Title,
			URL:     res.URL,
			Content: res.Description,
		}
	}
	return &TavilyResponse{
		Results: results,
	}
}
