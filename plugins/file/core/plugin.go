package core

import (
	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
)

var Plugin = plugin.DefinePlugin(plugin.PluginDef{
	Name:        "file",
	Version:     "1.0.0",
	Description: "File system checks and validation",
	Config:      &FileConfig{},
	Capabilities: []entities.Capability{
		entities.CapabilityFile,
	},
})
