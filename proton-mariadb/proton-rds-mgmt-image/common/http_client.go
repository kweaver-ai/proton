package common

import (
	"bytes"
	"crypto/tls"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"
	"time"

	jsoniter "github.com/json-iterator/go"
)

//go:generate mockgen -package mock -source ./http_client.go -destination ./mock/mock_http_client.go

// HTTPClient HTTP客户端服务接口
type HTTPClient interface {
	GetStream(url string, queryValues url.Values, headers map[string]string) (body []byte, err error)
	Get(url string, queryValues url.Values, headers map[string]string) (respParam interface{}, err error)
	Delete(url string, headers map[string]string) (respCode int, respParam interface{}, err error)
	Post(url string, headers map[string]string, reqParam interface{}) (respCode int, respParam interface{}, err error)
	PostText(url string, headers map[string]string, reqParam []byte) (respCode int, respParam interface{}, err error)
	Put(url string, headers map[string]string, reqParam interface{}) (respCode int, respParam interface{}, err error)
	PutText(url string, headers map[string]string, reqParam []byte) (respCode int, respParam interface{}, err error)
	Patch(url string, headers map[string]string, reqParam interface{}) (respCode int, respParam interface{}, err error)
	PatchText(url string, headers map[string]string, reqParam []byte) (respCode int, respParam interface{}, err error)
	PostNotUnmarshal(url string, headers map[string]string, reqParam interface{}) (respCode int, body []byte, err error)
}

var (
	rawOnce   sync.Once
	rawClient *http.Client
	httpOnce  sync.Once
	client    HTTPClient
)

// httpClient HTTP客户端结构
type httpClient struct {
	client *http.Client
}

// NewRawHTTPClient 创建原生HTTP客户端对象
func NewRawHTTPClient() *http.Client {
	rawOnce.Do(func() {
		rawClient = &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Transport: &http.Transport{
				TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
				MaxIdleConnsPerHost:   100,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
			Timeout: 10 * time.Second,
		}
	})

	return rawClient
}

// NewHTTPClient 创建HTTP客户端对象
func NewHTTPClient() HTTPClient {
	httpOnce.Do(func() {
		client = &httpClient{
			client: NewRawHTTPClient(),
		}
	})

	return client
}

//Get http client get
func (c *httpClient) GetStream(urlStr string, queryValues url.Values, headers map[string]string) (body []byte, err error) {
	uri, err := url.Parse(urlStr)
	if err != nil {
		return
	}

	if queryValues != nil {
		values := uri.Query()
		for k, v := range values {
			queryValues[k] = v
		}
		uri.RawQuery = queryValues.Encode()
	}

	req, err := http.NewRequest("GET", uri.String(), nil)
	if err != nil {
		return
	}

	_, body, err = c.httpDoNoUnmarshal(req, headers)
	return
}

// Get http client get
func (c *httpClient) Get(urlStr string, queryValues url.Values, headers map[string]string) (respParam interface{}, err error) {
	uri, err := url.Parse(urlStr)
	if err != nil {
		return
	}

	if queryValues != nil {
		values := uri.Query()
		for k, v := range values {
			queryValues[k] = v
		}
		uri.RawQuery = queryValues.Encode()
	}

	req, err := http.NewRequest("GET", uri.String(), nil)
	if err != nil {
		return
	}

	_, respParam, err = c.httpDo(req, headers)
	return
}

// Post http client post
func (c *httpClient) Post(url string, headers map[string]string, reqParam interface{}) (respCode int, respParam interface{}, err error) {
	var reqBody []byte
	if v, ok := reqParam.([]byte); ok {
		reqBody = v
	} else {
		reqBody, err = jsoniter.Marshal(reqParam)
		if err != nil {
			return
		}
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return
	}

	respCode, respParam, err = c.httpDo(req, headers)
	return
}

//post请求返回原始body
func (c *httpClient) PostNotUnmarshal(url string, headers map[string]string, reqParam interface{}) (respCode int, body []byte, err error) {
	var reqBody []byte
	if v, ok := reqParam.([]byte); ok {
		reqBody = v
	} else {
		reqBody, err = jsoniter.Marshal(reqParam)
		if err != nil {
			return
		}
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return
	}

	respCode, body, err = c.httpDoNoUnmarshal(req, headers)
	return
}

// Post http client post
func (c *httpClient) PostText(url string, headers map[string]string, reqBody []byte) (respCode int, respParam interface{}, err error) {

	req, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return
	}

	respCode, respParam, err = c.httpDo(req, headers)
	return
}

// Put http client put
func (c *httpClient) PutText(url string, headers map[string]string, reqBody []byte) (respCode int, respParam interface{}, err error) {
	req, err := http.NewRequest("PUT", url, bytes.NewReader(reqBody))
	if err != nil {
		return
	}

	respCode, respParam, err = c.httpDo(req, headers)
	return
}

// Put http client put
func (c *httpClient) Put(url string, headers map[string]string, reqParam interface{}) (respCode int, respParam interface{}, err error) {
	reqBody, err := jsoniter.Marshal(reqParam)
	if err != nil {
		return
	}

	req, err := http.NewRequest("PUT", url, bytes.NewReader(reqBody))
	if err != nil {
		return
	}

	respCode, respParam, err = c.httpDo(req, headers)
	return
}

// Delete http client delete
func (c *httpClient) Delete(url string, headers map[string]string) (respCode int, respParam interface{}, err error) {
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return
	}

	respCode, respParam, err = c.httpDo(req, headers)
	return
}

// Patch http client put
func (c *httpClient) PatchText(url string, headers map[string]string, reqBody []byte) (respCode int, respParam interface{}, err error) {
	req, err := http.NewRequest("PATCH", url, bytes.NewReader(reqBody))
	if err != nil {
		return
	}

	respCode, respParam, err = c.httpDo(req, headers)
	return
}

func (c *httpClient) Patch(url string, headers map[string]string, reqParam interface{}) (respCode int, respParam interface{}, err error) {
	reqBody, err := jsoniter.Marshal(reqParam)
	if err != nil {
		return
	}
	req, err := http.NewRequest("PATCH", url, bytes.NewReader(reqBody))
	if err != nil {
		return
	}

	respCode, respParam, err = c.httpDo(req, headers)
	return
}

func (c *httpClient) httpDo(req *http.Request, headers map[string]string) (respCode int, respParam interface{}, err error) {

	respCode, body, err := c.httpDoNoUnmarshal(req, headers)

	if len(body) != 0 {
		e := jsoniter.Unmarshal(body, &respParam)
		if e != nil {
			err = e
		}
	}

	return
}

//返回原始body, 不进行反序列化
func (c *httpClient) httpDoNoUnmarshal(req *http.Request, headers map[string]string) (respCode int, body []byte, err error) {
	if c.client == nil {
		return 0, nil, errors.New("http client is unavailable")
	}

	c.addHeaders(req, headers)

	resp, err := c.client.Do(req)
	if err != nil {
		return
	}
	defer func() {
		closeErr := resp.Body.Close()
		if closeErr != nil {
			NewLogger().Errorln(closeErr)
		}
	}()
	body, err = ioutil.ReadAll(resp.Body)
	respCode = resp.StatusCode
	if (respCode < http.StatusOK) || (respCode >= http.StatusMultipleChoices) {
		err = HTTPError{
			Message: string(body),
			Code:    respCode,
		}
		return
	}
	return
}

func (c *httpClient) addHeaders(req *http.Request, headers map[string]string) {
	for k, v := range headers {
		if len(v) > 0 {
			req.Header.Add(k, v)
		}
	}
}
