package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	regletsdk "github.com/reglet-dev/reglet-sdk/go/application/config"
	"github.com/reglet-dev/reglet-sdk/go/domain/ports"
)

type testHTTPClient struct {
	client *http.Client
}

func (c *testHTTPClient) Do(ctx context.Context, req ports.HTTPRequest) (*ports.HTTPResponse, error) {
	stdReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, bytes.NewReader(req.Body))
	if err != nil {
		return nil, err
	}

	for k, v := range req.Headers {
		stdReq.Header.Set(k, v)
	}

	resp, err := c.client.Do(stdReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	headers := make(map[string][]string)
	for k, v := range resp.Header {
		headers[k] = v
	}

	return &ports.HTTPResponse{
		StatusCode: resp.StatusCode,
		Headers:    headers,
		Body:       body,
		Proto:      resp.Proto,
	}, nil
}

func (c *testHTTPClient) Get(ctx context.Context, url string) (*ports.HTTPResponse, error) {
	return c.Do(ctx, ports.HTTPRequest{Method: "GET", URL: url})
}

func (c *testHTTPClient) Post(ctx context.Context, url string, contentType string, body []byte) (*ports.HTTPResponse, error) {
	return c.Do(ctx, ports.HTTPRequest{
		Method: "POST",
		URL:    url,
		Headers: map[string]string{
			"Content-Type": contentType,
		},
		Body: body,
	})
}

func TestHTTPPlugin_Check_Success(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello world"))
	}))
	defer server.Close()

	plugin := &httpPlugin{client: &testHTTPClient{client: server.Client()}}
	config := regletsdk.Config{
		"url": server.URL,
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if !evidence.IsSuccess() {
		t.Errorf("Expected status true, got false. Error: %v", evidence.Error)
	}

	if statusCode, ok := evidence.Data["status_code"].(int); !ok || statusCode != 200 {
		t.Errorf("Expected status code 200, got %v", statusCode)
	}
}

func TestHTTPPlugin_Check_ExpectedStatus_Pass(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	plugin := &httpPlugin{client: &testHTTPClient{client: server.Client()}}
	config := regletsdk.Config{
		"url":             server.URL,
		"expected_status": 201,
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if evidence.IsFailure() {
		t.Errorf("Expected expectation to pass, got failure: %v", evidence.Status)
	}
}

func TestHTTPPlugin_Check_ExpectedStatus_Fail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	plugin := &httpPlugin{client: &testHTTPClient{client: server.Client()}}
	config := regletsdk.Config{
		"url":             server.URL,
		"expected_status": 201,
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	// Expect Failure status, not "expectation_failed" key
	if !evidence.IsFailure() {
		t.Errorf("Expected status Failure, got %v", evidence.Status)
	}

	if actual, ok := evidence.Data["actual_status"].(int); !ok || actual != 200 {
		t.Errorf("Expected actual_status 200, got %v", actual)
	}
}

func TestHTTPPlugin_Check_ExpectedBody_Pass(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("foo bar baz"))
	}))
	defer server.Close()

	plugin := &httpPlugin{client: &testHTTPClient{client: server.Client()}}
	config := regletsdk.Config{
		"url":                    server.URL,
		"expected_body_contains": "bar",
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if evidence.IsFailure() {
		t.Errorf("Expected expectation to pass, got failure")
	}
}

func TestHTTPPlugin_Check_ExpectedBody_Fail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("foo bar baz"))
	}))
	defer server.Close()

	plugin := &httpPlugin{client: &testHTTPClient{client: server.Client()}}
	config := regletsdk.Config{
		"url":                    server.URL,
		"expected_body_contains": "qux",
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if !evidence.IsFailure() {
		t.Errorf("Expected status Failure, got %v", evidence.Status)
	}
}

func TestHTTPPlugin_Check_Method(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	plugin := &httpPlugin{client: &testHTTPClient{client: server.Client()}}
	config := regletsdk.Config{
		"url":    server.URL,
		"method": "POST",
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if statusCode, ok := evidence.Data["status_code"].(int); !ok || statusCode != 200 {
		t.Errorf("Expected status code 200, got %v", statusCode)
	}
}
