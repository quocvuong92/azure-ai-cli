package cmd

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/c-bata/go-prompt"
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

	// Interactive mode state
	interactiveClient   *api.AzureClient
	interactiveMessages []api.Message
	exitInteractive     bool
)

var rootCmd = &cobra.Command{
	Use:   "azure-ai [query]",
	Short: "A CLI client for Azure OpenAI with web search",
	Long: `Azure AI CLI is a command-line client for Azure OpenAI API,
with optional web search powered by Tavily.

Supports multiple Tavily API keys with automatic rotation for free tier usage.

Examples:
  azure-ai "What is Kubernetes?"
  azure-ai -m gpt-4o "Explain Docker"
  azure-ai --web "Latest news on Go 1.24"
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
	rootCmd.Flags().BoolVarP(&cfg.WebSearch, "web", "w", false, "Search web first using Tavily (requires TAVILY_API_KEYS)")
	rootCmd.Flags().BoolVarP(&cfg.Citations, "citations", "c", false, "Show citations/sources from web search")
	rootCmd.Flags().BoolVarP(&cfg.Interactive, "interactive", "i", false, "Interactive chat mode")
	rootCmd.Flags().StringVarP(&cfg.Model, "model", "m", "", "Model/deployment name (defaults to first in AZURE_OPENAI_MODELS)")
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
	systemPrompt := "Be precise and concise."
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
		fmt.Println("Web search: enabled (every message will search the web)")
	}
	fmt.Println("Type /help for commands, /exit to quit, Tab for autocomplete")
	fmt.Println()

	interactiveClient = api.NewAzureClient(cfg)
	interactiveMessages = []api.Message{
		{Role: "system", Content: "Be precise and concise."},
	}
	exitInteractive = false

	p := prompt.New(
		interactiveExecutor,
		interactiveCompleter,
		prompt.OptionPrefix("> "),
		prompt.OptionTitle("Azure AI CLI"),
		prompt.OptionPrefixTextColor(prompt.Cyan),
		prompt.OptionPreviewSuggestionTextColor(prompt.DarkGray),
		prompt.OptionSelectedSuggestionBGColor(prompt.DarkBlue),
		prompt.OptionSuggestionBGColor(prompt.DarkGray),
	)

	for !exitInteractive {
		input := p.Input()
		if exitInteractive {
			break
		}
		if input == "" {
			continue
		}
	}
}

func interactiveCompleter(d prompt.Document) []prompt.Suggest {
	text := d.TextBeforeCursor()

	// Only show suggestions when typing commands starting with /
	if !strings.HasPrefix(text, "/") {
		return nil
	}

	suggestions := []prompt.Suggest{
		{Text: "/exit", Description: "Exit interactive mode"},
		{Text: "/quit", Description: "Exit interactive mode"},
		{Text: "/clear", Description: "Clear conversation history"},
		{Text: "/help", Description: "Show available commands"},
		{Text: "/web ", Description: "Search web: /web <query>, /web on, /web off"},
		{Text: "/model ", Description: "Show or change model"},
	}

	return prompt.FilterHasPrefix(suggestions, text, true)
}

func interactiveExecutor(input string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return
	}

	// Handle commands
	if strings.HasPrefix(input, "/") {
		if handleCommand(input, &interactiveMessages, interactiveClient) {
			exitInteractive = true
			return
		}
		return
	}

	// Web search mode: automatically search for every message
	if cfg.WebSearch {
		handleWebSearch(input, &interactiveMessages, interactiveClient)
		return
	}

	// Add user message to history
	interactiveMessages = append(interactiveMessages, api.Message{Role: "user", Content: input})

	// Send request
	fmt.Println()
	response, err := sendInteractiveMessage(interactiveClient, interactiveMessages)
	if err != nil {
		display.ShowError(err.Error())
		// Remove the failed user message
		interactiveMessages = interactiveMessages[:len(interactiveMessages)-1]
		return
	}

	// Add assistant response to history
	interactiveMessages = append(interactiveMessages, api.Message{Role: "assistant", Content: response})
	fmt.Println()
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
			{Role: "system", Content: "Be precise and concise."},
		}
		fmt.Println("Conversation cleared.")
		fmt.Println()

	case "/help", "/h":
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  /exit, /quit, /q  - Exit interactive mode")
		fmt.Println("  /clear, /c        - Clear conversation history")
		fmt.Println("  /web <query>      - Search web and ask about results")
		fmt.Println("  /web on           - Enable auto web search for all messages")
		fmt.Println("  /web off          - Disable auto web search")
		fmt.Println("  /model <name>     - Switch model")
		fmt.Println("  /model            - Show current model")
		fmt.Println("  /help, /h         - Show this help")
		fmt.Println()

	case "/model":
		if len(parts) > 1 {
			newModel := strings.TrimSpace(parts[1])
			if len(cfg.AvailableModels) > 0 && !cfg.ValidateModel(newModel) {
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
		fmt.Println()

	case "/web":
		if len(parts) < 2 {
			status := "off"
			if cfg.WebSearch {
				status = "on"
			}
			fmt.Printf("Web search: %s\n", status)
			fmt.Println("Usage: /web <query> | /web on | /web off")
			fmt.Println()
			return false
		}
		arg := strings.TrimSpace(parts[1])
		switch strings.ToLower(arg) {
		case "on":
			cfg.WebSearch = true
			fmt.Println("Web search enabled for all messages.")
			fmt.Println()
		case "off":
			cfg.WebSearch = false
			fmt.Println("Web search disabled.")
			fmt.Println()
		default:
			// It's a search query
			handleWebSearch(arg, messages, client)
		}

	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		fmt.Println("Type /help for available commands")
		fmt.Println()
	}

	return false
}

func handleWebSearch(query string, messages *[]api.Message, client *api.AzureClient) {
	// Perform web search
	searchContext, err := performWebSearch(query)
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
	tavilyClient := api.NewTavilyClient(cfg)
	tavilyClient.SetKeyRotationCallback(func(from, to, total int) {
		display.ShowKeyRotation("Tavily", from, to, total)
	})

	sp := display.NewSpinner("Searching web...")
	sp.Start()

	results, err := tavilyClient.Search(query)
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
