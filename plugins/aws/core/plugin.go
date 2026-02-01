package core

import (
	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
)

// Plugin is the AWS plugin definition.
// Services register themselves against this on import.
var Plugin = plugin.DefinePlugin(plugin.PluginDef{
	Name:         "aws",
	Version:      "1.0.0",
	Description:  "AWS infrastructure inspection and compliance checks",
	Config:       &AWSConfig{},
	Capabilities: []entities.Capability{entities.CapabilityHTTP},
})
