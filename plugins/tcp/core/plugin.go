package core

import (
	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
)

var Plugin = plugin.DefinePlugin(plugin.PluginDef{
	Name:         "tcp",
	Version:      "1.0.0",
	Description:  "TCP connection testing and TLS validation",
	Config:       &TCPConfig{},
	Capabilities: entities.GrantSet{
		// Removed wildcard - capabilities should be extracted from profile config
	},
})
