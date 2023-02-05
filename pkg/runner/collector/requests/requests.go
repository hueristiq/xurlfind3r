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
