package cmd

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/quocvuong92/azure-ai-cli/internal/api"
	"github.com/quocvuong92/azure-ai-cli/internal/config"
	"github.com/quocvuong92/azure-ai-cli/internal/display"
)

// App holds the application state
type App struct {
	cfg           *config.Config
	verbose       bool
	listModels    bool
	searchResults *api.TavilyResponse // Store search results for citations
}

// NewApp creates a new App instance with default configuration
func NewApp() *App {
	return &App{
		cfg: config.NewConfig(),
	}
}

// Execute runs the root command
func Execute() {
	app := NewApp()

	rootCmd := &cobra.Command{
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
		Run: func(cmd *cobra.Command, args []string) {
			app.run(cmd, args)
		},
	}

	rootCmd.Flags().BoolVarP(&app.verbose, "verbose", "v", false, "Enable debug mode")
	rootCmd.Flags().BoolVarP(&app.cfg.Usage, "usage", "u", false, "Show token usage statistics")
	rootCmd.Flags().BoolVarP(&app.cfg.Stream, "stream", "s", false, "Stream output in real-time")
	rootCmd.Flags().BoolVarP(&app.cfg.Render, "render", "r", false, "Render markdown with colors and formatting")
	rootCmd.Flags().BoolVarP(&app.cfg.WebSearch, "web", "w", false, "Search web first (requires TAVILY_API_KEYS, LINKUP_API_KEYS, or BRAVE_API_KEYS)")
	rootCmd.Flags().BoolVarP(&app.cfg.Citations, "citations", "c", false, "Show citations/sources from web search")
	rootCmd.Flags().BoolVarP(&app.cfg.Interactive, "interactive", "i", false, "Interactive chat mode")
	rootCmd.Flags().StringVarP(&app.cfg.Model, "model", "m", "", "Model/deployment name (defaults to first in AZURE_OPENAI_MODELS)")
	rootCmd.Flags().StringVarP(&app.cfg.WebSearchProvider, "provider", "p", "", "Web search provider: tavily, linkup, or brave (default: auto-detect)")
	rootCmd.Flags().BoolVar(&app.listModels, "list-models", false, "List available models")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func (app *App) run(cmd *cobra.Command, args []string) {
	if app.verbose {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	} else {
		log.SetOutput(io.Discard)
	}

	// Handle --list-models flag
	if app.listModels {
		_ = app.cfg.Validate()
		if len(app.cfg.AvailableModels) == 0 {
			fmt.Println("No models configured. Set AZURE_OPENAI_MODELS environment variable.")
			fmt.Println("Example: export AZURE_OPENAI_MODELS=gpt-4o,gpt-35-turbo")
			os.Exit(1)
		}
		display.ShowModels(app.cfg.AvailableModels, app.cfg.Model)
		return
	}

	// Validate config
	if err := app.cfg.Validate(); err != nil {
		display.ShowError(err.Error())
		os.Exit(1)
	}

	// Initialize markdown renderer if render flag is set
	if app.cfg.Render {
		if err := display.InitRenderer(); err != nil {
			log.Printf("Failed to initialize renderer: %v", err)
		}
	}

	// Interactive mode
	if app.cfg.Interactive {
		app.runInteractive()
		return
	}

	// Require query if not interactive mode
	if len(args) == 0 {
		_ = cmd.Help()
		os.Exit(1)
	}

	query := args[0]
	log.Printf("Query: %s", query)
	log.Printf("Model: %s", app.cfg.Model)
	log.Printf("Stream: %v", app.cfg.Stream)
	log.Printf("WebSearch: %v", app.cfg.WebSearch)

	// Build system prompt and user message
	systemPrompt := config.DefaultSystemMessage
	userMessage := query

	// Web search if requested
	if app.cfg.WebSearch {
		searchContext, err := app.performWebSearch(query)
		if err != nil {
			display.ShowError(err.Error())
			os.Exit(1)
		}
		systemPrompt = buildWebSearchPrompt(searchContext)
	}

	// Create Azure client
	azureClient := api.NewAzureClient(app.cfg)

	log.Printf("Sending request to Azure OpenAI...")

	if app.cfg.Stream {
		app.runStream(azureClient, systemPrompt, userMessage)
	} else {
		app.runNormal(azureClient, systemPrompt, userMessage)
	}

	// Show citations if web search was used and citations flag is set
	if app.cfg.WebSearch && app.cfg.Citations && app.searchResults != nil && len(app.searchResults.Results) > 0 {
		fmt.Println()
		citations := make([]display.Citation, len(app.searchResults.Results))
		for i, r := range app.searchResults.Results {
			citations[i] = display.Citation{Title: r.Title, URL: r.URL}
		}
		display.ShowCitations(citations)
	}
}
