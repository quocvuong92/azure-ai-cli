package cmd

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

// Command suggestions for autocomplete
var commandSuggestions = []suggestion{
	{text: "/exit", desc: "Exit interactive mode"},
	{text: "/quit", desc: "Exit interactive mode"},
	{text: "/clear", desc: "Clear conversation history"},
	{text: "/help", desc: "Show available commands"},
	{text: "/web on", desc: "Enable auto web search"},
	{text: "/web off", desc: "Disable auto web search"},
	{text: "/web ", desc: "Search web for query"},
	{text: "/model ", desc: "Show or change model"},
}

type suggestion struct {
	text string
	desc string
}

// Interactive mode model for bubbletea
type interactiveModel struct {
	textInput   textinput.Model
	suggestions []suggestion
	showSuggest bool
	cursor      int
	client      *api.AzureClient
	messages    []api.Message
	quitting    bool
}

func newInteractiveModel(client *api.AzureClient) interactiveModel {
	ti := textinput.New()
	ti.Placeholder = "Type a message or /help for commands..."
	ti.Focus()
	ti.CharLimit = 4096
	ti.Width = 80
	ti.Prompt = "> "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("cyan"))

	return interactiveModel{
		textInput:   ti,
		suggestions: []suggestion{},
		showSuggest: false,
		cursor:      0,
		client:      client,
		messages: []api.Message{
			{Role: "system", Content: "Be precise and concise."},
		},
		quitting: false,
	}
}

func (m interactiveModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m interactiveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			fmt.Println("\nGoodbye!")
			return m, tea.Quit

		case tea.KeyUp:
			if m.showSuggest && len(m.suggestions) > 0 {
				if m.cursor > 0 {
					m.cursor--
				}
				return m, nil
			}

		case tea.KeyDown:
			if m.showSuggest && len(m.suggestions) > 0 {
				if m.cursor < len(m.suggestions)-1 {
					m.cursor++
				}
				return m, nil
			}

		case tea.KeyTab:
			if m.showSuggest && len(m.suggestions) > 0 {
				m.textInput.SetValue(m.suggestions[m.cursor].text)
				m.textInput.CursorEnd()
				m.showSuggest = false
				m.suggestions = []suggestion{}
				return m, nil
			}

		case tea.KeyEnter:
			if m.showSuggest && len(m.suggestions) > 0 {
				m.textInput.SetValue(m.suggestions[m.cursor].text)
				m.textInput.CursorEnd()
				m.showSuggest = false
				m.suggestions = []suggestion{}
				return m, nil
			}

			input := strings.TrimSpace(m.textInput.Value())
			if input == "" {
				return m, nil
			}

			m.textInput.SetValue("")
			m.showSuggest = false
			m.suggestions = []suggestion{}

			// Process input
			fmt.Println()
			if strings.HasPrefix(input, "/") {
				if m.handleCommand(input) {
					m.quitting = true
					return m, tea.Quit
				}
			} else if cfg.WebSearch {
				handleWebSearch(input, &m.messages, m.client)
			} else {
				m.messages = append(m.messages, api.Message{Role: "user", Content: input})
				response, err := sendInteractiveMessage(m.client, m.messages)
				if err != nil {
					display.ShowError(err.Error())
					m.messages = m.messages[:len(m.messages)-1]
				} else {
					m.messages = append(m.messages, api.Message{Role: "assistant", Content: response})
				}
				fmt.Println()
			}

			return m, nil
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)

	// Update suggestions based on input
	input := m.textInput.Value()
	if strings.HasPrefix(input, "/") {
		m.suggestions = filterSuggestions(input)
		m.showSuggest = len(m.suggestions) > 0
		if m.cursor >= len(m.suggestions) {
			m.cursor = 0
		}
	} else {
		m.showSuggest = false
		m.suggestions = []suggestion{}
	}

	return m, cmd
}

func (m interactiveModel) handleCommand(input string) bool {
	parts := strings.SplitN(input, " ", 2)
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "/exit", "/quit", "/q":
		fmt.Println("Goodbye!")
		return true

	case "/clear", "/c":
		m.messages = []api.Message{
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
			handleWebSearch(arg, &m.messages, m.client)
		}

	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		fmt.Println("Type /help for available commands")
		fmt.Println()
	}

	return false
}

func (m interactiveModel) View() string {
	if m.quitting {
		return ""
	}

	var s strings.Builder
	s.WriteString(m.textInput.View())
	s.WriteString("\n")

	if m.showSuggest && len(m.suggestions) > 0 {
		selectedStyle := lipgloss.NewStyle().Background(lipgloss.Color("62")).Foreground(lipgloss.Color("230"))
		normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
		descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

		for i, sug := range m.suggestions {
			if i == m.cursor {
				s.WriteString(selectedStyle.Render(fmt.Sprintf(" %s ", sug.text)))
				s.WriteString(descStyle.Render(fmt.Sprintf(" %s", sug.desc)))
			} else {
				s.WriteString(normalStyle.Render(fmt.Sprintf(" %s ", sug.text)))
				s.WriteString(descStyle.Render(fmt.Sprintf(" %s", sug.desc)))
			}
			s.WriteString("\n")
		}
	}

	return s.String()
}

func filterSuggestions(input string) []suggestion {
	var matches []suggestion
	input = strings.ToLower(input)
	for _, s := range commandSuggestions {
		if strings.HasPrefix(strings.ToLower(s.text), input) {
			matches = append(matches, s)
		}
	}
	return matches
}

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
	fmt.Println("Type /help for commands, Ctrl+C to quit")
	fmt.Println("Use Up/Down arrows to navigate suggestions, Enter/Tab to select")
	fmt.Println()

	client := api.NewAzureClient(cfg)
	model := newInteractiveModel(client)

	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		display.ShowError(err.Error())
		os.Exit(1)
	}
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
