package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/quocvuong92/azure-ai-cli/internal/api"
	"github.com/quocvuong92/azure-ai-cli/internal/display"
)

func (app *App) runNormal(client *api.AzureClient, systemPrompt, userMessage string) {
	sp := display.NewSpinner("Waiting for response...")
	sp.Start()

	resp, err := client.Query(systemPrompt, userMessage)
	sp.Stop()

	if err != nil {
		display.ShowError(err.Error())
		os.Exit(1)
	}

	if app.cfg.Render {
		display.ShowContentRendered(resp.GetContent())
	} else {
		display.ShowContent(resp.GetContent())
	}

	if app.cfg.Usage {
		fmt.Println()
		display.ShowUsage(resp.GetUsageMap())
	}
}

func (app *App) runStream(client *api.AzureClient, systemPrompt, userMessage string) {
	var finalResp *api.ChatResponse
	var fullContent strings.Builder
	firstChunk := true

	sp := display.NewSpinner("Waiting for response...")
	sp.Start()

	err := client.QueryStream(systemPrompt, userMessage,
		func(content string) {
			if firstChunk {
				firstChunk = false
				if app.cfg.Render {
					sp.UpdateMessage("Receiving response...")
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
		func(resp *api.ChatResponse) {
			finalResp = resp
		},
	)

	sp.Stop()

	if err != nil {
		display.ShowError(err.Error())
		os.Exit(1)
	}

	if app.cfg.Render {
		display.ShowContentRendered(fullContent.String())
	} else {
		fmt.Println()
	}

	if finalResp != nil && app.cfg.Usage {
		fmt.Println()
		display.ShowUsage(finalResp.GetUsageMap())
	}
}
