package services

import (
	"context"
	"fmt"

	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/ports"
	"github.com/reglet-dev/reglet/plugins/tcp/core"
)

// TCPService provides TCP connection checks.
type TCPService struct {
	plugin.Service `name:"tcp" desc:"TCP connectivity checks"`

	Connect plugin.Op[ConnectInput, ConnectOutput] `desc:"Verify TCP connection can be established" method:"ConnectHandler"`
}

func init() {
	plugin.RegisterOp[ConnectInput, ConnectOutput]("Connect",
		plugin.Example[ConnectInput, ConnectOutput]{
			Name:        "simple_connect",
			Description: "Connect to google.com on port 80",
			Input:       ConnectInput{Host: "google.com", Port: 80},
			ExpectedOutput: &ConnectOutput{
				Host:      "google.com",
				Port:      80,
				Connected: true,
			},
		},
		plugin.Example[ConnectInput, ConnectOutput]{
			Name:        "tls_connect",
			Description: "Connect to google.com on port 443 with TLS",
			Input:       ConnectInput{Host: "google.com", Port: 443, TLS: true},
			ExpectedOutput: &ConnectOutput{
				Host:      "google.com",
				Port:      443,
				Connected: true,
			},
		},
	)

	plugin.MustRegisterService(core.Plugin, &TCPService{})
}

// ConnectHandler performs the TCP connection check and optional TLS validation.
func (s *TCPService) ConnectHandler(ctx context.Context, in *ConnectInput) (*ConnectOutput, error) {
	dialer := plugin.GetClient[ports.TCPDialer](ctx)

	target := fmt.Sprintf("%s:%d", in.Host, in.Port)
	timeout := in.TimeoutMs
	if timeout == 0 {
		timeout = 5000
	}

	// Use DialSecure which supports timeout and TLS (matches SDK interface)
	conn, err := dialer.DialSecure(ctx, target, timeout, in.TLS)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer conn.Close()

	output := &ConnectOutput{
		Host:      in.Host,
		Port:      in.Port,
		Connected: true,
	}

	if in.TLS || conn.IsTLS() {
		output.TLSVersion = conn.TLSVersion()
		output.TLSCipherSuite = conn.TLSCipherSuite()
	}

	return output, nil
}
