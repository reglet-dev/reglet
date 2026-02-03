package core

import (
	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
)

var Plugin = plugin.DefinePlugin(plugin.PluginDef{
	Name:        "command",
	Version:     "1.0.0",
	Description: "Execute commands and validate output",
	Config:      &CommandConfig{},
	Capabilities: entities.GrantSet{
		Exec: &entities.ExecCapability{
			Commands: []string{"*"}, // Requested via manifest for specific commands
		},
	},
})
