package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/c-bata/go-prompt"
	"github.com/quocvuong92/azure-ai-cli/internal/api"
	"github.com/quocvuong92/azure-ai-cli/internal/config"
	"github.com/quocvuong92/azure-ai-cli/internal/display"
	"github.com/quocvuong92/azure-ai-cli/internal/executor"
)

// InteractiveSession holds the state for interactive mode
type InteractiveSession struct {
	app      *App
	client   *api.AzureClient
	exec     *executor.Executor
	messages []api.Message
	exitFlag bool
}

// completer provides auto-suggestions for commands
func (s *InteractiveSession) completer(d prompt.Document) []prompt.Suggest {
	// Only show suggestions when input starts with "/"
	text := d.TextBeforeCursor()
	if !strings.HasPrefix(text, "/") {
		return []prompt.Suggest{}
	}

	suggestions := []prompt.Suggest{
		{Text: "/exit", Description: "Exit interactive mode"},
		{Text: "/quit", Description: "Exit interactive mode"},
		{Text: "/q", Description: "Exit interactive mode"},
		{Text: "/clear", Description: "Clear conversation history"},
		{Text: "/c", Description: "Clear conversation history"},
		{Text: "/help", Description: "Show available commands"},
		{Text: "/h", Description: "Show available commands"},
		{Text: "/web on", Description: "Enable auto web search"},
		{Text: "/web off", Description: "Disable auto web search"},
		{Text: "/web tavily", Description: "Use Tavily search provider"},
		{Text: "/web linkup", Description: "Use Linkup search provider"},
		{Text: "/web brave", Description: "Use Brave search provider"},
		{Text: "/model", Description: "Show/switch model"},
		{Text: "/allow-dangerous", Description: "Enable dangerous commands (with confirmation)"},
		{Text: "/show-permissions", Description: "Show command execution permissions"},
	}

	return prompt.FilterHasPrefix(suggestions, d.GetWordBeforeCursor(), true)
}

func (app *App) runInteractive() {
	fmt.Println("Azure AI CLI - Interactive Mode")
	fmt.Printf("Model: %s\n", app.cfg.Model)
	if app.cfg.WebSearch {
		fmt.Printf("Web search: enabled (provider: %s)\n", app.cfg.WebSearchProvider)
	}
	fmt.Println("Type /help for commands, Ctrl+C or Ctrl+D to quit")
	fmt.Println("Commands auto-complete as you type")
	fmt.Println()

	session := &InteractiveSession{
		app:    app,
		client: api.NewAzureClient(app.cfg),
		exec:   executor.NewExecutor(),
		messages: []api.Message{
			{Role: "system", Content: config.DefaultSystemMessage},
		},
		exitFlag: false,
	}

	p := prompt.New(
		session.executor,
		session.completer,
		prompt.OptionPrefix("> "),
		prompt.OptionTitle("Azure AI CLI"),
		prompt.OptionPrefixTextColor(prompt.Green),
		prompt.OptionSuggestionBGColor(prompt.DarkGray),
		prompt.OptionSuggestionTextColor(prompt.White),
		prompt.OptionSelectedSuggestionBGColor(prompt.LightGray),
		prompt.OptionSelectedSuggestionTextColor(prompt.Black),
		prompt.OptionDescriptionBGColor(prompt.DarkGray),
		prompt.OptionDescriptionTextColor(prompt.White),
		prompt.OptionSelectedDescriptionBGColor(prompt.LightGray),
		prompt.OptionSelectedDescriptionTextColor(prompt.Black),
		prompt.OptionMaxSuggestion(10),
	)

	p.Run()
}

// executor handles the execution of each input line
func (s *InteractiveSession) executor(input string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return
	}

	// Handle commands
	if strings.HasPrefix(input, "/") {
		if s.app.handleCommand(input, &s.messages, s.client, s.exec) {
			fmt.Println("Goodbye!")
			s.exitFlag = true
		}
		return
	}

	// Web search mode: automatically search for every message
	if s.app.cfg.WebSearch {
		s.app.handleWebSearch(input, &s.messages, s.client)
		return
	}

	// Regular chat with tool support
	s.messages = append(s.messages, api.Message{Role: "user", Content: input})
	fmt.Println()
	response, err := s.app.sendInteractiveMessageWithTools(s.client, s.exec, &s.messages)
	if err != nil {
		display.ShowError(err.Error())
		s.messages = s.messages[:len(s.messages)-1]
		return
	}
	if response != "" {
		s.messages = append(s.messages, api.Message{Role: "assistant", Content: response})
	}
	fmt.Println()
}

