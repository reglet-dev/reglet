package core

import (
	"github.com/reglet-dev/reglet-sdk/application/plugin"
	"github.com/reglet-dev/reglet-sdk/domain/entities"
)

var Plugin = plugin.DefinePlugin(plugin.PluginDef{
	Name:        "http",
	Version:     "1.0.0",
	Description: "HTTP/HTTPS request checking and validation",
	Config:      &HTTPConfig{},
	Capabilities: entities.GrantSet{
		Network: &entities.NetworkCapability{
			Rules: []entities.NetworkRule{
				{Hosts: []string{"*"}, Ports: []string{"80", "443"}},
			},
		},
	},
})
