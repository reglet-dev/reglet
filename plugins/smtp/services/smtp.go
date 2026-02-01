package services

import (
	"context"
	"fmt"
	"time"

	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet-sdk/go/domain/ports"
	"github.com/reglet-dev/reglet/plugins/smtp/core"
)

// SMTPService provides SMTP connection checks.
type SMTPService struct {
	plugin.Service `name:"smtp" desc:"SMTP connection and security checks"`

	Connect plugin.Op `desc:"Verify SMTP connection and capabilities" method:"ConnectHandler"`
}

func init() {
	plugin.MustRegisterService(core.Plugin, &SMTPService{})
}

// ConnectHandler performs the SMTP connection check.
func (s *SMTPService) ConnectHandler(ctx context.Context, req *plugin.Request) (*entities.Result, error) {
	cfg := req.Config.(*core.SMTPConfig)
	client := req.Client.(ports.SMTPClient)

	// Port: Convert int to string for Connect interface
	portStr := fmt.Sprintf("%d", cfg.Port)
	if cfg.Port == 0 {
		// Default port logic if 0? SDK client might handle, or we set default.
		// Config schema says default=25, so unlikely 0 if properly parsed.
		portStr = "25"
	}

	timeout := time.Duration(cfg.TimeoutMs) * time.Millisecond
	if cfg.TimeoutMs == 0 {
		timeout = 5 * time.Second
	}

	res, err := client.Connect(ctx, cfg.Host, portStr, timeout, cfg.UseTLS, cfg.StartTLS)
	if err != nil {
		return entities.ResultFailurePtr(fmt.Sprintf("SMTP connection failed: %v", err), map[string]any{
			"host": cfg.Host,
			"port": cfg.Port,
		}), nil
	}

	data := map[string]any{
		"host":             cfg.Host,
		"port":             cfg.Port,
		"connected":        res.Connected,
		"banner":           res.Banner,
		"tls_enabled":      res.TLSEnabled,
		"tls_version":      res.TLSVersion,
		"tls_cipher_suite": res.TLSCipherSuite,
		"response_time_ms": res.ResponseTime.Milliseconds(),
	}

	// Logic from legacy: checking if TLS was expected vs obtained?
	// The client.Connect handles the handshake. If UseTLS is true, it fails if TLS fails.
	// Same for StartTLS.
	// So if err == nil, we are good regarding parameters.

	msg := fmt.Sprintf("Connected to SMTP server %s:%d", cfg.Host, cfg.Port)
	if res.TLSEnabled {
		msg += fmt.Sprintf(" (TLS: %s)", res.TLSVersion)
	}

	return entities.ResultSuccessPtr(msg, data), nil
}
