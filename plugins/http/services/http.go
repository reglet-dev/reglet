package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet-sdk/go/domain/ports"
	"github.com/reglet-dev/reglet/plugins/http/core"
)

// HTTPService provides HTTP endpoint checks.
// Auto-registers on package import via init().
type HTTPService struct {
	plugin.Service `name:"http" desc:"HTTP endpoint checks"`

	Get         plugin.Op `desc:"Perform HTTP GET request" method:"GetHandler"`
	Post        plugin.Op `desc:"Perform HTTP POST request" method:"PostHandler"`
	Head        plugin.Op `desc:"Perform HTTP HEAD request" method:"HeadHandler"`
	CheckStatus plugin.Op `desc:"Check expected status code" method:"CheckStatusHandler"`
	CheckSSL    plugin.Op `desc:"Verify SSL certificate" method:"CheckSSLHandler"`
}

func init() {
	plugin.MustRegisterService(core.Plugin, &HTTPService{})
}

// Handler implementations

func (s *HTTPService) GetHandler(ctx context.Context, req *plugin.Request) (*entities.Result, error) {
	return s.runCheck(ctx, req, "GET")
}

func (s *HTTPService) PostHandler(ctx context.Context, req *plugin.Request) (*entities.Result, error) {
	return s.runCheck(ctx, req, "POST")
}

func (s *HTTPService) HeadHandler(ctx context.Context, req *plugin.Request) (*entities.Result, error) {
	return s.runCheck(ctx, req, "HEAD")
}

func (s *HTTPService) CheckStatusHandler(ctx context.Context, req *plugin.Request) (*entities.Result, error) {
	// CheckStatus uses method from config or defaults to GET
	cfg := req.Config.(*core.HTTPConfig)
	method := cfg.Method
	if method == "" {
		method = "GET"
	}
	return s.runCheck(ctx, req, method)
}

func (s *HTTPService) CheckSSLHandler(ctx context.Context, req *plugin.Request) (*entities.Result, error) {
	// SSL check typically just needs a connection. HEAD is efficient.
	return s.runCheck(ctx, req, "HEAD")
}

// runCheck performs the common HTTP check logic
func (s *HTTPService) runCheck(ctx context.Context, req *plugin.Request, method string) (*entities.Result, error) {
	cfg := req.Config.(*core.HTTPConfig)
	client := req.Client.(ports.HTTPClient)

	// Prepare request
	httpReq := ports.HTTPRequest{
		Method:  method,
		URL:     cfg.URL,
		Headers: cfg.Headers,
		Body:    []byte(cfg.Body),
	}

	// Execute
	resp, err := client.Do(ctx, httpReq)
	if err != nil {
		return entities.ResultErrorPtr("network", fmt.Sprintf("Request failed: %v", err)), nil
	}

	// Validation
	expectedStatus := cfg.ExpectedStatus
	if expectedStatus == 0 {
		expectedStatus = 200
	}

	data := map[string]any{
		"status_code": resp.StatusCode,
		"url":         cfg.URL,
		"method":      method,
	}

	if resp.StatusCode != expectedStatus {
		return entities.ResultFailurePtr(
			fmt.Sprintf("Unexpected status code: got %d, want %d", resp.StatusCode, expectedStatus),
			data,
		), nil
	}

	// Body check
	if cfg.ExpectedBodyContains != "" {
		bodyStr := string(resp.Body)
		if !strings.Contains(bodyStr, cfg.ExpectedBodyContains) {
			return entities.ResultFailurePtr(
				fmt.Sprintf("Response body missing expected content: %q", cfg.ExpectedBodyContains),
				data,
			), nil
		}
	}

	// Success
	msg := fmt.Sprintf("%s %s returned %d OK", method, cfg.URL, resp.StatusCode)
	return entities.ResultSuccessPtr(msg, data), nil
}
