package services

import (
	"context"
	"fmt"

	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet-sdk/go/domain/ports"
	"github.com/reglet-dev/reglet/plugins/tcp/core"
)

// TCPService provides TCP connection checks.
type TCPService struct {
	plugin.Service `name:"tcp" desc:"TCP connectivity checks"`

	Connect plugin.Op `desc:"Verify TCP connection can be established" method:"ConnectHandler"`
	// PortOpen matches Connect logically but we can expose it if needed.
	// We'll stick to ConnectHandler for the main check.
}

func init() {
	plugin.MustRegisterService(core.Plugin, &TCPService{})
}

// ConnectHandler performs the TCP connection check and optional TLS validation.
func (s *TCPService) ConnectHandler(ctx context.Context, req *plugin.Request) (*entities.Result, error) {
	cfg := req.Config.(*core.TCPConfig)
	dialer := req.Client.(ports.TCPDialer)

	target := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	// Use DialSecure which supports timeout and TLS (matches SDK interface)
	conn, err := dialer.DialSecure(ctx, target, cfg.TimeoutMs, cfg.TLS)
	if err != nil {
		return entities.ResultFailurePtr(fmt.Sprintf("Connection failed: %v", err), map[string]any{
			"host": cfg.Host,
			"port": cfg.Port,
		}), nil
	}
	defer conn.Close()

	// Gather metadata
	data := map[string]any{
		"host":        cfg.Host,
		"port":        cfg.Port,
		"connected":   true,
		"remote_addr": conn.RemoteAddr(),
		"local_addr":  conn.LocalAddr(),
	}

	if cfg.TLS || conn.IsTLS() {
		data["tls_version"] = conn.TLSVersion()
		data["cipher_suite"] = conn.TLSCipherSuite()
		// conn.PeerCertificates logic removed as interface only exposes specific fields

		// TLS Version Check
		if cfg.ExpectedTLSVersion != "" {
			actual := conn.TLSVersion()
			if !isTLSVersionAtLeast(actual, cfg.ExpectedTLSVersion) {
				return entities.ResultFailurePtr(
					fmt.Sprintf("TLS version mismatch: expected >= %s, got %s", cfg.ExpectedTLSVersion, actual),
					data,
				), nil
			}
		}
	}

	return entities.ResultSuccessPtr(fmt.Sprintf("Connected to %s", target), data), nil
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
