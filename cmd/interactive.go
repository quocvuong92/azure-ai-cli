package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/chzyer/readline"

	"github.com/quocvuong92/azure-ai-cli/internal/api"
	"github.com/quocvuong92/azure-ai-cli/internal/config"
	"github.com/quocvuong92/azure-ai-cli/internal/display"
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

func (app *App) runInteractive() {
	fmt.Println("Azure AI CLI - Interactive Mode")
	fmt.Printf("Model: %s\n", app.cfg.Model)
	if app.cfg.WebSearch {
		fmt.Printf("Web search: enabled (provider: %s)\n", app.cfg.WebSearchProvider)
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

	client := api.NewAzureClient(app.cfg)
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
			if app.handleCommand(input, &messages, client) {
				return
			}
			continue
		}

		// Web search mode: automatically search for every message
		if app.cfg.WebSearch {
			app.handleWebSearch(input, &messages, client)
			continue
		}

		// Regular chat
		messages = append(messages, api.Message{Role: "user", Content: input})
		fmt.Println()
		response, err := app.sendInteractiveMessage(client, messages)
		if err != nil {
			display.ShowError(err.Error())
			messages = messages[:len(messages)-1]
			continue
		}
		messages = append(messages, api.Message{Role: "assistant", Content: response})
		fmt.Println()
	}
}

func (app *App) handleCommand(input string, messages *[]api.Message, client *api.AzureClient) bool {
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
		app.handleModelCommand(parts)

	case "/web":
		app.handleWebCommand(parts, messages, client)

	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		fmt.Println("Type /help for available commands")
	}

	return false
}

func (app *App) handleModelCommand(parts []string) {
	if len(parts) > 1 {
		newModel := strings.TrimSpace(parts[1])
		if newModel == "" {
			fmt.Printf("Current model: %s\n", app.cfg.Model)
			if len(app.cfg.AvailableModels) > 0 {
				fmt.Printf("Available: %s\n", app.cfg.GetAvailableModelsString())
			}
		} else if len(app.cfg.AvailableModels) > 0 && !app.cfg.ValidateModel(newModel) {
			fmt.Printf("Invalid model: %s\n", newModel)
			fmt.Printf("Available: %s\n", app.cfg.GetAvailableModelsString())
		} else {
			app.cfg.Model = newModel
			fmt.Printf("Switched to model: %s\n", app.cfg.Model)
		}
	} else {
		fmt.Printf("Current model: %s\n", app.cfg.Model)
		if len(app.cfg.AvailableModels) > 0 {
			fmt.Printf("Available: %s\n", app.cfg.GetAvailableModelsString())
		}
	}
}

func (app *App) handleWebCommand(parts []string, messages *[]api.Message, client *api.AzureClient) {
	if len(parts) < 2 {
		status := "off"
		if app.cfg.WebSearch {
			status = fmt.Sprintf("on (provider: %s)", app.cfg.WebSearchProvider)
		}
		fmt.Printf("Web search: %s\n", status)
		fmt.Println("Available providers: tavily, linkup, brave")
		fmt.Println("Usage: /web <query> | /web on | /web off | /web provider <name>")
		return
	}

	arg := strings.TrimSpace(parts[1])
	switch strings.ToLower(arg) {
	case "on":
		app.cfg.WebSearch = true
		fmt.Printf("Web search enabled (provider: %s).\n", app.cfg.WebSearchProvider)
	case "off":
		app.cfg.WebSearch = false
		fmt.Println("Web search disabled.")
	case "provider":
		// Check if there's a provider name after "provider"
		providerParts := strings.SplitN(parts[1], " ", 2)
		if len(providerParts) > 1 {
			newProvider := strings.ToLower(strings.TrimSpace(providerParts[1]))
			if newProvider == "tavily" || newProvider == "linkup" || newProvider == "brave" {
				app.cfg.WebSearchProvider = newProvider
				fmt.Printf("Web search provider changed to: %s\n", app.cfg.WebSearchProvider)
			} else {
				fmt.Printf("Invalid provider: %s\n", newProvider)
				fmt.Println("Available providers: tavily, linkup, brave")
			}
		} else {
			fmt.Printf("Current provider: %s\n", app.cfg.WebSearchProvider)
			fmt.Println("Available providers: tavily, linkup, brave")
			fmt.Println("Usage: /web provider <name>")
		}
	case "tavily", "linkup", "brave":
		// Allow shorthand: /web tavily, /web linkup, /web brave
		app.cfg.WebSearchProvider = strings.ToLower(arg)
		fmt.Printf("Web search provider changed to: %s\n", app.cfg.WebSearchProvider)
	default:
		app.handleWebSearch(arg, messages, client)
	}
}

func (app *App) sendInteractiveMessage(client *api.AzureClient, messages []api.Message) (string, error) {
	if app.cfg.Stream {
		var fullContent strings.Builder
		firstChunk := true

		sp := display.NewSpinner("Thinking...")
		sp.Start()

		err := client.QueryStreamWithHistory(messages,
			func(content string) {
				if firstChunk {
					firstChunk = false
					if app.cfg.Render {
						sp.UpdateMessage("Receiving...")
					} else {
						sp.Stop()
					}
				}
				if app.cfg.Render {
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

		if app.cfg.Render {
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
	if app.cfg.Render {
		display.ShowContentRendered(content)
	} else {
		display.ShowContent(content)
	}

	return content, nil
}
