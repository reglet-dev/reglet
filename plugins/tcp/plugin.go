//go:build wasip1

package main

import (
	"context"
	"fmt"
	"time"

	regletsdk "github.com/whiskeyjimbo/reglet/sdk"
	regletnet "github.com/whiskeyjimbo/reglet/sdk/net"
)

// tcpPlugin implements the sdk.Plugin interface.
type tcpPlugin struct {
	// DialTCP allows dependency injection for testing
	DialTCP func(ctx context.Context, host, port string, timeoutMs int, useTLS bool) (*regletnet.TCPConnectResult, error)
}

// Describe returns plugin metadata.
func (p *tcpPlugin) Describe(ctx context.Context) (regletsdk.Metadata, error) {
	return regletsdk.Metadata{
		Name:        "tcp",
		Version:     "1.0.0",
		Description: "TCP connection testing and TLS validation",
		Capabilities: []regletsdk.Capability{
			{
				Kind:    "network",
				Pattern: "outbound:*",
			},
		},
	}, nil
}

type TCPConfig struct {
	Host               string `json:"host" validate:"required" description:"Target host (hostname or IP)"`
	Port               string `json:"port" validate:"required" description:"Target port"`
	TimeoutMs          int    `json:"timeout_ms" default:"5000" description:"Connection timeout in milliseconds"`
	TLS                bool   `json:"tls,omitempty" description:"Use TLS/SSL connection"`
	ExpectedTLSVersion string `json:"expected_tls_version,omitempty" description:"Expected minimum TLS version (e.g., 'TLS 1.2')"`
}

// Schema returns the JSON schema for the plugin's configuration.
func (p *tcpPlugin) Schema(ctx context.Context) ([]byte, error) {
	return regletsdk.GenerateSchema(TCPConfig{})
}

// Check executes the TCP observation.
func (p *tcpPlugin) Check(ctx context.Context, config regletsdk.Config) (regletsdk.Evidence, error) {
	// Set defaults
	if _, ok := config["timeout_ms"]; !ok {
		config["timeout_ms"] = 5000
	}

	var cfg TCPConfig
	if err := regletsdk.ValidateConfig(config, &cfg); err != nil {
		return regletsdk.Evidence{
			Status: false,
			Error: regletsdk.ToErrorDetail(
				&regletsdk.ConfigError{
					Err: err,
				},
			),
		}, nil
	}

	address := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port) // Add this line

	if p.DialTCP == nil {
		return regletsdk.Failure("internal", "DialTCP not initialized"), nil
	}

	result, err := p.DialTCP(ctx, cfg.Host, cfg.Port, cfg.TimeoutMs, cfg.TLS)
	if err != nil {
		return regletsdk.Evidence{
			Status: false,
			Error: regletsdk.ToErrorDetail(
				&regletsdk.NetworkError{
					Operation: "tcp_connect",
					Target:    address,
					Err:       err,
				},
			),
		}, nil
	}

	// Prepare evidence data from result
	data := map[string]interface{}{
		"connected":        result.Connected,
		"address":          result.Address,
		"response_time_ms": result.ResponseTimeMs,
		"remote_addr":      result.RemoteAddr,
		"local_addr":       result.LocalAddr,
	}

	if result.TLS {
		data["tls"] = true
		data["tls_version"] = result.TLSVersion
		data["tls_cipher_suite"] = result.TLSCipherSuite
		data["tls_server_name"] = result.TLSServerName
		if result.TLSCertSubject != "" {
			data["tls_cert_subject"] = result.TLSCertSubject
			data["tls_cert_issuer"] = result.TLSCertIssuer
		}
		if result.TLSCertNotAfter != nil {
			data["tls_cert_not_after"] = result.TLSCertNotAfter.Format(time.RFC3339)
			// Calculate days remaining
			days := int(time.Until(*result.TLSCertNotAfter).Hours() / 24)
			data["tls_cert_days_remaining"] = days
		}
	}

	// Check TLS version expectation
	if cfg.ExpectedTLSVersion != "" {
		if !isTLSVersionAtLeast(result.TLSVersion, cfg.ExpectedTLSVersion) {
			data["expectation_failed"] = true
			data["expectation_error"] = fmt.Sprintf("expected TLS version >= %s, got %s", cfg.ExpectedTLSVersion, result.TLSVersion)
			return regletsdk.Success(data), nil
		}
	}

	return regletsdk.Success(data), nil
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
