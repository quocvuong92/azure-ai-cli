package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// Environment variable names
const (
	EnvAzureEndpoint     = "AZURE_OPENAI_ENDPOINT"
	EnvAzureAPIKey       = "AZURE_OPENAI_API_KEY"
	EnvAzureModels       = "AZURE_OPENAI_MODELS"
	EnvTavilyAPIKeys     = "TAVILY_API_KEYS"
	EnvLinkupAPIKeys     = "LINKUP_API_KEYS"
	EnvBraveAPIKeys      = "BRAVE_API_KEYS"
	EnvWebSearchProvider = "WEB_SEARCH_PROVIDER"
)

// Defaults
const (
	DefaultModel          = "gpt-5.1-chat"
	DefaultSystemMessage  = "Be precise and concise."
	DefaultSearchProvider = "tavily"
)

// Errors
var (
	ErrEndpointNotFound      = errors.New("Azure endpoint not found. Set AZURE_OPENAI_ENDPOINT environment variable")
	ErrAPIKeyNotFound        = errors.New("Azure API key not found. Set AZURE_OPENAI_API_KEY environment variable")
	ErrModelNotFound         = errors.New("model not found. Set AZURE_OPENAI_MODEL or use --model flag")
	ErrInvalidModel          = errors.New("invalid model specified")
	ErrNoAvailableKeys       = errors.New("all API keys exhausted")
	ErrWebSearchKeyNotFound  = errors.New("web search API key not found. Set TAVILY_API_KEYS, LINKUP_API_KEYS, or BRAVE_API_KEYS to use --web flag")
	ErrInvalidSearchProvider = errors.New("invalid search provider. Use 'tavily', 'linkup', or 'brave'")
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

	// Linkup (multiple keys for free tier rotation)
	LinkupAPIKey        string
	LinkupAPIKeys       []string
	LinkupCurrentKeyIdx int

	// Brave (multiple keys for free tier rotation)
	BraveAPIKey        string
	BraveAPIKeys       []string
	BraveCurrentKeyIdx int

	// Web search provider selection
	WebSearchProvider string // "tavily", "linkup", or "brave"

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

	// Load Tavily keys (optional, only required if --web is used with tavily provider)
	c.TavilyAPIKeys = getTavilyKeysFromEnv()
	if len(c.TavilyAPIKeys) > 0 {
		c.TavilyCurrentKeyIdx = 0
		c.TavilyAPIKey = c.TavilyAPIKeys[0]
	}

	// Load Linkup keys (optional, only required if --web is used with linkup provider)
	c.LinkupAPIKeys = getLinkupKeysFromEnv()
	if len(c.LinkupAPIKeys) > 0 {
		c.LinkupCurrentKeyIdx = 0
		c.LinkupAPIKey = c.LinkupAPIKeys[0]
	}

	// Load Brave keys (optional, only required if --web is used with brave provider)
	c.BraveAPIKeys = getBraveKeysFromEnv()
	if len(c.BraveAPIKeys) > 0 {
		c.BraveCurrentKeyIdx = 0
		c.BraveAPIKey = c.BraveAPIKeys[0]
	}

	// Set web search provider (default to tavily, or auto-detect based on available keys)
	if c.WebSearchProvider == "" {
		c.WebSearchProvider = os.Getenv(EnvWebSearchProvider)
	}
	if c.WebSearchProvider == "" {
		// Auto-detect: prefer tavily if available, then linkup, then brave
		if len(c.TavilyAPIKeys) > 0 {
			c.WebSearchProvider = "tavily"
		} else if len(c.LinkupAPIKeys) > 0 {
			c.WebSearchProvider = "linkup"
		} else if len(c.BraveAPIKeys) > 0 {
			c.WebSearchProvider = "brave"
		} else {
			c.WebSearchProvider = DefaultSearchProvider
		}
	}

	// Validate provider
	if c.WebSearchProvider != "tavily" && c.WebSearchProvider != "linkup" && c.WebSearchProvider != "brave" {
		return ErrInvalidSearchProvider
	}

	// Validate web search keys if web search is requested
	if c.WebSearch {
		if c.WebSearchProvider == "tavily" && len(c.TavilyAPIKeys) == 0 {
			return ErrWebSearchKeyNotFound
		}
		if c.WebSearchProvider == "linkup" && len(c.LinkupAPIKeys) == 0 {
			return ErrWebSearchKeyNotFound
		}
		if c.WebSearchProvider == "brave" && len(c.BraveAPIKeys) == 0 {
			return ErrWebSearchKeyNotFound
		}
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

// RotateLinkupKey moves to the next available Linkup API key
func (c *Config) RotateLinkupKey() (string, error) {
	if len(c.LinkupAPIKeys) <= 1 {
		return "", ErrNoAvailableKeys
	}
	nextIndex := c.LinkupCurrentKeyIdx + 1
	if nextIndex >= len(c.LinkupAPIKeys) {
		return "", ErrNoAvailableKeys
	}
	c.LinkupCurrentKeyIdx = nextIndex
	c.LinkupAPIKey = c.LinkupAPIKeys[nextIndex]
	return c.LinkupAPIKey, nil
}

// GetLinkupKeyCount returns the total number of Linkup keys
func (c *Config) GetLinkupKeyCount() int {
	return len(c.LinkupAPIKeys)
}

// getLinkupKeysFromEnv retrieves Linkup API keys from environment variable
func getLinkupKeysFromEnv() []string {
	if keysEnv := os.Getenv(EnvLinkupAPIKeys); keysEnv != "" {
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

// RotateBraveKey moves to the next available Brave API key
func (c *Config) RotateBraveKey() (string, error) {
	if len(c.BraveAPIKeys) <= 1 {
		return "", ErrNoAvailableKeys
	}
	nextIndex := c.BraveCurrentKeyIdx + 1
	if nextIndex >= len(c.BraveAPIKeys) {
		return "", ErrNoAvailableKeys
	}
	c.BraveCurrentKeyIdx = nextIndex
	c.BraveAPIKey = c.BraveAPIKeys[nextIndex]
	return c.BraveAPIKey, nil
}

// GetBraveKeyCount returns the total number of Brave keys
func (c *Config) GetBraveKeyCount() int {
	return len(c.BraveAPIKeys)
}

// getBraveKeysFromEnv retrieves Brave API keys from environment variable
func getBraveKeysFromEnv() []string {
	if keysEnv := os.Getenv(EnvBraveAPIKeys); keysEnv != "" {
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
