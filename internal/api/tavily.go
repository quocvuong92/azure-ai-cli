package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/quocvuong92/azure-ai-cli/internal/config"
)

const TavilyAPIURL = "https://api.tavily.com/search"

// TavilyRequest represents the Tavily search request
type TavilyRequest struct {
	APIKey      string `json:"api_key"`
	Query       string `json:"query"`
	SearchDepth string `json:"search_depth"`
	MaxResults  int    `json:"max_results"`
}

// TavilyResponse represents the Tavily search response
type TavilyResponse struct {
	Results []TavilyResult `json:"results"`
	Answer  string         `json:"answer,omitempty"`
}

// TavilyResult represents a single search result
type TavilyResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

// TavilyErrorResponse represents an error from Tavily
type TavilyErrorResponse struct {
	Detail string `json:"detail"`
}

// TavilyClient is the Tavily search API client
type TavilyClient struct {
	httpClient    *http.Client
	config        *config.Config
	onKeyRotation func(fromIndex, toIndex, totalKeys int)
}

// NewTavilyClient creates a new Tavily client
func NewTavilyClient(cfg *config.Config) *TavilyClient {
	return &TavilyClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		config: cfg,
	}
}

// SetKeyRotationCallback sets a callback function for key rotation events
func (c *TavilyClient) SetKeyRotationCallback(callback func(fromIndex, toIndex, totalKeys int)) {
	c.onKeyRotation = callback
}

// Search performs a web search using Tavily
func (c *TavilyClient) Search(query string) (*TavilyResponse, error) {
	return c.searchWithRetry(query)
}

// searchWithRetry performs search with automatic key rotation on failure
func (c *TavilyClient) searchWithRetry(query string) (*TavilyResponse, error) {
	if c.config.GetTavilyKeyCount() <= 1 {
		return c.doSearch(query)
	}

	for {
		resp, err := c.doSearch(query)
		if err == nil {
			return resp, nil
		}

		apiErr, ok := err.(*APIError)
		if !ok || !c.shouldRotateKey(apiErr.StatusCode) {
			return nil, err
		}

		if rotateErr := c.rotateKey(); rotateErr != nil {
			return nil, fmt.Errorf("%v (no more Tavily API keys available)", err)
		}
	}
}

// doSearch performs a single search attempt
func (c *TavilyClient) doSearch(query string) (*TavilyResponse, error) {
	reqBody := TavilyRequest{
		APIKey:      c.config.TavilyAPIKey,
		Query:       query,
		SearchDepth: "basic",
		MaxResults:  5,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, TavilyAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

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
		var errResp TavilyErrorResponse
		errMsg := fmt.Sprintf("status code %d", resp.StatusCode)
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Detail != "" {
			errMsg = errResp.Detail
		}
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("Tavily API error: %s", errMsg),
		}
	}

	var tavilyResp TavilyResponse
	if err := json.Unmarshal(body, &tavilyResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &tavilyResp, nil
}

// shouldRotateKey checks if the error indicates we should try another key
func (c *TavilyClient) shouldRotateKey(statusCode int) bool {
	for _, code := range config.RotatableErrorCodes {
		if statusCode == code {
			return true
		}
	}
	return false
}

// rotateKey attempts to switch to the next available API key
func (c *TavilyClient) rotateKey() error {
	oldIndex := c.config.TavilyCurrentKeyIdx
	_, err := c.config.RotateTavilyKey()
	if err != nil {
		return err
	}

	if c.onKeyRotation != nil {
		c.onKeyRotation(oldIndex+1, c.config.TavilyCurrentKeyIdx+1, c.config.GetTavilyKeyCount())
	}

	return nil
}

// FormatResultsAsContext formats search results for use as LLM context
func (r *TavilyResponse) FormatResultsAsContext() string {
	if len(r.Results) == 0 {
		return ""
	}

	var result string
	for i, res := range r.Results {
		result += fmt.Sprintf("[%d] %s\nURL: %s\n%s\n\n", i+1, res.Title, res.URL, res.Content)
	}
	return result
}
