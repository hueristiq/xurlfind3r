package requests

import (
	"fmt"

	"github.com/corpix/uarand"
	"github.com/valyala/fasthttp"
)

var (
	client = &fasthttp.Client{}
)

func httpRequestWrapper(req *fasthttp.Request) (res *fasthttp.Response, err error) {
	res = fasthttp.AcquireResponse()

	client.Do(req, res)

	if res.StatusCode() != fasthttp.StatusOK {
		err = fmt.Errorf("Unexpected status code")
	}

	return
}

func Request(method, URL, cookies string, headers map[string]string, body []byte) (*fasthttp.Response, error) {
	req := fasthttp.AcquireRequest()

	req.SetRequestURI(URL)
	req.SetBody(body)
	req.Header.SetMethod(method)

	req.Header.Set("User-Agent", uarand.GetRandom())
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en")
	req.Header.Set("Connection", "close")

	if cookies != "" {
		req.Header.Set("Cookie", cookies)
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return httpRequestWrapper(req)
}

func SimpleGet(URL string) (*fasthttp.Response, error) {
	return Request(fasthttp.MethodGet, URL, "", map[string]string{}, nil)
}

func Get(URL, cookies string, headers map[string]string) (*fasthttp.Response, error) {
	return Request(fasthttp.MethodGet, URL, cookies, headers, nil)
}

func SimplePost(URL, contentType string, body []byte) (*fasthttp.Response, error) {
	return Request(fasthttp.MethodPost, URL, "", map[string]string{"Content-Type": contentType}, body)
}

func Post(URL, cookies string, headers map[string]string, body []byte) (*fasthttp.Response, error) {
	return Request(fasthttp.MethodPost, URL, cookies, headers, body)
}

// type Keys struct {
// 	GitHub     []string `json:"github"`
// 	Intelx     string   `json:"intelx"` // unused, add for the purpose of adding an asterisk `*` on listing sources
// 	IntelXHost string   `json:"intelXHost"`
// 	IntelXKey  string   `json:"intelXKey"`
// }

// type Scope struct {
// 	Domain            string
// 	FilterRegex       *regexp.Regexp
// 	IncludeSubdomains bool
// }

// type Session struct {
// 	Client *fasthttp.Client
// 	Keys   Keys
// 	Scope  Scope
// }

// func New(domain string, filterRegex *regexp.Regexp, includeSubdomains bool, timeout int, keys Keys) (session *Session, err error) {
// 	client := &fasthttp.Client{}

// 	session = &Session{
// 		Client: client,
// 		Keys:   keys,
// 		Scope: Scope{
// 			Domain:            domain,
// 			FilterRegex:       filterRegex,
// 			IncludeSubdomains: includeSubdomains,
// 		},
// 	}

// 	return
// }

// Get makes a GET request to a URL with extended parameters
// func (session *Session) Get(URL, cookies string, headers map[string]string) (*fasthttp.Response, error) {
// 	return session.Request(fasthttp.MethodGet, URL, cookies, headers, nil)
// }

// SimpleGet makes a simple GET request to a URL
// func (session *Session) SimpleGet(URL string) (*fasthttp.Response, error) {
// 	return session.Request(fasthttp.MethodGet, URL, "", map[string]string{}, nil)
// }

// Post makes a POST request to a URL with extended parameters
// func (session *Session) Post(URL, cookies string, headers map[string]string, body []byte) (*fasthttp.Response, error) {
// 	return session.Request(fasthttp.MethodPost, URL, cookies, headers, body)
// }

// SimplePost makes a simple POST request to a URL
// func (session *Session) SimplePost(URL, contentType string, body []byte) (*fasthttp.Response, error) {
// 	return session.Request(fasthttp.MethodPost, URL, "", map[string]string{"Content-Type": contentType}, body)
// }

// Request makes any HTTP request to a URL with extended parameters
// func (session *Session) Request(method, URL, cookies string, headers map[string]string, body []byte) (*fasthttp.Response, error) {
// 	req := fasthttp.AcquireRequest()

// 	req.SetRequestURI(URL)
// 	req.SetBody(body)
// 	req.Header.SetMethod(method)

// 	req.Header.Set("User-Agent", uarand.GetRandom())
// 	req.Header.Set("Accept", "*/*")
// 	req.Header.Set("Accept-Language", "en")
// 	req.Header.Set("Connection", "close")

// 	if cookies != "" {
// 		req.Header.Set("Cookie", cookies)
// 	}

// 	for key, value := range headers {
// 		req.Header.Set(key, value)
// 	}

// 	return httpRequestWrapper(session.Client, req)
// }

// func httpRequestWrapper(client *fasthttp.Client, req *fasthttp.Request) (res *fasthttp.Response, err error) {
// 	res = fasthttp.AcquireResponse()

// 	client.Do(req, res)

// 	if res.StatusCode() != fasthttp.StatusOK {
// 		err = fmt.Errorf("Unexpected status code")
// 	}

// 	return
// }
