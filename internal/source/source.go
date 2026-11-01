// Package source defines the shared contracts for remote data providers.
package source

import "net/http"

// Doer executes an HTTP request. Implementations must honor request context
// cancellation and return responses whose bodies the caller can close.
type Doer interface {
	Do(request *http.Request) (*http.Response, error)
}

// Result is one provider value, warning, or error. Exactly one of Value,
// Warning, and Err is meaningful in a result. Kind optionally distinguishes
// values within a provider.
type Result struct {
	Value   string
	Warning string
	Err     error
	Kind    string
}

// DoerFunc adapts a function to the Doer interface.
type DoerFunc func(*http.Request) (*http.Response, error)

// Do executes the function as an HTTP request.
func (f DoerFunc) Do(request *http.Request) (*http.Response, error) {
	return f(request)
}
