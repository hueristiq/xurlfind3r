package httpclient

import (
	"github.com/valyala/fasthttp"
)

var (
	client = &fasthttp.Client{}
)

func httpRequestWrapper(req *fasthttp.Request) (res *fasthttp.Response, err error) {
	res = fasthttp.AcquireResponse()

	if err = client.Do(req, res); err != nil {
		return
	}

	return
}

func Request(method, URL, cookies string, headers map[string]string, body []byte) (res *fasthttp.Response, err error) {
	req := fasthttp.AcquireRequest()

	req.SetRequestURI(URL)
	req.SetBody(body)
	req.Header.SetMethod(method)

	var agent string

	agent, err = UserAgent()
	if err != nil {
		return
	}

	req.Header.Set("User-Agent", agent)
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
