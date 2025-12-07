package cmd

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/chzyer/readline"
	"github.com/spf13/cobra"

	"github.com/quocvuong92/azure-ai-cli/internal/api"
	"github.com/quocvuong92/azure-ai-cli/internal/config"
	"github.com/quocvuong92/azure-ai-cli/internal/display"
)

var (
	cfg           *config.Config
	verbose       bool
	listModels    bool
	searchResults *api.TavilyResponse // Store search results for citations
)

// Command items for autocomplete
var commandItems = []readline.PrefixCompleterInterface{
	readline.PcItem("/exit"),
	readline.PcItem("/quit"),
	readline.PcItem("/q"),
	readline.PcItem("/clear"),
	readline.PcItem("/c"),
	readline.PcItem("/help"),
	readline.PcItem("/h"),
	readline.PcItem("/web",
		readline.PcItem("on"),
		readline.PcItem("off"),
		readline.PcItem("tavily"),
		readline.PcItem("linkup"),
		readline.PcItem("brave"),
	),
	readline.PcItem("/model"),
}

var rootCmd = &cobra.Command{
	Use:   "azure-ai [query]",
	Short: "A CLI client for Azure OpenAI with web search",
	Long: `Azure AI CLI is a command-line client for Azure OpenAI API,
with optional web search powered by Tavily, Linkup, or Brave.

Supports multiple API keys with automatic rotation for free tier usage.

Examples:
  azure-ai "What is Kubernetes?"
  azure-ai -m gpt-4o "Explain Docker"
  azure-ai --web "Latest news on Go 1.24"
  azure-ai --web --provider brave "Latest AI news"
  azure-ai -i                             # Interactive mode
  azure-ai -ir                            # Interactive with markdown rendering`,
	Args: cobra.MaximumNArgs(1),
	Run:  run,
}

func init() {
	cfg = config.NewConfig()

	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable debug mode")
	rootCmd.Flags().BoolVarP(&cfg.Usage, "usage", "u", false, "Show token usage statistics")
	rootCmd.Flags().BoolVarP(&cfg.Stream, "stream", "s", false, "Stream output in real-time")
	rootCmd.Flags().BoolVarP(&cfg.Render, "render", "r", false, "Render markdown with colors and formatting")
	rootCmd.Flags().BoolVarP(&cfg.WebSearch, "web", "w", false, "Search web first (requires TAVILY_API_KEYS, LINKUP_API_KEYS, or BRAVE_API_KEYS)")
	rootCmd.Flags().BoolVarP(&cfg.Citations, "citations", "c", false, "Show citations/sources from web search")
	rootCmd.Flags().BoolVarP(&cfg.Interactive, "interactive", "i", false, "Interactive chat mode")
	rootCmd.Flags().StringVarP(&cfg.Model, "model", "m", "", "Model/deployment name (defaults to first in AZURE_OPENAI_MODELS)")
	rootCmd.Flags().StringVarP(&cfg.WebSearchProvider, "provider", "p", "", "Web search provider: tavily, linkup, or brave (default: auto-detect)")
	rootCmd.Flags().BoolVar(&listModels, "list-models", false, "List available models")
}

