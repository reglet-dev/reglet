package main

import (
	"context"
	"fmt"

	"github.com/reglet-dev/reglet-sdk/go/application/config"
	"github.com/reglet-dev/reglet-sdk/go/application/schema"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet-sdk/go/domain/ports"
	sdknet "github.com/reglet-dev/reglet-sdk/go/net"
)

// tcpPlugin implements the sdk.Plugin interface.
type tcpPlugin struct {
	dialer ports.TCPDialer
}

// Describe returns plugin metadata.
func (p *tcpPlugin) Describe(ctx context.Context) (entities.Metadata, error) {
	return entities.Metadata{
		Name:        "tcp",
		Version:     "1.0.0",
		Description: "TCP connection testing and TLS validation",
		Capabilities: []entities.Capability{
			{
				Category: "network",
				Resource: "outbound:*",
			},
		},
	}, nil
}

type TCPConfig struct {
	Host               string      `json:"host" validate:"required" description:"Target host (hostname or IP)"`
	Port               interface{} `json:"port" validate:"required" description:"Target port (number or string)"`
	TimeoutMs          int         `json:"timeout_ms" default:"5000" description:"Connection timeout in milliseconds"`
	TLS                bool        `json:"tls,omitempty" description:"Use TLS/SSL connection"`
	ExpectedTLSVersion string      `json:"expected_tls_version,omitempty" description:"Expected minimum TLS version (e.g., 'TLS 1.2')"`
}

// Schema returns the JSON schema for the plugin's configuration.
func (p *tcpPlugin) Schema(ctx context.Context) ([]byte, error) {
	return schema.GenerateSchema(TCPConfig{})
}

// Check executes the TCP observation.
func (p *tcpPlugin) Check(ctx context.Context, cfgRaw config.Config) (entities.Result, error) {
	var cfg TCPConfig
	if err := config.Validate(cfgRaw, &cfg); err != nil {
		return entities.ResultError(entities.NewErrorDetail("config", err.Error())), nil
	}
	// Ensure port is passed as an integer to the SDK
	// This helps when YAML/JSON config passes port as a string ("443")
	var portInt int
	switch v := cfg.Port.(type) {
	case float64:
		portInt = int(v)
	case int:
		portInt = v
	case string:
		var err error
		if _, err = fmt.Sscanf(v, "%d", &portInt); err != nil {
			return entities.ResultError(entities.NewErrorDetail("config", "invalid port format: must be integer")), nil
		}
	default:
		return entities.ResultError(entities.NewErrorDetail("config", fmt.Sprintf("invalid port type: %T", cfg.Port))), nil
	}

	// Update the raw config with the integer port
	// SDK RunTCPCheck expects 'port' key to be parseable as int via config.GetInt
	// which handles string-to-int conversion, BUT config validation above would fail
	// if struct field was strictly int. Now struct is interface{}, validation passes.
	cfgRaw["port"] = portInt

	// RunTCPCheck expects config map, which cfgRaw is.
	result, err := sdknet.RunTCPCheck(ctx, cfgRaw, sdknet.WithTCPDialer(p.dialer))
	if err != nil {
		return result, err
	}

	// Post-check: TLS version expectation (plugin-specific logic)
	expectedTLS := cfg.ExpectedTLSVersion
	if expectedTLS != "" && result.IsSuccess() {
		actualTLS, _ := result.Data["tls_version"].(string)
		if !isTLSVersionAtLeast(actualTLS, expectedTLS) {
			result.Data["expectation_failed"] = true
			result.Data["expectation_error"] = fmt.Sprintf("expected TLS version >= %s, got %s", expectedTLS, actualTLS)
		}
	}
	return result, nil
}

// isTLSVersionAtLeast checks if actual TLS version meets the minimum requirement
func isTLSVersionAtLeast(actual, minimum string) bool {
	versions := map[string]int{
		"TLS 1.0": 10,
		"TLS 1.1": 11,
		"TLS 1.2": 12,
		"TLS 1.3": 13,
	}

	actualVal, okActual := versions[actual]
	minimumVal, okMinimum := versions[minimum]

	if !okActual || !okMinimum {
		return false
	}

	return actualVal >= minimumVal
}