func (app *App) handleCommand(input string, messages *[]api.Message, client *api.AzureClient, exec *executor.Executor) bool {
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
		fmt.Printf("  %-24s %s\n", "/exit, /quit, /q", "Exit interactive mode")
		fmt.Printf("  %-24s %s\n", "/clear, /c", "Clear conversation history")
		fmt.Printf("  %-24s %s\n", "/web <query>", "Search web and ask about results")
		fmt.Printf("  %-24s %s\n", "/web on", "Enable auto web search for all messages")
		fmt.Printf("  %-24s %s\n", "/web off", "Disable auto web search")
		fmt.Printf("  %-24s %s\n", "/web <provider>", "Switch provider (tavily, linkup, brave)")
		fmt.Printf("  %-24s %s\n", "/model <name>", "Switch model")
		fmt.Printf("  %-24s %s\n", "/model", "Show current model")
		fmt.Printf("  %-24s %s\n", "/allow-dangerous", "Allow dangerous commands (with confirmation)")
		fmt.Printf("  %-24s %s\n", "/show-permissions", "Show command execution permissions")
		fmt.Printf("  %-24s %s\n", "/help, /h", "Show this help")
		fmt.Println()

	case "/model":
		app.handleModelCommand(parts)

	case "/web":
		app.handleWebCommand(parts, messages, client)

	case "/allow-dangerous":
		exec.GetPermissionManager().EnableDangerous()
		fmt.Println("⚠️  Dangerous commands enabled for this session")
		fmt.Println("Note: You will still be asked to confirm before execution")

	case "/show-permissions":
		settings := exec.GetPermissionManager().GetSettings()
		display.ShowPermissionSettings(settings)

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

func (app *App) sendInteractiveMessageWithTools(client *api.AzureClient, exec *executor.Executor, messages *[]api.Message) (string, error) {
	ctx := context.Background()
	tools := api.GetDefaultTools()

	// Keep calling the API until there are no more tool calls
	for {
		sp := display.NewSpinner("Thinking...")
		sp.Start()

		resp, err := client.QueryWithHistoryAndToolsContext(ctx, *messages, tools)
		sp.Stop()

		if err != nil {
			return "", err
		}

		// Check if there are tool calls
		if len(resp.Choices) > 0 && resp.Choices[0].HasToolCalls() {
			toolCalls := resp.Choices[0].GetToolCalls()

			// Add assistant message with tool calls to history
			*messages = append(*messages, api.Message{
				Role:      "assistant",
				ToolCalls: toolCalls,
			})

			// Process each tool call
			for _, toolCall := range toolCalls {
				if toolCall.Function.Name == "execute_command" {
					// Parse arguments
					var args struct {
						Command   string `json:"command"`
						Reasoning string `json:"reasoning"`
					}
					if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
						display.ShowError(fmt.Sprintf("Failed to parse tool arguments: %v", err))
						continue
					}

					// Check permission
					allowed, needsConfirm, reason := exec.GetPermissionManager().CheckPermission(args.Command)

					var result *executor.ExecutionResult
					var toolResult string

					if !allowed && !needsConfirm {
						// Blocked
						display.ShowCommandBlocked(args.Command, reason)
						toolResult = fmt.Sprintf("Command blocked: %s", reason)
					} else {
						// Ask for confirmation if needed
						if needsConfirm {
							allow, always := display.AskCommandConfirmation(args.Command, args.Reasoning)
							if !allow {
								toolResult = "Command execution denied by user"
								*messages = append(*messages, api.Message{
									Role:       "tool",
									Content:    toolResult,
									ToolCallID: toolCall.ID,
								})
								continue
							}
							if always {
								exec.GetPermissionManager().AddToAllowlist(args.Command)
							}
						}

						// Execute the command
						display.ShowCommandExecuting(args.Command)
						result, err = exec.Execute(ctx, args.Command)

						if err != nil || !result.IsSuccess() {
							display.ShowCommandError(args.Command, result.Error)
							toolResult = result.FormatResult()
						} else {
							display.ShowCommandOutput(result.Output)
							toolResult = result.Output
							if toolResult == "" {
								toolResult = "Command executed successfully (no output)"
							}
						}
					}

					// Add tool result to messages
					*messages = append(*messages, api.Message{
						Role:       "tool",
						Content:    toolResult,
						ToolCallID: toolCall.ID,
					})
				}
			}

			// Continue loop to get AI's response to the tool results
			continue
		}

		// No tool calls, display the final response
		content := resp.GetContent()
		if content != "" {
			if app.cfg.Render {
				display.ShowContentRendered(content)
			} else {
				display.ShowContent(content)
			}
		}

		return content, nil
	}
}
