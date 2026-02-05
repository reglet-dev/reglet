package core

import (
	"github.com/reglet-dev/reglet-sdk/application/plugin"
	"github.com/reglet-dev/reglet-sdk/domain/entities"
)

var Plugin = plugin.DefinePlugin(plugin.PluginDef{
	Name:        "file",
	Version:     "1.0.0",
	Description: "File system checks and validation",
	Config:      &FileConfig{},
	Capabilities: entities.GrantSet{
		FS: &entities.FileSystemCapability{
			Rules: []entities.FileSystemRule{
				{Read: []string{"*"}}, // Requested via manifest for specific paths
			},
		},
	},
})
