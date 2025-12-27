package providers

import "fmt"

// ProviderError represents an error from a provider
type ProviderError struct {
	Provider   string
	StatusCode int
	Code       string
	Message    string
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("%s error (%d): %s - %s", e.Provider, e.StatusCode, e.Code, e.Message)
}
