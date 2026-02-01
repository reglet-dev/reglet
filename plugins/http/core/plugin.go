package core

import (
	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
)

var Plugin = plugin.DefinePlugin(plugin.PluginDef{
	Name:        "http",
	Version:     "1.0.0",
	Description: "HTTP/HTTPS request checking and validation",
	Config:      &HTTPConfig{},
	Capabilities: []entities.Capability{
		entities.CapabilityHTTP,
	},
})
