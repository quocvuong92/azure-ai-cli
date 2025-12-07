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

// Error codes that should trigger key rotation
var RotatableErrorCodes = []int{401, 403, 429}

// KeyRotator manages a pool of API keys with rotation support
type KeyRotator struct {
	keys       []string
	currentIdx int
	currentKey string
}

// NewKeyRotator creates a new KeyRotator from an environment variable
func NewKeyRotator(envVar string) *KeyRotator {
	keys := getKeysFromEnv(envVar)
	kr := &KeyRotator{
		keys:       keys,
		currentIdx: 0,
	}
	if len(keys) > 0 {
		kr.currentKey = keys[0]
	}
	return kr
}

// GetCurrentKey returns the current active API key
func (kr *KeyRotator) GetCurrentKey() string {
	return kr.currentKey
}

// GetKeyCount returns the total number of keys
func (kr *KeyRotator) GetKeyCount() int {
	return len(kr.keys)
}

// GetCurrentIndex returns the current key index (0-based)
func (kr *KeyRotator) GetCurrentIndex() int {
	return kr.currentIdx
}

// HasKeys returns true if there are any keys configured
func (kr *KeyRotator) HasKeys() bool {
	return len(kr.keys) > 0
}

// Rotate moves to the next available API key
func (kr *KeyRotator) Rotate() (string, error) {
	if len(kr.keys) <= 1 {
		return "", ErrNoAvailableKeys
	}
	nextIndex := kr.currentIdx + 1
	if nextIndex >= len(kr.keys) {
		return "", ErrNoAvailableKeys
	}
	kr.currentIdx = nextIndex
	kr.currentKey = kr.keys[nextIndex]
	return kr.currentKey, nil
}

// getKeysFromEnv retrieves API keys from an environment variable (comma-separated)
func getKeysFromEnv(envVar string) []string {
	keysEnv := os.Getenv(envVar)
	if keysEnv == "" {
		return nil
	}
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

// Config holds the application configuration
type Config struct {
	// Azure OpenAI (single key)
	AzureEndpoint   string
	AzureAPIKey     string
	Model           string
	AvailableModels []string

	// Key rotators for search providers
	TavilyKeys *KeyRotator
	LinkupKeys *KeyRotator
	BraveKeys  *KeyRotator

	// Legacy fields for backward compatibility (used by API clients)
	TavilyAPIKey        string
	TavilyAPIKeys       []string
	TavilyCurrentKeyIdx int
	LinkupAPIKey        string
	LinkupAPIKeys       []string
	LinkupCurrentKeyIdx int
	BraveAPIKey         string
	BraveAPIKeys        []string
	BraveCurrentKeyIdx  int

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

	// Initialize key rotators
	c.TavilyKeys = NewKeyRotator(EnvTavilyAPIKeys)
	c.LinkupKeys = NewKeyRotator(EnvLinkupAPIKeys)
	c.BraveKeys = NewKeyRotator(EnvBraveAPIKeys)

	// Sync legacy fields for backward compatibility
	c.syncLegacyFields()

	// Set web search provider (default to tavily, or auto-detect based on available keys)
	if c.WebSearchProvider == "" {
		c.WebSearchProvider = os.Getenv(EnvWebSearchProvider)
	}
	if c.WebSearchProvider == "" {
		// Auto-detect: prefer tavily if available, then linkup, then brave
		if c.TavilyKeys.HasKeys() {
			c.WebSearchProvider = "tavily"
		} else if c.LinkupKeys.HasKeys() {
			c.WebSearchProvider = "linkup"
		} else if c.BraveKeys.HasKeys() {
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
		if c.WebSearchProvider == "tavily" && !c.TavilyKeys.HasKeys() {
			return ErrWebSearchKeyNotFound
		}
		if c.WebSearchProvider == "linkup" && !c.LinkupKeys.HasKeys() {
			return ErrWebSearchKeyNotFound
		}
		if c.WebSearchProvider == "brave" && !c.BraveKeys.HasKeys() {
			return ErrWebSearchKeyNotFound
		}
	}

	return nil
}

// syncLegacyFields synchronizes KeyRotator state to legacy fields for backward compatibility
func (c *Config) syncLegacyFields() {
	// Tavily
	c.TavilyAPIKey = c.TavilyKeys.GetCurrentKey()
	c.TavilyAPIKeys = c.TavilyKeys.keys
	c.TavilyCurrentKeyIdx = c.TavilyKeys.GetCurrentIndex()

	// Linkup
	c.LinkupAPIKey = c.LinkupKeys.GetCurrentKey()
	c.LinkupAPIKeys = c.LinkupKeys.keys
	c.LinkupCurrentKeyIdx = c.LinkupKeys.GetCurrentIndex()

	// Brave
	c.BraveAPIKey = c.BraveKeys.GetCurrentKey()
	c.BraveAPIKeys = c.BraveKeys.keys
	c.BraveCurrentKeyIdx = c.BraveKeys.GetCurrentIndex()
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
	key, err := c.TavilyKeys.Rotate()
	if err != nil {
		return "", err
	}
	c.TavilyAPIKey = key
	c.TavilyCurrentKeyIdx = c.TavilyKeys.GetCurrentIndex()
	return key, nil
}

// GetTavilyKeyCount returns the total number of Tavily keys
func (c *Config) GetTavilyKeyCount() int {
	return c.TavilyKeys.GetKeyCount()
}

// RotateLinkupKey moves to the next available Linkup API key
func (c *Config) RotateLinkupKey() (string, error) {
	key, err := c.LinkupKeys.Rotate()
	if err != nil {
		return "", err
	}
	c.LinkupAPIKey = key
	c.LinkupCurrentKeyIdx = c.LinkupKeys.GetCurrentIndex()
	return key, nil
}

// GetLinkupKeyCount returns the total number of Linkup keys
func (c *Config) GetLinkupKeyCount() int {
	return c.LinkupKeys.GetKeyCount()
}

// RotateBraveKey moves to the next available Brave API key
func (c *Config) RotateBraveKey() (string, error) {
	key, err := c.BraveKeys.Rotate()
	if err != nil {
		return "", err
	}
	c.BraveAPIKey = key
	c.BraveCurrentKeyIdx = c.BraveKeys.GetCurrentIndex()
	return key, nil
}

// GetBraveKeyCount returns the total number of Brave keys
func (c *Config) GetBraveKeyCount() int {
	return c.BraveKeys.GetKeyCount()
}
