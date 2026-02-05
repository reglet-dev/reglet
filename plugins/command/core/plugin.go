package core

import (
	"github.com/reglet-dev/reglet-sdk/application/plugin"
	"github.com/reglet-dev/reglet-sdk/domain/entities"
)

var Plugin = plugin.DefinePlugin(plugin.PluginDef{
	Name:        "command",
	Version:     "1.0.0",
	Description: "Execute commands and validate output",
	Config:      &CommandConfig{},
	Capabilities: entities.GrantSet{
		Exec: &entities.ExecCapability{
			Commands: []string{"/bin/sh", "/bin/echo", "/usr/bin/env", "echo", "sh", "*"}, // Requested via manifest for specific commands
		},
	},
})
