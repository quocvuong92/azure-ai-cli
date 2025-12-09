package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/quocvuong92/azure-ai-cli/internal/api"
	"github.com/quocvuong92/azure-ai-cli/internal/display"
	"github.com/quocvuong92/azure-ai-cli/internal/executor"
)

func (app *App) optimizeSearchQuery(query string, messages []api.Message, client *api.AzureClient) (string, error) {
	// Build messages for query optimization
	// Include conversation history so LLM understands context
	optimizeMessages := []api.Message{
		{
			Role:    "system",
			Content: QueryOptimizationPrompt,
		},
	}

	// Add conversation history (skip original system message, limit to last N messages)
	startIdx := 1 // Skip system message
	if len(messages) > MaxHistoryMessagesForOptimization+1 {
		startIdx = len(messages) - MaxHistoryMessagesForOptimization
	}

	for i := startIdx; i < len(messages); i++ {
		msg := messages[i]
		// Truncate long assistant responses to save tokens
		if msg.Role == "assistant" && len(msg.Content) > MaxMessageLengthForOptimization {
			optimizeMessages = append(optimizeMessages, api.Message{
				Role:    msg.Role,
				Content: msg.Content[:MaxMessageLengthForOptimization] + "...",
			})
		} else {
			optimizeMessages = append(optimizeMessages, msg)
		}
	}

	// Add the current query as the final user message
	optimizeMessages = append(optimizeMessages, api.Message{
		Role:    "user",
		Content: fmt.Sprintf("Generate a search query for: %s", query),
	})

	sp := display.NewSpinner("Optimizing query...")
	sp.Start()

	resp, err := client.QueryWithHistory(optimizeMessages)
	sp.Stop()

	if err != nil {
		return "", err
	}

	optimizedQuery := strings.TrimSpace(resp.GetContent())
	// Remove quotes if the LLM wrapped the query in them
	optimizedQuery = strings.Trim(optimizedQuery, "\"'`")

	return optimizedQuery, nil
}

func (app *App) handleWebSearch(query string, messages *[]api.Message, client *api.AzureClient, exec *executor.Executor) {
	// Optimize search query using LLM if there's conversation context
	optimizedQuery := query
	if len(*messages) > 1 { // More than just system message
		var err error
		optimizedQuery, err = app.optimizeSearchQuery(query, *messages, client)
		if err != nil {
			// Fall back to original query if optimization fails
			log.Printf("Query optimization failed: %v, using original query", err)
			optimizedQuery = query
		} else if optimizedQuery != query {
			// Show the optimized query so user knows what was searched
			fmt.Fprintf(os.Stderr, "Searching for: %s\n", optimizedQuery)
		}
	}

	// Perform web search with optimized query
	searchContext, err := app.performWebSearch(optimizedQuery)
	if err != nil {
		display.ShowError(err.Error())
		return
	}

	// Add web search results as a system context message, then add user query
	// This preserves conversation flow while providing web context
	webContextMsg := api.Message{
		Role:    "system",
		Content: fmt.Sprintf(WebContextMessageTemplate, searchContext),
	}

	// Add web context to messages temporarily
	*messages = append(*messages, webContextMsg)
	*messages = append(*messages, api.Message{Role: "user", Content: query})

	// Send request with tools support
	fmt.Println()
	response, err := app.sendInteractiveMessageWithTools(client, exec, messages)
	if err != nil {
		display.ShowError(err.Error())
		// Remove the messages we added on error
		*messages = (*messages)[:len(*messages)-2]
		return
	}

	// Remove web context from history, keep only user query and response
	// Remove web context message (second to last before we added response)
	*messages = append((*messages)[:len(*messages)-3], (*messages)[len(*messages)-2:]...)

	if response != "" {
		// Response is already added by sendInteractiveMessageWithTools
	} else {
		// If no response, add it manually
		*messages = append(*messages, api.Message{Role: "assistant", Content: response})
	}

	// Show citations if enabled
	if app.cfg.Citations && app.searchResults != nil && len(app.searchResults.Results) > 0 {
		fmt.Println()
		citations := make([]display.Citation, len(app.searchResults.Results))
		for i, r := range app.searchResults.Results {
			citations[i] = display.Citation{Title: r.Title, URL: r.URL}
		}
		display.ShowCitations(citations)
	}
	fmt.Println()
}

func (app *App) performWebSearch(query string) (string, error) {
	sp := display.NewSpinner("Searching web...")
	sp.Start()

	ctx := context.Background()
	var results *api.TavilyResponse

	switch app.cfg.WebSearchProvider {
	case "linkup":
		linkupClient := api.NewLinkupClient(app.cfg)
		linkupClient.SetKeyRotationCallback(func(from, to, total int) {
			display.ShowKeyRotation("Linkup", from, to, total)
		})

		searchResp, searchErr := linkupClient.Search(ctx, query)
		if searchErr != nil {
			sp.Stop()
			return "", searchErr
		}
		results = searchResp.ToTavilyResponse()

	case "brave":
		braveClient := api.NewBraveClient(app.cfg)
		braveClient.SetKeyRotationCallback(func(from, to, total int) {
			display.ShowKeyRotation("Brave", from, to, total)
		})

		searchResp, searchErr := braveClient.Search(ctx, query)
		if searchErr != nil {
			sp.Stop()
			return "", searchErr
		}
		results = searchResp.ToTavilyResponse()

	default: // tavily
		tavilyClient := api.NewTavilyClient(app.cfg)
		tavilyClient.SetKeyRotationCallback(func(from, to, total int) {
			display.ShowKeyRotation("Tavily", from, to, total)
		})

		searchResp, searchErr := tavilyClient.Search(ctx, query)
		if searchErr != nil {
			sp.Stop()
			return "", searchErr
		}
		results = searchResp.ToTavilyResponse()
	}

	sp.Stop()

	// Store results for citations
	app.searchResults = results

	return results.FormatResultsAsContext(), nil
}

func buildWebSearchPrompt(searchContext string) string {
	return fmt.Sprintf(WebSearchPromptTemplate, searchContext)
}
