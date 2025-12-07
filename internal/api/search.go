package api

import (
	"context"
	"fmt"
)

// SearchResult represents a unified search result across all providers
type SearchResult struct {
	Title   string
	URL     string
	Content string
	Score   float64
}

// SearchResponse represents a unified search response across all providers
type SearchResponse struct {
	Results []SearchResult
	Answer  string // Optional answer from some providers
}

// FormatResultsAsContext formats search results for use as LLM context
func (r *SearchResponse) FormatResultsAsContext() string {
	if len(r.Results) == 0 {
		return ""
	}

	var result string
	for i, res := range r.Results {
		result += fmt.Sprintf("[%d] %s\nURL: %s\n%s\n\n", i+1, res.Title, res.URL, res.Content)
	}
	return result
}

// ToTavilyResponse converts SearchResponse to TavilyResponse for backward compatibility
func (r *SearchResponse) ToTavilyResponse() *TavilyResponse {
	results := make([]TavilyResult, len(r.Results))
	for i, res := range r.Results {
		results[i] = TavilyResult{
			Title:   res.Title,
			URL:     res.URL,
			Content: res.Content,
			Score:   res.Score,
		}
	}
	return &TavilyResponse{
		Results: results,
		Answer:  r.Answer,
	}
}

// SearchClient defines the interface for web search providers
type SearchClient interface {
	// Search performs a web search with the given query
	Search(ctx context.Context, query string) (*SearchResponse, error)

	// SetKeyRotationCallback sets a callback function for key rotation events
	SetKeyRotationCallback(callback func(fromIndex, toIndex, totalKeys int))
}

// KeyRotationCallback is the function signature for key rotation notifications
type KeyRotationCallback func(fromIndex, toIndex, totalKeys int)
