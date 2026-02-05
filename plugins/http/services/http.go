package services

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/reglet-dev/reglet-sdk/application/plugin"
	"github.com/reglet-dev/reglet-sdk/domain/ports"
	"github.com/reglet-dev/reglet/plugins/http/core"
)

// HTTPService provides HTTP endpoint checks.
type HTTPService struct {
	plugin.Service `name:"http" desc:"HTTP endpoint checks"`

	Request plugin.Op[RequestInput, RequestOutput] `desc:"Perform HTTP request" method:"RequestHandler"`
}

func init() {
	plugin.RegisterOp[RequestInput, RequestOutput]("Request",
		plugin.Example[RequestInput, RequestOutput]{
			Name:        "get_example_com",
			Description: "GET http://example.com",
			Input:       RequestInput{Method: "GET", URL: "http://example.com"},
			ExpectedOutput: &RequestOutput{
				URL:        "http://example.com",
				Method:     "GET",
				StatusCode: 200,
				Status:     "200 OK",
			},
		},
		plugin.Example[RequestInput, RequestOutput]{
			Name:        "post_json",
			Description: "POST JSON data",
			Input: RequestInput{
				URL:     "https://httpbin.org/post",
				Method:  "POST",
				Headers: map[string]string{"Content-Type": "application/json"},
				Body:    `{"key": "value"}`,
			},
			ExpectedOutput: &RequestOutput{
				URL:        "https://httpbin.org/post",
				Method:     "POST",
				StatusCode: 200,
			},
		},
		plugin.Example[RequestInput, RequestOutput]{
			Name:  "not_found",
			Input: RequestInput{URL: "https://httpbin.org/status/404"},
			ExpectedOutput: &RequestOutput{
				StatusCode: 404,
			},
		},
	)

	plugin.MustRegisterService(core.Plugin, &HTTPService{})
}

// RequestHandler performs the HTTP request.
func (s *HTTPService) RequestHandler(ctx context.Context, in *RequestInput) (*RequestOutput, error) {
	client := plugin.GetClient[ports.HTTPClient](ctx)

	method := strings.ToUpper(in.Method)
	if method == "" {
		method = "GET"
	}

	timeout := time.Duration(in.TimeoutSeconds) * time.Second
	if in.TimeoutSeconds <= 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	httpReq := ports.HTTPRequest{
		Method:  method,
		URL:     in.URL,
		Headers: in.Headers,
		Body:    []byte(in.Body),
	}

	start := time.Now()
	resp, err := client.Do(ctx, httpReq)
	duration := time.Since(start).Milliseconds()
	if err != nil {
		// Return result with status=0 and error details instead of plugin error
		return &RequestOutput{
			URL:            in.URL,
			Method:         method,
			StatusCode:     0,
			Status:         fmt.Sprintf("ERROR: %v", err),
			ResponseTimeMs: duration,
		}, nil
	}

	// Reconstruct Status string since ports.HTTPResponse doesn't have it
	statusText := http.StatusText(resp.StatusCode)
	statusStr := fmt.Sprintf("%d %s", resp.StatusCode, statusText)

	return &RequestOutput{
		URL:            in.URL,
		Method:         method,
		StatusCode:     resp.StatusCode,
		Status:         statusStr,
		Protocol:       resp.Proto,
		Headers:        resp.Headers,
		Body:           string(resp.Body),
		BodySize:       len(resp.Body),
		ResponseTimeMs: time.Since(start).Milliseconds(),
		// TLS fields populated when HTTPResponse has TLS info
	}, nil
}
