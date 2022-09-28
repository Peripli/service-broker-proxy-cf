/*
 * The MIT License
 *
 * Copyright (c) 2017 Long Nguyen
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 */

package cfclient

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// Client used to communicate with Cloud Foundry
type Client struct {
	Config   Config
	Endpoint Endpoints
}

type Endpoints struct {
	Links Links `json:"links"`
}

type Links struct {
	AuthEndpoint  EndpointUrl `json:"login"`
	TokenEndpoint EndpointUrl `json:"uaa"`
}

type EndpointUrl struct {
	URL string `json:"href"`
}

// Config is used to configure the creation of a client
type Config struct {
	ApiAddress          string `json:"api_url"`
	Username            string `json:"user"`
	Password            string `json:"password"`
	ClientID            string `json:"client_id"`
	ClientSecret        string `json:"client_secret"`
	SkipSslValidation   bool   `json:"skip_ssl_validation"`
	HttpClient          *http.Client
	Token               string `json:"auth_token"`
	TokenSource         oauth2.TokenSource
	tokenSourceDeadline *time.Time
	UserAgent           string `json:"user_agent"`
	Origin              string `json:"-"`
}

type LoginHint struct {
	Origin string `json:"origin"`
}

// Request is used to help build up a request
type Request struct {
	method string
	url    string
	params url.Values
	body   io.Reader
	obj    interface{}
}

// NewClient returns a new client
func NewClient(config *Config) (client *Client, err error) {
	// bootstrap the config
	defConfig := DefaultConfig()

	if len(config.ApiAddress) == 0 {
		config.ApiAddress = defConfig.ApiAddress
	}

	if len(config.Username) == 0 {
		config.Username = defConfig.Username
	}

	if len(config.Password) == 0 {
		config.Password = defConfig.Password
	}

	if len(config.Token) == 0 {
		config.Token = defConfig.Token
	}

	if len(config.UserAgent) == 0 {
		config.UserAgent = defConfig.UserAgent
	}

	if config.HttpClient == nil {
		config.HttpClient = defConfig.HttpClient
	}

	if config.HttpClient.Transport == nil {
		config.HttpClient.Transport = shallowDefaultTransport()
	}

	var tp *http.Transport

	switch t := config.HttpClient.Transport.(type) {
	case *http.Transport:
		tp = t
	case *oauth2.Transport:
		if bt, ok := t.Base.(*http.Transport); ok {
			tp = bt
		}
	}

	if tp != nil {
		if tp.TLSClientConfig == nil {
			tp.TLSClientConfig = &tls.Config{}
		}
		tp.TLSClientConfig.InsecureSkipVerify = config.SkipSslValidation
	}

	config.ApiAddress = strings.TrimRight(config.ApiAddress, "/")

	client = &Client{
		Config: *config,
	}

	if err := client.refreshEndpoint(); err != nil {
		return nil, err
	}

	return client, nil
}

// DefaultConfig creates a default config object used by CF client
func DefaultConfig() *Config {
	return &Config{
		ApiAddress:        "http://api.bosh-lite.com",
		Username:          "admin",
		Password:          "admin",
		Token:             "",
		SkipSslValidation: false,
		HttpClient:        http.DefaultClient,
		UserAgent:         "SM-CF-client/1.0",
	}
}

// NewRequest is used to create a new Request
func (c *Client) NewRequest(method, path string) *Request {
	requestUrl := path
	if !strings.HasPrefix(path, "http") {
		requestUrl = c.Config.ApiAddress + path
	}

	r := &Request{
		method: method,
		url:    requestUrl,
		params: make(map[string][]string),
	}
	return r
}

// NewRequestWithBody is used to create a new request with
func (c *Client) NewRequestWithBody(method, path string, body io.Reader) *Request {
	r := c.NewRequest(method, path)
	r.body = body

	return r
}

// DoRequest runs a request with our client
func (c *Client) DoRequest(r *Request) (*http.Response, error) {
	req, err := r.toHTTP()
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", c.Config.UserAgent)
	if req.Body != nil && req.Header.Get("Content-type") == "" {
		req.Header.Set("Content-type", "application/json")
	}

	resp, err := c.Config.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return c.handleError(resp)
	}

	return resp, nil
}

func (c *Client) refreshEndpoint() error {
	// we want to keep the Timeout value from config.HttpClient
	timeout := c.Config.HttpClient.Timeout

	ctx := context.Background()
	ctx = context.WithValue(ctx, oauth2.HTTPClient, c.Config.HttpClient)

	endpoint, err := getInfo(c.Config.ApiAddress, oauth2.NewClient(ctx, nil))

	if err != nil {
		return errors.Wrap(err, "Could not get endpoints from the root call")
	}

	switch {
	case c.Config.Token != "":
		c.Config = getUserTokenAuth(ctx, c.Config, endpoint)
	case c.Config.ClientID != "":
		c.Config = getClientAuth(ctx, c.Config, endpoint)
	default:
		c.Config, err = getUserAuth(ctx, c.Config, endpoint)
		if err != nil {
			return err
		}
	}
	// make sure original Timeout value will be used
	if c.Config.HttpClient.Timeout != timeout {
		c.Config.HttpClient.Timeout = timeout
	}

	c.Endpoint = *endpoint
	return nil
}

