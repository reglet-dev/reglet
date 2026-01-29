package main

import (
	"context"

	"github.com/reglet-dev/reglet-sdk/go/application/config"
	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/application/schema"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet-sdk/go/domain/ports"
	sdknet "github.com/reglet-dev/reglet-sdk/go/net"
)

// smtpPlugin implements the sdk.Plugin interface.
type smtpPlugin struct {
	client ports.SMTPClient
}

// Describe returns plugin metadata.
func (p *smtpPlugin) Describe(ctx context.Context) (entities.Metadata, error) {
	return entities.Metadata{
		Name:        "smtp",
		Version:     "1.0.0",
		Description: "SMTP connection testing and server validation",
		Capabilities: []entities.Capability{
			{
				Category: "network",
				Resource: "outbound:25,465,587",
			},
		},
	}, nil
}

type SMTPConfig struct {
	Host      string `json:"host" validate:"required" description:"SMTP server host (hostname or IP)"`
	Port      int    `json:"port" validate:"required" description:"SMTP server port (25, 465, 587, 2525)"`
	TimeoutMs int    `json:"timeout_ms" default:"5000" description:"Connection timeout in milliseconds"`
	TLS       bool   `json:"use_tls,omitempty" description:"Use direct TLS/SSL connection (SMTPS on port 465)"`
	StartTLS  bool   `json:"use_starttls,omitempty" description:"Use STARTTLS to upgrade connection to TLS"`
}

// Schema returns the JSON schema for the plugin's configuration.
func (p *smtpPlugin) Schema(ctx context.Context) ([]byte, error) {
	return schema.GenerateSchema(SMTPConfig{})
}

// Check executes the SMTP observation.
func (p *smtpPlugin) Check(ctx context.Context, cfgRaw config.Config) (entities.Result, error) {
	var cfg SMTPConfig
	if err := config.Validate(cfgRaw, &cfg); err != nil {
		return entities.ResultError(entities.NewErrorDetail("config", err.Error())), nil
	}
	return sdknet.RunSMTPCheck(ctx, cfgRaw, sdknet.WithSMTPClient(p.client))
}

func init() {
	plugin.Register(&smtpPlugin{})
}

func main() {}
