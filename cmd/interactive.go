package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/chzyer/readline"

	"github.com/quocvuong92/azure-ai-cli/internal/api"
	"github.com/quocvuong92/azure-ai-cli/internal/config"
	"github.com/quocvuong92/azure-ai-cli/internal/display"
	"github.com/quocvuong92/azure-ai-cli/internal/executor"
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
	readline.PcItem("/allow-dangerous"),
	readline.PcItem("/show-permissions"),
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
	exec := executor.NewExecutor()
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
			if app.handleCommand(input, &messages, client, exec) {
				return
			}
			continue
		}

		// Web search mode: automatically search for every message
		if app.cfg.WebSearch {
			app.handleWebSearch(input, &messages, client)
			continue
		}

		// Regular chat with tool support
		messages = append(messages, api.Message{Role: "user", Content: input})
		fmt.Println()
		response, err := app.sendInteractiveMessageWithTools(client, exec, &messages)
		if err != nil {
			display.ShowError(err.Error())
			messages = messages[:len(messages)-1]
			continue
		}
		if response != "" {
			messages = append(messages, api.Message{Role: "assistant", Content: response})
		}
		fmt.Println()
	}
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
