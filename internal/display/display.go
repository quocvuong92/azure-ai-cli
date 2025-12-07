package display

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	"github.com/charmbracelet/glamour"
)

// renderer is the markdown renderer instance
var (
	renderer     *glamour.TermRenderer
	rendererOnce sync.Once
	rendererErr  error
)

// Spinner wraps the spinner with elapsed time display
type Spinner struct {
	s         *spinner.Spinner
	startTime time.Time
	message   string
	stopChan  chan struct{}
	wg        sync.WaitGroup
	stopped   bool
	mu        sync.Mutex
}

// NewSpinner creates a new spinner with the given message
func NewSpinner(message string) *Spinner {
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = fmt.Sprintf(" %s (0.0s)", message)
	s.Writer = os.Stderr
	return &Spinner{
		s:        s,
		message:  message,
		stopChan: make(chan struct{}),
	}
}

// Start begins the spinner animation
func (sp *Spinner) Start() {
	sp.startTime = time.Now()
	sp.s.Start()

	sp.wg.Add(1)
	go func() {
		defer sp.wg.Done()
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-sp.stopChan:
				return
			case <-ticker.C:
				elapsed := time.Since(sp.startTime).Seconds()
				sp.s.Suffix = fmt.Sprintf(" %s (%.1fs)", sp.message, elapsed)
			}
		}
	}()
}

// Stop stops the spinner and clears the line
func (sp *Spinner) Stop() {
	sp.mu.Lock()
	if sp.stopped {
		sp.mu.Unlock()
		return
	}
	sp.stopped = true
	sp.mu.Unlock()

	close(sp.stopChan)
	sp.wg.Wait()
	sp.s.Stop()
}

// UpdateMessage updates the spinner message while keeping it running
func (sp *Spinner) UpdateMessage(message string) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	if sp.stopped {
		return
	}
	sp.message = message
	elapsed := time.Since(sp.startTime).Seconds()
	sp.s.Suffix = fmt.Sprintf(" %s (%.1fs)", message, elapsed)
}

// InitRenderer initializes the markdown renderer
func InitRenderer() error {
	rendererOnce.Do(func() {
		r, err := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(100),
		)
		if err != nil {
			rendererErr = err
			return
		}
		renderer = r
	})
	return rendererErr
}

// ShowUsage displays token usage statistics
func ShowUsage(usage map[string]int) {
	fmt.Println("## Tokens")
	fmt.Println()
	fmt.Println("| Type | Count |")
	fmt.Println("|------|-------|")
	fmt.Printf("| Input | %d |\n", usage["input_tokens"])
	fmt.Printf("| Output | %d |\n", usage["output_tokens"])
	fmt.Printf("| **Total** | **%d** |\n", usage["total_tokens"])
	fmt.Println()
}

// ShowContent displays the main content response
func ShowContent(content string) {
	fmt.Println(strings.TrimSpace(content))
}

// ShowContentRendered displays markdown content with terminal rendering
func ShowContentRendered(content string) {
	if renderer == nil {
		ShowContent(content)
		return
	}
	rendered, err := renderer.Render(content)
	if err != nil {
		ShowContent(content)
		return
	}
	fmt.Print(strings.TrimSuffix(rendered, "\n"))
}

// ShowError displays an error message
func ShowError(message string) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", message)
}

// ShowKeyRotation displays a message when API key is rotated
func ShowKeyRotation(service string, fromIndex, toIndex, totalKeys int) {
	fmt.Fprintf(os.Stderr, "Note: %s API key %d/%d failed, switching to key %d/%d\n",
		service, fromIndex, totalKeys, toIndex, totalKeys)
}

// ShowWebSearching displays a message when web search starts
func ShowWebSearching(query string) {
	fmt.Fprintf(os.Stderr, "Searching web for: %s\n", query)
}

// ShowWebResults displays the number of web results found
func ShowWebResults(count int) {
	fmt.Fprintf(os.Stderr, "Found %d results\n", count)
}

// ShowModels displays available models
func ShowModels(models []string, currentModel string) {
	fmt.Println("Available models:")
	for _, m := range models {
		if m == currentModel {
			fmt.Printf("  * %s (current)\n", m)
		} else {
			fmt.Printf("    %s\n", m)
		}
	}
}

// Citation represents a source citation
type Citation struct {
	Title string
	URL   string
}

// ShowCitations displays the source citations from web search
func ShowCitations(citations []Citation) {
	fmt.Println("## Sources")
	fmt.Println()
	for i, c := range citations {
		fmt.Printf("[%d] %s - %s\n", i+1, c.Title, c.URL)
	}
}