func run(cmd *cobra.Command, args []string) {
	if verbose {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	} else {
		log.SetOutput(io.Discard)
	}

	// Handle --list-models flag
	if listModels {
		_ = cfg.Validate()
		if len(cfg.AvailableModels) == 0 {
			fmt.Println("No models configured. Set AZURE_OPENAI_MODELS environment variable.")
			fmt.Println("Example: export AZURE_OPENAI_MODELS=gpt-4o,gpt-35-turbo")
			os.Exit(1)
		}
		display.ShowModels(cfg.AvailableModels, cfg.Model)
		return
	}

	// Validate config
	if err := cfg.Validate(); err != nil {
		display.ShowError(err.Error())
		os.Exit(1)
	}

	// Initialize markdown renderer if render flag is set
	if cfg.Render {
		if err := display.InitRenderer(); err != nil {
			log.Printf("Failed to initialize renderer: %v", err)
		}
	}

	// Interactive mode
	if cfg.Interactive {
		runInteractive()
		return
	}

	// Require query if not interactive mode
	if len(args) == 0 {
		_ = cmd.Help()
		os.Exit(1)
	}

	query := args[0]
	log.Printf("Query: %s", query)
	log.Printf("Model: %s", cfg.Model)
	log.Printf("Stream: %v", cfg.Stream)
	log.Printf("WebSearch: %v", cfg.WebSearch)

	// Build system prompt and user message
	systemPrompt := config.DefaultSystemMessage
	userMessage := query

	// Web search if requested
	if cfg.WebSearch {
		searchContext, err := performWebSearch(query)
		if err != nil {
			display.ShowError(err.Error())
			os.Exit(1)
		}
		systemPrompt = buildWebSearchPrompt(searchContext)
	}

	// Create Azure client
	azureClient := api.NewAzureClient(cfg)

	log.Printf("Sending request to Azure OpenAI...")

	if cfg.Stream {
		runStream(azureClient, systemPrompt, userMessage)
	} else {
		runNormal(azureClient, systemPrompt, userMessage)
	}

	// Show citations if web search was used and citations flag is set
	if cfg.WebSearch && cfg.Citations && searchResults != nil && len(searchResults.Results) > 0 {
		fmt.Println()
		citations := make([]display.Citation, len(searchResults.Results))
		for i, r := range searchResults.Results {
			citations[i] = display.Citation{Title: r.Title, URL: r.URL}
		}
		display.ShowCitations(citations)
	}
}

func runInteractive() {
	fmt.Println("Azure AI CLI - Interactive Mode")
	fmt.Printf("Model: %s\n", cfg.Model)
	if cfg.WebSearch {
		fmt.Printf("Web search: enabled (provider: %s)\n", cfg.WebSearchProvider)
	}
	fmt.Println("Type /help for commands, Ctrl+C to quit, Tab for autocomplete")
	fmt.Println("Tip: End a line with \\ for multiline input")
	fmt.Println()

	// Setup readline with autocomplete
	completer := readline.NewPrefixCompleter(commandItems...)

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "> ",
		AutoComplete:    completer,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		display.ShowError(err.Error())
		return
	}
	defer rl.Close()

	client := api.NewAzureClient(cfg)
	messages := []api.Message{
		{Role: "system", Content: config.DefaultSystemMessage},
	}

	for {
		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				fmt.Println("Goodbye!")
				return
			} else if err == io.EOF {
				fmt.Println("Goodbye!")
				return
			}
			display.ShowError(fmt.Sprintf("Error reading input: %v", err))
			continue
		}

		// Support multiline input with trailing backslash
		input := line
		for strings.HasSuffix(strings.TrimRight(input, " \t"), "\\") {
			input = strings.TrimSuffix(strings.TrimRight(input, " \t"), "\\") + "\n"
			rl.SetPrompt("... ")
			nextLine, err := rl.Readline()
			if err != nil {
				break
			}
			input += nextLine
		}
		rl.SetPrompt("> ")

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Handle commands
		if strings.HasPrefix(input, "/") {
			if handleCommand(input, &messages, client) {
				return
			}
			continue
		}

		// Web search mode: automatically search for every message
		if cfg.WebSearch {
			handleWebSearch(input, &messages, client)
			continue
		}

		// Regular chat
		messages = append(messages, api.Message{Role: "user", Content: input})
		fmt.Println()
		response, err := sendInteractiveMessage(client, messages)
		if err != nil {
			display.ShowError(err.Error())
			messages = messages[:len(messages)-1]
			continue
		}
		messages = append(messages, api.Message{Role: "assistant", Content: response})
		fmt.Println()
	}
}

