package api

import (
	"time"

	"github.com/quocvuong92/azure-ai-cli/internal/config"
)

// Retry configuration constants
const (
	MaxRetryAttempts  = 5
	InitialBackoff    = 100 * time.Millisecond
	MaxBackoff        = 2 * time.Second
	BackoffMultiplier = 2.0
)

// ShouldRotateKey checks if the error status code indicates we should try another key
func ShouldRotateKey(statusCode int) bool {
	for _, code := range config.RotatableErrorCodes {
		if statusCode == code {
			return true
		}
	}
	return false
}

// CalculateBackoff returns the backoff duration for a given attempt number
func CalculateBackoff(attempt int) time.Duration {
	backoff := InitialBackoff
	for i := 0; i < attempt; i++ {
		backoff = time.Duration(float64(backoff) * BackoffMultiplier)
		if backoff > MaxBackoff {
			backoff = MaxBackoff
			break
		}
	}
	return backoff
}
