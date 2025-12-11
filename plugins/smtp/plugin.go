//go:build wasip1

package main

import (
	"context"
	"fmt"

	regletsdk "github.com/whiskeyjimbo/reglet/sdk"
	regletnet "github.com/whiskeyjimbo/reglet/sdk/net"
)

// smtpPlugin implements the sdk.Plugin interface.
type smtpPlugin struct {
	// DialSMTP allows dependency injection for testing
	DialSMTP func(ctx context.Context, host, port string, timeoutMs int, useTLS bool, useStartTLS bool) (*regletnet.SMTPConnectResult, error)
}

// Describe returns plugin metadata.
func (p *smtpPlugin) Describe(ctx context.Context) (regletsdk.Metadata, error) {
	return regletsdk.Metadata{
		Name:        "smtp",
		Version:     "1.0.0",
		Description: "SMTP connection testing and server validation",
		Capabilities: []regletsdk.Capability{
			{
				Kind:    "network",
				Pattern: "outbound:25,465,587",
			},
		},
	}, nil
}

type SMTPConfig struct {
	Host      string `json:"host" validate:"required" description:"SMTP server host (hostname or IP)"`
	Port      string `json:"port" validate:"required" description:"SMTP server port (25, 465, 587, 2525)"`
	TimeoutMs int    `json:"timeout_ms" default:"5000" description:"Connection timeout in milliseconds"`
	TLS       bool   `json:"tls,omitempty" description:"Use direct TLS/SSL connection (SMTPS on port 465)"`
	StartTLS  bool   `json:"starttls,omitempty" description:"Use STARTTLS to upgrade connection to TLS"`
}

// Schema returns the JSON schema for the plugin's configuration.
func (p *smtpPlugin) Schema(ctx context.Context) ([]byte, error) {
	return regletsdk.GenerateSchema(SMTPConfig{})
}

// Check executes the SMTP observation.
func (p *smtpPlugin) Check(ctx context.Context, config regletsdk.Config) (regletsdk.Evidence, error) {
	// Set defaults
	if _, ok := config["timeout_ms"]; !ok {
		config["timeout_ms"] = 5000
	}

	var cfg SMTPConfig
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

	address := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)

	if p.DialSMTP == nil {
		return regletsdk.Failure("internal", "DialSMTP not initialized"), nil
	}

	result, err := p.DialSMTP(ctx, cfg.Host, cfg.Port, cfg.TimeoutMs, cfg.TLS, cfg.StartTLS)
	if err != nil {
		return regletsdk.Evidence{
			Status: false,
			Error: regletsdk.ToErrorDetail(
				&regletsdk.NetworkError{
					Operation: "smtp_connect",
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
		"banner":           result.Banner,
	}

	if result.TLS {
		data["tls"] = true
		data["tls_version"] = result.TLSVersion
		data["tls_cipher_suite"] = result.TLSCipherSuite
		data["tls_server_name"] = result.TLSServerName
	}

	return regletsdk.Success(data), nil
}

func main() {
	plugin := &smtpPlugin{
		DialSMTP: regletnet.DialSMTP,
	}
	regletsdk.Register(plugin)
}