func handleCommand(input string, messages *[]api.Message, client *api.AzureClient) bool {
	parts := strings.SplitN(input, " ", 2)
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/exit", "/quit", "/q":
		fmt.Println("Goodbye!")
		return true

	case "/clear", "/c":
		*messages = []api.Message{
			{Role: "system", Content: config.DefaultSystemMessage},
		}
		fmt.Println("Conversation cleared.")

	case "/help", "/h":
		fmt.Println("\nCommands:")
		fmt.Printf("  %-18s %s\n", "/exit, /quit, /q", "Exit interactive mode")
		fmt.Printf("  %-18s %s\n", "/clear, /c", "Clear conversation history")
		fmt.Printf("  %-18s %s\n", "/web <query>", "Search web and ask about results")
		fmt.Printf("  %-18s %s\n", "/web on", "Enable auto web search for all messages")
		fmt.Printf("  %-18s %s\n", "/web off", "Disable auto web search")
		fmt.Printf("  %-18s %s\n", "/web <provider>", "Switch provider (tavily, linkup, brave)")
		fmt.Printf("  %-18s %s\n", "/model <name>", "Switch model")
		fmt.Printf("  %-18s %s\n", "/model", "Show current model")
		fmt.Printf("  %-18s %s\n", "/help, /h", "Show this help")
		fmt.Println()

	case "/model":
		if len(parts) > 1 {
			newModel := strings.TrimSpace(parts[1])
			if newModel == "" {
				fmt.Printf("Current model: %s\n", cfg.Model)
				if len(cfg.AvailableModels) > 0 {
					fmt.Printf("Available: %s\n", cfg.GetAvailableModelsString())
				}
			} else if len(cfg.AvailableModels) > 0 && !cfg.ValidateModel(newModel) {
				fmt.Printf("Invalid model: %s\n", newModel)
				fmt.Printf("Available: %s\n", cfg.GetAvailableModelsString())
			} else {
				cfg.Model = newModel
				fmt.Printf("Switched to model: %s\n", cfg.Model)
			}
		} else {
			fmt.Printf("Current model: %s\n", cfg.Model)
			if len(cfg.AvailableModels) > 0 {
				fmt.Printf("Available: %s\n", cfg.GetAvailableModelsString())
			}
		}

	case "/web":
		if len(parts) < 2 {
			status := "off"
			if cfg.WebSearch {
				status = fmt.Sprintf("on (provider: %s)", cfg.WebSearchProvider)
			}
			fmt.Printf("Web search: %s\n", status)
			fmt.Println("Available providers: tavily, linkup, brave")
			fmt.Println("Usage: /web <query> | /web on | /web off | /web provider <name>")
			return false
		}
		arg := strings.TrimSpace(parts[1])
		switch strings.ToLower(arg) {
		case "on":
			cfg.WebSearch = true
			fmt.Printf("Web search enabled (provider: %s).\n", cfg.WebSearchProvider)
		case "off":
			cfg.WebSearch = false
			fmt.Println("Web search disabled.")
		case "provider":
			if len(parts) > 1 {
				// Check if there's a provider name after "provider"
				providerParts := strings.SplitN(parts[1], " ", 2)
				if len(providerParts) > 1 {
					newProvider := strings.ToLower(strings.TrimSpace(providerParts[1]))
					if newProvider == "tavily" || newProvider == "linkup" || newProvider == "brave" {
						cfg.WebSearchProvider = newProvider
						fmt.Printf("Web search provider changed to: %s\n", cfg.WebSearchProvider)
					} else {
						fmt.Printf("Invalid provider: %s\n", newProvider)
						fmt.Println("Available providers: tavily, linkup, brave")
					}
				} else {
					fmt.Printf("Current provider: %s\n", cfg.WebSearchProvider)
					fmt.Println("Available providers: tavily, linkup, brave")
					fmt.Println("Usage: /web provider <name>")
				}
			}
		case "tavily", "linkup", "brave":
			// Allow shorthand: /web tavily, /web linkup, /web brave
			cfg.WebSearchProvider = strings.ToLower(arg)
			fmt.Printf("Web search provider changed to: %s\n", cfg.WebSearchProvider)
		default:
			handleWebSearch(arg, messages, client)
		}

	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		fmt.Println("Type /help for available commands")
	}

	return false
}

