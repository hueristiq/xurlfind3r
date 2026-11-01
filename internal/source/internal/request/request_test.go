package request_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/hueristiq/xurlfind3r/internal/source/internal/request"
)

type trackingBody struct {
	io.Reader
	closed bool
}

func (b *trackingBody) Close() error {
	b.closed = true

	return nil
}

type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestDoAndDecode(t *testing.T) {
	t.Parallel()

	body := &trackingBody{Reader: strings.NewReader(`{"value":"example.com"}`)}

	client := doerFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s; want POST", r.Method)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}

		if r.URL.Query().Get("k") != "secret" {
			t.Errorf("query = %q", r.URL.RawQuery)
		}

		payload, err := io.ReadAll(r.Body)
		if err != nil || string(payload) != "payload" {
			t.Errorf("request body = %q, %v", payload, err)
		}

		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: body}, nil
	})

	response, err := request.Do(context.Background(), client, http.MethodPost, "https://example.com/search", strings.NewReader("payload"), url.Values{"k": {"secret"}}, http.Header{"Content-Type": {"application/json"}}, "secret") //nolint:bodyclose // request.Decode closes the body.
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	var decoded struct {
		Value string `json:"value"`
	}

	if err := request.Decode(response, &decoded); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if decoded.Value != "example.com" || !body.closed {
		t.Fatalf("decoded = %+v, body closed = %t", decoded, body.closed)
	}
}

func TestDoRedactsTransportErrorsAndClosesResponse(t *testing.T) {
	t.Parallel()

	body := &trackingBody{Reader: strings.NewReader("")}
	original := errors.New("request included secret")

	client := doerFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{Body: body}, original
	})

	_, err := request.Do(context.Background(), client, http.MethodGet, "https://example.com", http.NoBody, nil, nil, "secret") //nolint:bodyclose // The error path closes the body.
	if err == nil || strings.Contains(err.Error(), "secret") || !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatalf("error = %v; want redacted transport error", err)
	}

	if !errors.Is(err, original) {
		t.Fatal("redacted error does not unwrap to the original")
	}

	if !body.closed {
		t.Fatal("response body was not closed")
	}
}

func TestDoRejectsStatusAndRedactsBody(t *testing.T) {
	t.Parallel()

	body := &trackingBody{Reader: strings.NewReader("secret denied")}
	client := doerFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusForbidden, Status: "403 Forbidden", Body: body}, nil
	})

	_, err := request.Do(context.Background(), client, http.MethodGet, "https://example.com", http.NoBody, nil, nil, "secret") //nolint:bodyclose // The error path closes the body.
	if err == nil || strings.Contains(err.Error(), "secret") || !strings.Contains(err.Error(), "403 Forbidden") {
		t.Fatalf("error = %v; want redacted status error", err)
	}

	if !body.closed {
		t.Fatal("response body was not closed")
	}
}

func TestDoRejectsNilClientAndResponse(t *testing.T) {
	t.Parallel()

	if _, err := request.Do(context.Background(), nil, http.MethodGet, "https://example.com", http.NoBody, nil, nil); err == nil { //nolint:bodyclose // Error paths own the response.
		t.Fatal("Do accepted a nil client")
	}

	client := doerFunc(func(*http.Request) (*http.Response, error) {
		return nil, nil //nolint:nilnil // Exercise the empty-response guard.
	})

	if _, err := request.Do(context.Background(), client, http.MethodGet, "https://example.com", http.NoBody, nil, nil); err == nil { //nolint:bodyclose // Error paths own the response.
		t.Fatal("Do accepted an empty response")
	}
}

func TestDecodeClosesBodyOnError(t *testing.T) {
	t.Parallel()

	body := &trackingBody{Reader: strings.NewReader("{")}
	response := &http.Response{Body: body}

	var destination any
	if err := request.Decode(response, &destination); err == nil {
		t.Fatal("Decode accepted malformed JSON")
	}

	if !body.closed {
		t.Fatal("response body was not closed")
	}
}

func TestRead(t *testing.T) {
	t.Parallel()

	body := &trackingBody{Reader: strings.NewReader("content")}
	response := &http.Response{Body: body}

	content, err := request.Read(response)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if string(content) != "content" || !body.closed {
		t.Fatalf("content = %q, body closed = %t", content, body.closed)
	}
}

func TestExecutePreservesStatus(t *testing.T) {
	t.Parallel()

	body := &trackingBody{Reader: strings.NewReader("limited")}
	client := doerFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusTooManyRequests, Status: "429 Too Many Requests", Body: body}, nil
	})

	response, err := request.Execute(context.Background(), client, http.MethodGet, "https://example.com", http.NoBody, nil, nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if response.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d; want 429", response.StatusCode)
	}

	_ = response.Body.Close()

	if !body.closed {
		t.Fatal("response body was not closed")
	}
}
