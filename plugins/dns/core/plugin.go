package core

import (
	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
)

var Plugin = plugin.DefinePlugin(plugin.PluginDef{
	Name:        "dns",
	Version:     "1.0.0",
	Description: "DNS resolution and record validation",
	Config:      &DNSConfig{},
	Capabilities: []entities.Capability{
		entities.CapabilityDNS,
	},
})
