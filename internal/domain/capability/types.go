package capability

// Requirement represents a request for capabilities by a plugin.
type Requirement struct {
	Requested  GrantSet
	PluginName string
}

// Grant represents the actual capabilities granted to a plugin after policy enforcement.
type Grant struct {
	Granted    GrantSet
	PluginName string
}

// Request represents a single capability request for prompting constraints.
type Request struct {
	Rule        interface{}
	Kind        string
	Description string
	IsBroad     bool
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
