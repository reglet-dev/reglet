package core

import (
	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
)

var Plugin = plugin.DefinePlugin(plugin.PluginDef{
	Name:        "smtp",
	Version:     "1.0.0",
	Description: "SMTP connection testing and server validation",
	Config:      &SMTPConfig{},
	Capabilities: entities.GrantSet{
		Network: &entities.NetworkCapability{
			Rules: []entities.NetworkRule{
				{Hosts: []string{"*"}, Ports: []string{"25", "465", "587"}},
			},
		},
	},
})
