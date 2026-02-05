package core

import (
	"github.com/reglet-dev/reglet-sdk/application/plugin"
	"github.com/reglet-dev/reglet-sdk/domain/entities"
)

// DNSConfig defines the plugin-level configuration.
// This is the union of all operation inputs.
type DNSConfig struct {
	Hostname   string `json:"hostname" jsonschema:"required,description=Target hostname to resolve"`
	RecordType string `json:"record_type,omitempty" jsonschema:"enum=A,enum=AAAA,enum=MX,enum=TXT,enum=CNAME,enum=NS,default=A,description=DNS record type to query"`
	Nameserver string `json:"nameserver,omitempty" jsonschema:"description=Custom nameserver to use"`
}

var Plugin = plugin.DefinePlugin(plugin.PluginDef{
	Name:        "dns",
	Version:     "1.0.0",
	Description: "DNS resolution and record lookup",
	Config:      &DNSConfig{},
	Capabilities: entities.GrantSet{
		Network: &entities.NetworkCapability{
			Rules: []entities.NetworkRule{
				{Hosts: []string{"*"}, Ports: []string{"53"}},
			},
		},
	},
})