// getUserTokenAuth initializes client credentials from existing bearer token.
func getUserTokenAuth(ctx context.Context, config Config, endpoints *Endpoints) Config {
	authConfig := &oauth2.Config{
		ClientID: "cf",
		Scopes:   []string{""},
		Endpoint: oauth2.Endpoint{
			AuthURL:  endpoints.Links.AuthEndpoint.URL + "/oauth/auth",
			TokenURL: endpoints.Links.TokenEndpoint.URL + "/oauth/token",
		},
	}

	// Token is expected to have no "bearer" prefix
	token := &oauth2.Token{
		AccessToken: config.Token,
		TokenType:   "Bearer"}

	config.TokenSource = authConfig.TokenSource(ctx, token)
	config.HttpClient = oauth2.NewClient(ctx, config.TokenSource)

	return config
}

func getClientAuth(ctx context.Context, config Config, endpoints *Endpoints) Config {
	authConfig := &clientcredentials.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		TokenURL:     endpoints.Links.TokenEndpoint.URL + "/oauth/token",
	}

	config.TokenSource = authConfig.TokenSource(ctx)
	config.HttpClient = authConfig.Client(ctx)
	return config
}

func getUserAuth(ctx context.Context, config Config, endpoints *Endpoints) (Config, error) {
	authConfig := &oauth2.Config{
		ClientID: "cf",
		Scopes:   []string{""},
		Endpoint: oauth2.Endpoint{
			AuthURL:  endpoints.Links.AuthEndpoint.URL + "/oauth/auth",
			TokenURL: endpoints.Links.TokenEndpoint.URL + "/oauth/token",
		},
	}
	if config.Origin != "" {
		loginHint := LoginHint{config.Origin}
		origin, err := json.Marshal(loginHint)
		if err != nil {
			return config, errors.Wrap(err, "Error creating login_hint")
		}
		val := url.Values{}
		val.Set("login_hint", string(origin))
		authConfig.Endpoint.TokenURL = fmt.Sprintf("%s?%s", authConfig.Endpoint.TokenURL, val.Encode())
	}

	token, err := authConfig.PasswordCredentialsToken(ctx, config.Username, config.Password)
	if err != nil {
		return config, errors.Wrap(err, "Error getting token")
	}

	config.tokenSourceDeadline = &token.Expiry
	config.TokenSource = authConfig.TokenSource(ctx, token)
	config.HttpClient = oauth2.NewClient(ctx, config.TokenSource)

	return config, err
}

func getInfo(api string, httpClient *http.Client) (*Endpoints, error) {
	var endpoints Endpoints

	if api == "" {
		return nil, fmt.Errorf("CF ApiAddress cannot be empty")
	}

	resp, err := httpClient.Get(api + "/")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	err = decodeBody(resp, &endpoints)
	if err != nil {
		return nil, err
	}

	return &endpoints, err
}

func shallowDefaultTransport() *http.Transport {
	defaultTransport := http.DefaultTransport.(*http.Transport)
	return &http.Transport{
		Proxy:                 defaultTransport.Proxy,
		TLSHandshakeTimeout:   defaultTransport.TLSHandshakeTimeout,
		ExpectContinueTimeout: defaultTransport.ExpectContinueTimeout,
	}
}

// decodeBody is used to JSON decode a body
func decodeBody(resp *http.Response, out interface{}) error {
	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)
	return dec.Decode(out)
}

// encodeBody is used to encode a request body
func encodeBody(obj interface{}) (io.Reader, error) {
	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	if err := enc.Encode(obj); err != nil {
		return nil, err
	}
	return buf, nil
}

// toHTTP converts the request to an HTTP Request
func (r *Request) toHTTP() (*http.Request, error) {

	// Check if we should encode the body
	if r.body == nil && r.obj != nil {
		b, err := encodeBody(r.obj)
		if err != nil {
			return nil, err
		}
		r.body = b
	}

	// Create the HTTP Request
	return http.NewRequest(r.method, r.url, r.body)
}

func (c *Client) handleError(resp *http.Response) (*http.Response, error) {
	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return resp, CloudFoundryHTTPError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       body,
		}
	}
	defer resp.Body.Close()

	var cfErrors CloudFoundryErrors
	if err := json.Unmarshal(body, &cfErrors); err != nil {
		return resp, CloudFoundryHTTPError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       body,
		}
	}
	return nil, CloudFoundryToHttpError(cfErrors)
}
