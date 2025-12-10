//go:build wasip1

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	regletsdk "github.com/whiskeyjimbo/reglet/sdk"
)

func TestHTTPPlugin_Check_Success(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello world"))
	}))
	defer server.Close()

	plugin := &httpPlugin{client: server.Client()}
	config := regletsdk.Config{
		"url": server.URL,
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if !evidence.Status {
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

	plugin := &httpPlugin{client: server.Client()}
	config := regletsdk.Config{
		"url":             server.URL,
		"expected_status": 201,
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if evidence.Data["expectation_failed"] == true {
		t.Errorf("Expected expectation to pass")
	}
}

func TestHTTPPlugin_Check_ExpectedStatus_Fail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	plugin := &httpPlugin{client: server.Client()}
	config := regletsdk.Config{
		"url":             server.URL,
		"expected_status": 201,
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if evidence.Data["expectation_failed"] != true {
		t.Errorf("Expected expectation_failed to be true")
	}
}

func TestHTTPPlugin_Check_ExpectedBody_Pass(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("foo bar baz"))
	}))
	defer server.Close()

	plugin := &httpPlugin{client: server.Client()}
	config := regletsdk.Config{
		"url":                    server.URL,
		"expected_body_contains": "bar",
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if evidence.Data["expectation_failed"] == true {
		t.Errorf("Expected expectation to pass")
	}
}

func TestHTTPPlugin_Check_ExpectedBody_Fail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("foo bar baz"))
	}))
	defer server.Close()

	plugin := &httpPlugin{client: server.Client()}
	config := regletsdk.Config{
		"url":                    server.URL,
		"expected_body_contains": "qux",
	}

	evidence, err := plugin.Check(context.Background(), config)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	if evidence.Data["expectation_failed"] != true {
		t.Errorf("Expected expectation_failed to be true")
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

	plugin := &httpPlugin{client: server.Client()}
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
