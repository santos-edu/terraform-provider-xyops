package xyops

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientGet(t *testing.T) {
	tests := []struct {
		name            string
		status          int
		body            string
		wantID          string
		wantAPIError    bool
		wantCode        any
		wantDescription string
		wantNotFound    bool
		wantError       string
	}{
		{
			name:   "numeric zero code decodes payload",
			status: http.StatusOK,
			body:   `{"code":0,"tag":{"id":"production"}}`,
			wantID: "production",
		},
		{
			name:            "string notfound code returns APIError",
			status:          http.StatusOK,
			body:            `{"code":"notfound","description":"Tag not found"}`,
			wantAPIError:    true,
			wantCode:        "notfound",
			wantDescription: "Tag not found",
			wantNotFound:    true,
		},
		{
			name:            "string api code returns APIError",
			status:          http.StatusOK,
			body:            `{"code":"api","description":"Malformed parameter: id"}`,
			wantAPIError:    true,
			wantCode:        "api",
			wantDescription: "Malformed parameter: id",
		},
		{
			name:            "numeric nonzero code returns APIError",
			status:          http.StatusOK,
			body:            `{"code":7,"description":"Unexpected code"}`,
			wantAPIError:    true,
			wantCode:        float64(7),
			wantDescription: "Unexpected code",
		},
		{
			name:      "non-success HTTP status is returned",
			status:    http.StatusServiceUnavailable,
			body:      `{"code":0,"description":"temporarily unavailable"}`,
			wantError: `xyOps returned HTTP 503 Service Unavailable: {"code":0,"description":"temporarily unavailable"}`,
		},
		{
			name:      "invalid response JSON is returned",
			status:    http.StatusOK,
			body:      `not-json`,
			wantError: "decode xyOps response:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("request method = %q, want %q", r.Method, http.MethodGet)
				}
				if r.URL.Path != "/api/app/get_tag/v1" {
					t.Errorf("request path = %q, want %q", r.URL.Path, "/api/app/get_tag/v1")
				}
				if r.Header.Get("X-API-Key") != "test-key" {
					t.Errorf("X-API-Key = %q, want %q", r.Header.Get("X-API-Key"), "test-key")
				}
				w.WriteHeader(tt.status)
				if _, err := io.WriteString(w, tt.body); err != nil {
					t.Errorf("write response: %s", err)
				}
			}))
			t.Cleanup(server.Close)

			client, err := NewClient(server.URL+"/", "test-key")
			if err != nil {
				t.Fatalf("NewClient() error = %s", err)
			}

			var response struct {
				Tag struct {
					ID string `json:"id"`
				} `json:"tag"`
			}
			err = client.Get("get_tag", &response)

			if tt.wantAPIError {
				var apiErr *APIError
				if !errors.As(err, &apiErr) {
					t.Fatalf("Get() error = %v, want *APIError", err)
				}
				if apiErr.Code != tt.wantCode {
					t.Errorf("APIError.Code = %#v, want %#v", apiErr.Code, tt.wantCode)
				}
				if apiErr.Description != tt.wantDescription {
					t.Errorf("APIError.Description = %q, want %q", apiErr.Description, tt.wantDescription)
				}
				if got := apiErr.IsNotFound(); got != tt.wantNotFound {
					t.Errorf("APIError.IsNotFound() = %t, want %t", got, tt.wantNotFound)
				}
				if got := apiErr.Error(); got != fmt.Sprintf("xyOps API error (%v): %s", tt.wantCode, tt.wantDescription) {
					t.Errorf("APIError.Error() = %q", got)
				}
				return
			}

			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("Get() error = %v, want it to contain %q", err, tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("Get() error = %s", err)
			}
			if response.Tag.ID != tt.wantID {
				t.Errorf("response.Tag.ID = %q, want %q", response.Tag.ID, tt.wantID)
			}
		})
	}
}

func TestClientValidateKey(t *testing.T) {
	t.Run("valid key sends probe with header", func(t *testing.T) {
		var gotPath, gotMethod, gotKey string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod, gotKey = r.URL.Path, r.Method, r.Header.Get("X-API-Key")
			_, _ = io.WriteString(w, `{"code":0}`)
		}))
		t.Cleanup(server.Close)

		client, err := NewClient(server.URL, "test-key")
		if err != nil {
			t.Fatalf("NewClient() error = %s", err)
		}
		if err := client.ValidateKey(); err != nil {
			t.Fatalf("ValidateKey() error = %s", err)
		}
		if gotPath != "/api/app/search_alerts/v1" || gotMethod != http.MethodGet || gotKey != "test-key" {
			t.Errorf("probe = %s %s key=%q, want GET search_alerts with X-API-Key", gotMethod, gotPath, gotKey)
		}
	})

	t.Run("invalid key surfaces APIError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"code":"unauthorized","description":"Invalid API key"}`)
		}))
		t.Cleanup(server.Close)

		client, err := NewClient(server.URL, "wrong-key")
		if err != nil {
			t.Fatalf("NewClient() error = %s", err)
		}
		err = client.ValidateKey()
		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("ValidateKey() error = %v, want *APIError", err)
		}
		if apiErr.Description != "Invalid API key" {
			t.Errorf("APIError.Description = %q, want %q", apiErr.Description, "Invalid API key")
		}
	})
}
