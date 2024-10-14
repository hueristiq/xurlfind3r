package httpclient

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	hqgohttp "github.com/hueristiq/hq-go-http"
	"github.com/hueristiq/hq-go-http/methods"
	"github.com/hueristiq/hq-go-http/status"
	"github.com/hueristiq/xurlfind3r/internal/configuration"
)

var client *hqgohttp.Client

func init() {
	cfg := hqgohttp.DefaultSprayingClientConfiguration
	cfg.Timeout = 1 * time.Hour

	client, _ = hqgohttp.NewClient(cfg)
}

func httpRequestWrapper(req *hqgohttp.Request) (res *http.Response, err error) {
	res, err = client.Do(req)
	if err != nil {
		return
	}

	if res.StatusCode != status.OK {
		requestURL, _ := url.QueryUnescape(req.URL.String())

		err = fmt.Errorf("unexpected status code %d received from %s", res.StatusCode, requestURL)

		return
	}

	return
}

// HTTPRequest makes any HTTP request to a URL with extended parameters
func HTTPRequest(method, requestURL, cookies string, headers map[string]string, body io.Reader) (*http.Response, error) {
	req, err := hqgohttp.NewRequest(method, requestURL, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en")
	req.Header.Set("User-Agent", fmt.Sprintf("%s v%s (https://github.com/hueristiq/%s)", configuration.NAME, configuration.VERSION, configuration.NAME))
	// req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:130.0) Gecko/20100101 Firefox/130.0")

	if cookies != "" {
		req.Header.Set("Cookie", cookies)
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return httpRequestWrapper(req)
}

// Get makes a GET request to a URL with extended parameters
func Get(URL, cookies string, headers map[string]string) (*http.Response, error) {
	return HTTPRequest(methods.Get, URL, cookies, headers, nil)
}

// SimpleGet makes a simple GET request to a URL
func SimpleGet(URL string) (*http.Response, error) {
	return HTTPRequest(methods.Get, URL, "", map[string]string{}, nil)
}

// Post makes a POST request to a URL with extended parameters
func Post(URL, cookies string, headers map[string]string, body io.Reader) (*http.Response, error) {
	return HTTPRequest(methods.Post, URL, cookies, headers, body)
}

func DiscardResponse(response *http.Response) {
	if response != nil {
		_, err := io.Copy(io.Discard, response.Body)
		if err != nil {
			return
		}

		response.Body.Close()
	}
}