func optimizeSearchQuery(query string, messages []api.Message, client *api.AzureClient) (string, error) {
	// Build messages for query optimization
	// Include conversation history so LLM understands context
	optimizeMessages := []api.Message{
		{
			Role: "system",
			Content: `You are an expert search query optimizer. Your task is to transform a user's follow-up question into an effective web search query based on the conversation history.

## Instructions:
1. Read the conversation history to understand the context
2. Extract key entities, topics, and technical terms from the conversation
3. Create a search query that:
   - Is self-contained (doesn't use pronouns like "it", "that", "this" without context)
   - Includes specific names, versions, technologies mentioned in the conversation
   - Uses search-friendly keywords (not conversational language)
   - Is concise (typically 3-8 words)
   - Focuses on finding factual, up-to-date information

## Examples:
- If conversation was about "Kubernetes v1.33 features" and user asks "why that version?" → output: "Kubernetes 1.33 release version naming reason"
- If conversation was about "React hooks" and user asks "what about performance?" → output: "React hooks performance optimization"
- If conversation was about "Python 3.12" and user asks "when was it released?" → output: "Python 3.12 release date"

## Output ONLY the search query, nothing else. No quotes, no explanation.`,
		},
	}

	// Add conversation history (skip original system message, limit to last 6 messages)
	startIdx := 1 // Skip system message
	if len(messages) > 7 {
		startIdx = len(messages) - 6
	}

	for i := startIdx; i < len(messages); i++ {
		msg := messages[i]
		// Truncate long assistant responses to save tokens
		if msg.Role == "assistant" && len(msg.Content) > 400 {
			optimizeMessages = append(optimizeMessages, api.Message{
				Role:    msg.Role,
				Content: msg.Content[:400] + "...",
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

func handleWebSearch(query string, messages *[]api.Message, client *api.AzureClient) {
	// Optimize search query using LLM if there's conversation context
	optimizedQuery := query
	if len(*messages) > 1 { // More than just system message
		var err error
		optimizedQuery, err = optimizeSearchQuery(query, *messages, client)
		if err != nil {
			// Fall back to original query if optimization fails
			log.Printf("Query optimization failed: %v, using original query", err)
			optimizedQuery = query
		}
	}

	// Perform web search with optimized query
	searchContext, err := performWebSearch(optimizedQuery)
	if err != nil {
		display.ShowError(err.Error())
		return
	}

	// Add web search results as a system context message, then add user query
	// This preserves conversation flow while providing web context
	webContextMsg := api.Message{
		Role: "system",
		Content: fmt.Sprintf(`Web search results for additional context (cite using [1], [2], etc. if relevant):

%s`, searchContext),
	}

	// Build messages: existing history + web context + user question
	messagesWithWeb := make([]api.Message, len(*messages))
	copy(messagesWithWeb, *messages)
	messagesWithWeb = append(messagesWithWeb, webContextMsg)
	messagesWithWeb = append(messagesWithWeb, api.Message{Role: "user", Content: query})

	// Send request
	fmt.Println()
	response, err := sendInteractiveMessage(client, messagesWithWeb)
	if err != nil {
		display.ShowError(err.Error())
		return
	}

	// Add only the user message and response to history (not the web context)
	*messages = append(*messages, api.Message{Role: "user", Content: query})
	*messages = append(*messages, api.Message{Role: "assistant", Content: response})

	// Show citations if enabled
	if cfg.Citations && searchResults != nil && len(searchResults.Results) > 0 {
		fmt.Println()
		citations := make([]display.Citation, len(searchResults.Results))
		for i, r := range searchResults.Results {
			citations[i] = display.Citation{Title: r.Title, URL: r.URL}
		}
		display.ShowCitations(citations)
	}
	fmt.Println()
}

func sendInteractiveMessage(client *api.AzureClient, messages []api.Message) (string, error) {
	if cfg.Stream {
		var fullContent strings.Builder
		firstChunk := true

		sp := display.NewSpinner("Thinking...")
		sp.Start()

		err := client.QueryStreamWithHistory(messages,
			func(content string) {
				if firstChunk {
					firstChunk = false
					if cfg.Render {
						sp.UpdateMessage("Receiving...")
					} else {
						sp.Stop()
					}
				}
				if cfg.Render {
					fullContent.WriteString(content)
				} else {
					fmt.Print(content)
				}
			},
			nil,
		)

		sp.Stop()

		if err != nil {
			return "", err
		}

		if cfg.Render {
			display.ShowContentRendered(fullContent.String())
			return fullContent.String(), nil
		}
		fmt.Println()
		return fullContent.String(), nil
	}

	// Non-streaming
	sp := display.NewSpinner("Thinking...")
	sp.Start()

	resp, err := client.QueryWithHistory(messages)
	sp.Stop()

	if err != nil {
		return "", err
	}

	content := resp.GetContent()
	if cfg.Render {
		display.ShowContentRendered(content)
	} else {
		display.ShowContent(content)
	}

	return content, nil
}

func performWebSearch(query string) (string, error) {
	sp := display.NewSpinner("Searching web...")
	sp.Start()

	var results *api.TavilyResponse
	var err error

	switch cfg.WebSearchProvider {
	case "linkup":
		linkupClient := api.NewLinkupClient(cfg)
		linkupClient.SetKeyRotationCallback(func(from, to, total int) {
			display.ShowKeyRotation("Linkup", from, to, total)
		})

		linkupResults, searchErr := linkupClient.Search(query)
		if searchErr != nil {
			sp.Stop()
			return "", searchErr
		}
		results = linkupResults.ToTavilyResponse()

	case "brave":
		braveClient := api.NewBraveClient(cfg)
		braveClient.SetKeyRotationCallback(func(from, to, total int) {
			display.ShowKeyRotation("Brave", from, to, total)
		})

		braveResults, searchErr := braveClient.Search(query)
		if searchErr != nil {
			sp.Stop()
			return "", searchErr
		}
		results = braveResults.ToTavilyResponse()

	default: // tavily
		tavilyClient := api.NewTavilyClient(cfg)
		tavilyClient.SetKeyRotationCallback(func(from, to, total int) {
			display.ShowKeyRotation("Tavily", from, to, total)
		})

		results, err = tavilyClient.Search(query)
	}

	sp.Stop()

	if err != nil {
		return "", err
	}

	// Store results for citations
	searchResults = results

	return results.FormatResultsAsContext(), nil
}

func buildWebSearchPrompt(searchContext string) string {
	return fmt.Sprintf(`You are a helpful assistant. Use the following web search results to answer the user's question.
Cite sources when possible using [1], [2], etc.

Web Search Results:
%s

Instructions:
- Answer based on the search results above
- Be precise and concise
- If the search results don't contain relevant information, say so`, searchContext)
}

func runNormal(client *api.AzureClient, systemPrompt, userMessage string) {
	sp := display.NewSpinner("Waiting for response...")
	sp.Start()

	resp, err := client.Query(systemPrompt, userMessage)
	sp.Stop()

	if err != nil {
		display.ShowError(err.Error())
		os.Exit(1)
	}

	if cfg.Render {
		display.ShowContentRendered(resp.GetContent())
	} else {
		display.ShowContent(resp.GetContent())
	}

	if cfg.Usage {
		fmt.Println()
		display.ShowUsage(resp.GetUsageMap())
	}
}

func runStream(client *api.AzureClient, systemPrompt, userMessage string) {
	var finalResp *api.ChatResponse
	var fullContent strings.Builder
	firstChunk := true

	sp := display.NewSpinner("Waiting for response...")
	sp.Start()

	err := client.QueryStream(systemPrompt, userMessage,
		func(content string) {
			if firstChunk {
				firstChunk = false
				if cfg.Render {
					sp.UpdateMessage("Receiving response...")
				} else {
					sp.Stop()
				}
			}

			if cfg.Render {
				fullContent.WriteString(content)
			} else {
				fmt.Print(content)
			}
		},
		func(resp *api.ChatResponse) {
			finalResp = resp
		},
	)

	sp.Stop()

	if err != nil {
		display.ShowError(err.Error())
		os.Exit(1)
	}

	if cfg.Render {
		display.ShowContentRendered(fullContent.String())
	} else {
		fmt.Println()
	}

	if finalResp != nil && cfg.Usage {
		fmt.Println()
		display.ShowUsage(finalResp.GetUsageMap())
	}
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
