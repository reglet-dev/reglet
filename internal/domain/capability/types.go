package capability

import (
	"github.com/reglet-dev/reglet-abi/hostfunc"
)

// Requirement represents a request for capabilities by a plugin.
type Requirement struct {
	PluginName string
	Requested  GrantSet
}

// Grant represents the actual capabilities granted to a plugin after policy enforcement.
type Grant struct {
	PluginName string
	Granted    GrantSet
}

// Request represents a single capability request for prompting constraints.
type Request struct {
	Kind        string
	Description string
	IsBroad     bool
	// We keep the Rule as interface{} for now or use specific rule types if needed for formatting
	Rule interface{}
}

// GrantSet is the internal representation of plugin capabilities.
// This mirrors the ABI GrantSet but is decoupled to allow internal evolution.
type GrantSet struct {
	Network *NetworkCapability     `json:"network,omitempty" yaml:"network,omitempty"`
	FS      *FileSystemCapability  `json:"fs,omitempty" yaml:"fs,omitempty"`
	Env     *EnvironmentCapability `json:"env,omitempty" yaml:"env,omitempty"`
	Exec    *ExecCapability        `json:"exec,omitempty" yaml:"exec,omitempty"`
	KV      *KeyValueCapability    `json:"kv,omitempty" yaml:"kv,omitempty"`
}

// NetworkCapability defines permitted network access.
type NetworkCapability struct {
	Rules []NetworkRule `json:"rules" yaml:"rules"`
}

// NetworkRule defines a single network access rule.
type NetworkRule struct {
	Hosts []string `json:"hosts" yaml:"hosts"`
	Ports []string `json:"ports" yaml:"ports"`
}

// FileSystemCapability defines permitted filesystem access.
type FileSystemCapability struct {
	Rules []FileSystemRule `json:"rules" yaml:"rules"`
}

// FileSystemRule defines a single filesystem access rule.
type FileSystemRule struct {
	Read  []string `json:"read,omitempty" yaml:"read,omitempty"`
	Write []string `json:"write,omitempty" yaml:"write,omitempty"`
}

// EnvironmentCapability defines permitted environment variables.
type EnvironmentCapability struct {
	Variables []string `json:"variables" yaml:"variables"`
}

// ExecCapability defines permitted command execution.
type ExecCapability struct {
	Commands []string `json:"commands" yaml:"commands"`
}

// KeyValueCapability defines permitted key-value store access.
type KeyValueCapability struct {
	Rules []KeyValueRule `json:"rules" yaml:"rules"`
}

// KeyValueRule defines a single key-value access rule.
type KeyValueRule struct {
	Operation string   `json:"operation" yaml:"operation"`
	Keys      []string `json:"keys" yaml:"keys"`
}

// IsEmpty returns true if no capabilities are present.
func (g GrantSet) IsEmpty() bool {
	if g.Network != nil && len(g.Network.Rules) > 0 {
		return false
	}
	if g.FS != nil && len(g.FS.Rules) > 0 {
		return false
	}
	if g.Env != nil && len(g.Env.Variables) > 0 {
		return false
	}
	if g.Exec != nil && len(g.Exec.Commands) > 0 {
		return false
	}
	if g.KV != nil && len(g.KV.Rules) > 0 {
		return false
	}
	return true
}

// FromABI converts an ABI GrantSet to an internal GrantSet.
func FromABI(abiGS *hostfunc.GrantSet) GrantSet {
	if abiGS == nil {
		return GrantSet{}
	}

	gs := GrantSet{}

	if abiGS.Network != nil {
		rules := make([]NetworkRule, len(abiGS.Network.Rules))
		for i, r := range abiGS.Network.Rules {
			rules[i] = NetworkRule{
				Hosts: append([]string(nil), r.Hosts...),
				Ports: append([]string(nil), r.Ports...),
			}
		}
		gs.Network = &NetworkCapability{Rules: rules}
	}

	if abiGS.FS != nil {
		rules := make([]FileSystemRule, len(abiGS.FS.Rules))
		for i, r := range abiGS.FS.Rules {
			rules[i] = FileSystemRule{
				Read:  append([]string(nil), r.Read...),
				Write: append([]string(nil), r.Write...),
			}
		}
		gs.FS = &FileSystemCapability{Rules: rules}
	}

	if abiGS.Env != nil {
		gs.Env = &EnvironmentCapability{
			Variables: append([]string(nil), abiGS.Env.Variables...),
		}
	}

	if abiGS.Exec != nil {
		gs.Exec = &ExecCapability{
			Commands: append([]string(nil), abiGS.Exec.Commands...),
		}
	}

	if abiGS.KV != nil {
		rules := make([]KeyValueRule, len(abiGS.KV.Rules))
		for i, r := range abiGS.KV.Rules {
			rules[i] = KeyValueRule{
				Operation: r.Operation,
				Keys:      append([]string(nil), r.Keys...),
			}
		}
		gs.KV = &KeyValueCapability{Rules: rules}
	}

	return gs
}

// ToABI converts an internal GrantSet to an ABI GrantSet.
func ToABI(gs GrantSet) *hostfunc.GrantSet {
	abiGS := &hostfunc.GrantSet{}

	if gs.Network != nil {
		rules := make([]hostfunc.NetworkRule, len(gs.Network.Rules))
		for i, r := range gs.Network.Rules {
			rules[i] = hostfunc.NetworkRule{
				Hosts: append([]string(nil), r.Hosts...),
				Ports: append([]string(nil), r.Ports...),
			}
		}
		abiGS.Network = &hostfunc.NetworkCapability{Rules: rules}
	}

	if gs.FS != nil {
		rules := make([]hostfunc.FileSystemRule, len(gs.FS.Rules))
		for i, r := range gs.FS.Rules {
			rules[i] = hostfunc.FileSystemRule{
				Read:  append([]string(nil), r.Read...),
				Write: append([]string(nil), r.Write...),
			}
		}
		abiGS.FS = &hostfunc.FileSystemCapability{Rules: rules}
	}

	if gs.Env != nil {
		abiGS.Env = &hostfunc.EnvironmentCapability{
			Variables: append([]string(nil), gs.Env.Variables...),
		}
	}

	if gs.Exec != nil {
		abiGS.Exec = &hostfunc.ExecCapability{
			Commands: append([]string(nil), gs.Exec.Commands...),
		}
	}

	if gs.KV != nil {
		rules := make([]hostfunc.KeyValueRule, len(gs.KV.Rules))
		for i, r := range gs.KV.Rules {
			rules[i] = hostfunc.KeyValueRule{
				Operation: r.Operation,
				Keys:      append([]string(nil), r.Keys...),
			}
		}
		abiGS.KV = &hostfunc.KeyValueCapability{Rules: rules}
	}

	return abiGS
}
