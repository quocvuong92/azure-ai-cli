package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// Environment variable names
const (
	EnvAzureEndpoint = "AZURE_OPENAI_ENDPOINT"
	EnvAzureAPIKey   = "AZURE_OPENAI_API_KEY"
	EnvAzureModels   = "AZURE_OPENAI_MODELS"
	EnvTavilyAPIKeys = "TAVILY_API_KEYS"
)

// Defaults
const (
	DefaultModel = "gpt-5.1-chat"
)

// Errors
var (
	ErrEndpointNotFound  = errors.New("Azure endpoint not found. Set AZURE_OPENAI_ENDPOINT environment variable")
	ErrAPIKeyNotFound    = errors.New("Azure API key not found. Set AZURE_OPENAI_API_KEY environment variable")
	ErrModelNotFound     = errors.New("model not found. Set AZURE_OPENAI_MODEL or use --model flag")
	ErrInvalidModel      = errors.New("invalid model specified")
	ErrNoAvailableKeys   = errors.New("all API keys exhausted")
	ErrTavilyKeyNotFound = errors.New("Tavily API key not found. Set TAVILY_API_KEYS or TAVILY_API_KEY to use --web flag")
)

// Error codes that should trigger key rotation (for Tavily)
var RotatableErrorCodes = []int{401, 403, 429}

// Config holds the application configuration
type Config struct {
	// Azure OpenAI (single key)
	AzureEndpoint   string
	AzureAPIKey     string
	Model           string
	AvailableModels []string

	// Tavily (multiple keys for free tier rotation)
	TavilyAPIKey        string
	TavilyAPIKeys       []string
	TavilyCurrentKeyIdx int

	// Flags
	Stream      bool
	Render      bool
	Usage       bool
	WebSearch   bool
	Citations   bool // Show citations/sources from web search
	Interactive bool // Interactive chat mode
}

// NewConfig creates a new Config with defaults
func NewConfig() *Config {
	return &Config{}
}

// Validate validates the configuration and loads from environment
func (c *Config) Validate() error {
	// Load Azure endpoint
	if c.AzureEndpoint == "" {
		c.AzureEndpoint = os.Getenv(EnvAzureEndpoint)
	}
	if c.AzureEndpoint == "" {
		return ErrEndpointNotFound
	}
	// Remove trailing slash
	c.AzureEndpoint = strings.TrimSuffix(c.AzureEndpoint, "/")

	// Load Azure API key (single key)
	if c.AzureAPIKey == "" {
		c.AzureAPIKey = strings.TrimSpace(os.Getenv(EnvAzureAPIKey))
	}
	if c.AzureAPIKey == "" {
		return ErrAPIKeyNotFound
	}

	// Load available models
	if modelsEnv := os.Getenv(EnvAzureModels); modelsEnv != "" {
		models := strings.Split(modelsEnv, ",")
		for _, m := range models {
			m = strings.TrimSpace(m)
			if m != "" {
				c.AvailableModels = append(c.AvailableModels, m)
			}
		}
	}

	// Load default model
	if c.Model == "" && len(c.AvailableModels) > 0 {
		c.Model = c.AvailableModels[0]
	}
	if c.Model == "" {
		c.Model = DefaultModel
	}

	// Validate model if available models are configured
	if len(c.AvailableModels) > 0 && !c.ValidateModel(c.Model) {
		return fmt.Errorf("%w: %s. Available: %s", ErrInvalidModel, c.Model, c.GetAvailableModelsString())
	}

	// Load Tavily keys (optional, only required if --web is used)
	c.TavilyAPIKeys = getTavilyKeysFromEnv()
	if len(c.TavilyAPIKeys) > 0 {
		c.TavilyCurrentKeyIdx = 0
		c.TavilyAPIKey = c.TavilyAPIKeys[0]
	}

	// Validate Tavily keys if web search is requested
	if c.WebSearch && len(c.TavilyAPIKeys) == 0 {
		return ErrTavilyKeyNotFound
	}

	return nil
}

// GetAzureAPIURL builds the full API URL for chat completions
func (c *Config) GetAzureAPIURL() string {
	return fmt.Sprintf("%s/openai/v1/chat/completions",
		c.AzureEndpoint)
}

// ValidateModel checks if the given model is in available models
func (c *Config) ValidateModel(model string) bool {
	if len(c.AvailableModels) == 0 {
		return true // No validation if models not configured
	}
	for _, m := range c.AvailableModels {
		if m == model {
			return true
		}
	}
	return false
}

// GetAvailableModelsString returns a formatted string of available models
func (c *Config) GetAvailableModelsString() string {
	if len(c.AvailableModels) == 0 {
		return "(not configured - set AZURE_OPENAI_MODELS)"
	}
	return strings.Join(c.AvailableModels, ", ")
}

// RotateTavilyKey moves to the next available Tavily API key
func (c *Config) RotateTavilyKey() (string, error) {
	if len(c.TavilyAPIKeys) <= 1 {
		return "", ErrNoAvailableKeys
	}
	nextIndex := c.TavilyCurrentKeyIdx + 1
	if nextIndex >= len(c.TavilyAPIKeys) {
		return "", ErrNoAvailableKeys
	}
	c.TavilyCurrentKeyIdx = nextIndex
	c.TavilyAPIKey = c.TavilyAPIKeys[nextIndex]
	return c.TavilyAPIKey, nil
}

// GetTavilyKeyCount returns the total number of Tavily keys
func (c *Config) GetTavilyKeyCount() int {
	return len(c.TavilyAPIKeys)
}

// getTavilyKeysFromEnv retrieves Tavily API keys from environment variable
func getTavilyKeysFromEnv() []string {
	if keysEnv := os.Getenv(EnvTavilyAPIKeys); keysEnv != "" {
		keys := strings.Split(keysEnv, ",")
		var result []string
		for _, key := range keys {
			key = strings.TrimSpace(key)
			if key != "" {
				result = append(result, key)
			}
		}
		return result
	}
	return nil
}
