package session

import (
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

type Keys struct {
	GitHub     []string `json:"github"`
	Intelx     string   `json:"intelx"` // unused, add just for the purpose of adding * on listing sources
	IntelXHost string   `json:"intelXHost"`
	IntelXKey  string   `json:"intelXKey"`
}

type Scope struct {
	Domain            string
	FilterRegex       *regexp.Regexp
	IncludeSubdomains bool
}

type Session struct {
	Client *http.Client
	Keys   Keys
	Scope  Scope
}

func New(domain string, filterRegex *regexp.Regexp, includeSubdomains bool, timeout int, keys Keys) (*Session, error) {
	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Timeout: time.Duration(timeout) * time.Second,
	}

	return &Session{
		Client: client,
		Keys:   keys,
		Scope: Scope{
			Domain:            domain,
			FilterRegex:       filterRegex,
			IncludeSubdomains: includeSubdomains,
		},
	}, nil
}

func (session *Session) Get(getURL string, headers map[string]string) (*http.Response, error) {
	return session.HTTPRequest(http.MethodGet, getURL, headers, nil)
}

func (session *Session) SimpleGet(getURL string) (*http.Response, error) {
	return session.HTTPRequest(http.MethodGet, getURL, map[string]string{}, nil)
}

func (session *Session) Post(postURL, cookies string, headers map[string]string, body io.Reader) (*http.Response, error) {
	return session.HTTPRequest(http.MethodPost, postURL, headers, body)
}

func (session *Session) SimplePost(postURL, contentType string, body io.Reader) (*http.Response, error) {
	return session.HTTPRequest(http.MethodPost, postURL, map[string]string{"Content-Type": contentType}, body)
}

func (session *Session) HTTPRequest(method, requestURL string, headers map[string]string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, requestURL, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/78.0.3904.108 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en")
	req.Header.Set("Connection", "close")

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return httpRequestWrapper(session.Client, req)
}

func (session *Session) DiscardHTTPResponse(response *http.Response) {
	if response != nil {
		_, err := io.Copy(ioutil.Discard, response.Body)
		if err != nil {
			return
		}

		response.Body.Close()
	}
}

func httpRequestWrapper(client *http.Client, request *http.Request) (*http.Response, error) {
	res, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		requestURL, _ := url.QueryUnescape(request.URL.String())
		return res, fmt.Errorf("unexpected status code %d received from %s", res.StatusCode, requestURL)
	}
	return res, nil
}
