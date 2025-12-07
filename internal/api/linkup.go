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

const LinkupAPIURL = "https://api.linkup.so/v1/search"

// LinkupRequest represents the Linkup search request
type LinkupRequest struct {
	Query      string `json:"q"`
	Depth      string `json:"depth"`
	OutputType string `json:"outputType"`
	MaxResults int    `json:"maxResults,omitempty"`
}

// LinkupResponse represents the Linkup search response
type LinkupResponse struct {
	Results []LinkupResult `json:"results"`
}

// LinkupResult represents a single search result
type LinkupResult struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

// LinkupErrorResponse represents an error from Linkup
type LinkupErrorResponse struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

// LinkupClient is the Linkup search API client
type LinkupClient struct {
	httpClient    *http.Client
	config        *config.Config
	onKeyRotation func(fromIndex, toIndex, totalKeys int)
}

// NewLinkupClient creates a new Linkup client
func NewLinkupClient(cfg *config.Config) *LinkupClient {
	return &LinkupClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		config: cfg,
	}
}

// SetKeyRotationCallback sets a callback function for key rotation events
func (c *LinkupClient) SetKeyRotationCallback(callback func(fromIndex, toIndex, totalKeys int)) {
	c.onKeyRotation = callback
}

// Search performs a web search using Linkup
func (c *LinkupClient) Search(query string) (*LinkupResponse, error) {
	return c.searchWithRetry(query)
}

// searchWithRetry performs search with automatic key rotation on failure
func (c *LinkupClient) searchWithRetry(query string) (*LinkupResponse, error) {
	if c.config.GetLinkupKeyCount() <= 1 {
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
			return nil, fmt.Errorf("%v (no more Linkup API keys available)", err)
		}
	}
}

// doSearch performs a single search attempt
func (c *LinkupClient) doSearch(query string) (*LinkupResponse, error) {
	reqBody := LinkupRequest{
		Query:      query,
		Depth:      "standard",
		OutputType: "searchResults",
		MaxResults: 5,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, LinkupAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.LinkupAPIKey)

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
		var errResp LinkupErrorResponse
		errMsg := fmt.Sprintf("status code %d", resp.StatusCode)
		if err := json.Unmarshal(body, &errResp); err == nil {
			if errResp.Message != "" {
				errMsg = errResp.Message
			} else if errResp.Error != "" {
				errMsg = errResp.Error
			}
		}
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("Linkup API error: %s", errMsg),
		}
	}

	var linkupResp LinkupResponse
	if err := json.Unmarshal(body, &linkupResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &linkupResp, nil
}

// shouldRotateKey checks if the error indicates we should try another key
func (c *LinkupClient) shouldRotateKey(statusCode int) bool {
	for _, code := range config.RotatableErrorCodes {
		if statusCode == code {
			return true
		}
	}
	return false
}

// rotateKey attempts to switch to the next available API key
func (c *LinkupClient) rotateKey() error {
	oldIndex := c.config.LinkupCurrentKeyIdx
	_, err := c.config.RotateLinkupKey()
	if err != nil {
		return err
	}

	if c.onKeyRotation != nil {
		c.onKeyRotation(oldIndex+1, c.config.LinkupCurrentKeyIdx+1, c.config.GetLinkupKeyCount())
	}

	return nil
}

// FormatResultsAsContext formats search results for use as LLM context
func (r *LinkupResponse) FormatResultsAsContext() string {
	if len(r.Results) == 0 {
		return ""
	}

	var result string
	for i, res := range r.Results {
		result += fmt.Sprintf("[%d] %s\nURL: %s\n%s\n\n", i+1, res.Name, res.URL, res.Content)
	}
	return result
}

// ToTavilyResponse converts LinkupResponse to TavilyResponse for compatibility
func (r *LinkupResponse) ToTavilyResponse() *TavilyResponse {
	results := make([]TavilyResult, len(r.Results))
	for i, res := range r.Results {
		results[i] = TavilyResult{
			Title:   res.Name,
			URL:     res.URL,
			Content: res.Content,
		}
	}
	return &TavilyResponse{
		Results: results,
	}
}
