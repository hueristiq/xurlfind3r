// Package request implements the shared HTTP transport policy for providers.
package request

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	hqgosource "github.com/hueristiq/xurlfind3r/internal/source"
)

const maxResponseSize = 16 << 20

// Do builds and executes one request, rejecting non-2xx responses and closing
// failed responses. The caller owns a successful response body.
func Do(ctx context.Context, client hqgosource.Doer, method, endpoint string, body io.Reader, query url.Values, headers http.Header, secrets ...string) (*http.Response, error) {
	response, err := Execute(ctx, client, method, endpoint, body, query, headers, secrets...)
	if err != nil {
		return nil, err
	}

	if err := Check(response, secrets...); err != nil {
		if response.Body != nil {
			_ = response.Body.Close()
		}

		return nil, err
	}

	return response, nil
}

// Execute builds and executes one request without rejecting its HTTP status.
// The caller owns the response body and must apply its own status policy.
func Execute(ctx context.Context, client hqgosource.Doer, method, endpoint string, body io.Reader, query url.Values, headers http.Header, secrets ...string) (*http.Response, error) {
	if client == nil {
		return nil, errors.New("source: nil HTTP client")
	}

	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("source: build request: %w", err)
	}

	if query != nil {
		request.URL.RawQuery = query.Encode()
	}

	for name, values := range headers {
		for _, value := range values {
			request.Header.Add(name, value)
		}
	}

	response, err := client.Do(request)
	if err != nil {
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}

		return nil, Redact(fmt.Errorf("source: request failed: %w", err), secrets...)
	}

	if response == nil {
		return nil, errors.New("source: empty response")
	}

	return response, nil
}

// Check rejects a nil response or non-2xx status and bounds error body excerpts.
func Check(response *http.Response, secrets ...string) error {
	if response == nil {
		return errors.New("source: empty response")
	}

	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		return nil
	}

	snippet := ""

	if response.Body != nil {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4<<10))
		snippet = strings.TrimSpace(string(body))
	}

	message := "unexpected status " + response.Status
	if snippet != "" {
		message += ": " + snippet
	}

	return Redact(errors.New(message), secrets...)
}

// Decode reads one bounded JSON value and closes the response body.
func Decode(response *http.Response, destination any) error {
	if response == nil || response.Body == nil {
		return errors.New("source: empty response body")
	}

	defer func() {
		_ = response.Body.Close()
	}()

	if err := json.NewDecoder(io.LimitReader(response.Body, maxResponseSize)).Decode(destination); err != nil {
		return fmt.Errorf("source: decode JSON response: %w", err)
	}

	return nil
}

// Read reads a bounded response body and closes it.
func Read(response *http.Response) ([]byte, error) {
	if response == nil || response.Body == nil {
		return nil, errors.New("source: empty response body")
	}

	defer func() {
		_ = response.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseSize+1))
	if err != nil {
		return nil, fmt.Errorf("source: read response: %w", err)
	}

	if len(body) > maxResponseSize {
		return nil, fmt.Errorf("source: response exceeds %d bytes", maxResponseSize)
	}

	return body, nil
}

type redactedError struct {
	err     error
	message string
}

func (e *redactedError) Error() string { return e.message }
func (e *redactedError) Unwrap() error { return e.err }

// Redact replaces plaintext and URL-escaped secrets in an error message while
// retaining the original error for errors.Is and errors.As.
func Redact(err error, secrets ...string) error {
	if err == nil {
		return nil
	}

	message := err.Error()

	for _, secret := range secrets {
		if secret == "" {
			continue
		}

		message = strings.ReplaceAll(message, secret, "[REDACTED]")
		message = strings.ReplaceAll(message, url.QueryEscape(secret), "[REDACTED]")
	}

	if message == err.Error() {
		return err
	}

	return &redactedError{err: err, message: message}
}
