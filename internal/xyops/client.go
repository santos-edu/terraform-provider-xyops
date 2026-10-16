package xyops

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	endpoint string
	apiKey   string
	http     *http.Client
}

type APIError struct {
	Code        any
	Description string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("xyOps API error (%v): %s", e.Code, e.Description)
}
func (e *APIError) IsNotFound() bool { return e.Code == "notfound" }

// ValidateKey probes the key with the lightest authenticated read. A search
// action answers with the standard envelope, needs no specific privilege and,
// with a query that matches no records, returns an empty payload instead of a
// potentially large listing. API key actions are not usable here: they require
// a user session and CSRF, not just the key.
func (c *Client) ValidateKey() error {
	params := url.Values{}
	params.Set("query", "alert:_xyops_provider_probe")
	params.Set("limit", "1")
	return c.GetQuery("search_alerts", params, nil)
}

func NewClient(endpoint, apiKey string) (*Client, error) {
	u, err := url.Parse(strings.TrimRight(endpoint, "/"))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("endpoint must be an absolute http or https URL")
	}
	return &Client{endpoint: strings.TrimRight(endpoint, "/"), apiKey: apiKey, http: &http.Client{Timeout: 30 * time.Second}}, nil
}

func (c *Client) Call(action string, input any, output any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("encode xyOps request: %w", err)
	}
	return c.do(http.MethodPost, action+"/v1", bytes.NewReader(body), output)
}

func (c *Client) Get(action string, output any) error {
	return c.GetQuery(action, nil, output)
}

// GetQuery issues a GET with query string parameters against the action
// endpoint. GET requests need no CSRF token, so read-only actions take this
// path.
func (c *Client) GetQuery(action string, params url.Values, output any) error {
	path := action + "/v1"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	return c.do(http.MethodGet, path, nil, output)
}

func (c *Client) do(method, path string, body io.Reader, output any) error {
	req, err := http.NewRequest(method, c.endpoint+"/api/app/"+path, body)
	if err != nil {
		return fmt.Errorf("create xyOps request: %w", err)
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-API-Key", c.apiKey)
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("call xyOps %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()
	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("read xyOps response: %w", err)
	}
	var envelope struct {
		Code        any    `json:"code"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(responseBody, &envelope); err != nil {
		return fmt.Errorf("decode xyOps response: %w", err)
	}
	if envelope.Code != float64(0) && envelope.Code != 0 {
		return &APIError{Code: envelope.Code, Description: envelope.Description}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("xyOps returned HTTP %s: %s", res.Status, strings.TrimSpace(string(responseBody)))
	}
	if output != nil {
		if err := json.Unmarshal(responseBody, output); err != nil {
			return fmt.Errorf("decode xyOps response payload: %w", err)
		}
	}
	return nil
}
