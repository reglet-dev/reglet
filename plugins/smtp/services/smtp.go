package services

import (
	"context"
	"fmt"
	"time"

	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/ports"
	"github.com/reglet-dev/reglet/plugins/smtp/core"
)

// SMTPService provides SMTP connection checks.
type SMTPService struct {
	plugin.Service `name:"smtp" desc:"SMTP connection and security checks"`

	Connect plugin.Op[ConnectInput, ConnectOutput] `desc:"Verify SMTP connection and capabilities" method:"ConnectHandler"`
}

func init() {
	plugin.RegisterOp[ConnectInput, ConnectOutput]("Connect",
		plugin.Example[ConnectInput, ConnectOutput]{
			Name:        "simple_connect",
			Description: "Connect to smtp.gmail.com on port 587 with STARTTLS",
			Input:       ConnectInput{Host: "smtp.gmail.com", Port: 587, UseSTARTTLS: true},
			ExpectedOutput: &ConnectOutput{
				Host: "smtp.gmail.com",
				Port: 587,
			},
		},
	)

	plugin.MustRegisterService(core.Plugin, &SMTPService{})
}

// ConnectHandler performs the SMTP connection check.
func (s *SMTPService) ConnectHandler(ctx context.Context, in *ConnectInput) (*ConnectOutput, error) {
	client := plugin.GetClient[ports.SMTPClient](ctx)

	// Port: Convert int to string for Connect interface
	portStr := fmt.Sprintf("%d", in.Port)
	if in.Port == 0 {
		portStr = "25"
	}

	timeout := time.Duration(in.TimeoutMs) * time.Millisecond
	if in.TimeoutMs == 0 {
		timeout = 5 * time.Second
	}

	res, err := client.Connect(ctx, in.Host, portStr, timeout, in.UseTLS, in.UseSTARTTLS)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}

	output := &ConnectOutput{
		Host:         in.Host,
		Port:         in.Port,
		Banner:       res.Banner,
		Extensions:   res.Extensions,
		SupportsAuth: res.SupportsAuth,
	}

	if res.TLSEnabled {
		output.TLSVersion = res.TLSVersion
	}

	return output, nil
}
