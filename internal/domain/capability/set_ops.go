package capability

// Merge unions two grant sets.
func (g *GrantSet) Merge(other *GrantSet) {
	if other == nil {
		return
	}
	g.mergeNetwork(other.Network)
	g.mergeFS(other.FS)
	g.mergeEnv(other.Env)
	g.mergeExec(other.Exec)
	g.mergeKV(other.KV)
}

func (g *GrantSet) mergeNetwork(other *NetworkCapability) {
	if other == nil || len(other.Rules) == 0 {
		return
	}
	if g.Network == nil {
		g.Network = &NetworkCapability{}
	}
	// Only append rules that don't already exist
	for _, rule := range other.Rules {
		if !g.containsNetworkRule(rule) {
			g.Network.Rules = append(g.Network.Rules, rule)
		}
	}
}

func (g *GrantSet) mergeFS(other *FileSystemCapability) {
	if other == nil || len(other.Rules) == 0 {
		return
	}
	if g.FS == nil {
		g.FS = &FileSystemCapability{}
	}
	// Only append rules that don't already exist
	for _, rule := range other.Rules {
		if !g.containsFSRule(rule) {
			g.FS.Rules = append(g.FS.Rules, rule)
		}
	}
}

func (g *GrantSet) mergeEnv(other *EnvironmentCapability) {
	if other == nil || len(other.Variables) == 0 {
		return
	}
	if g.Env == nil {
		g.Env = &EnvironmentCapability{}
	}
	// Only append variables that don't already exist
	for _, v := range other.Variables {
		if !g.containsEnvVar(v) {
			g.Env.Variables = append(g.Env.Variables, v)
		}
	}
}

func (g *GrantSet) mergeExec(other *ExecCapability) {
	if other == nil || len(other.Commands) == 0 {
		return
	}
	if g.Exec == nil {
		g.Exec = &ExecCapability{}
	}
	// Only append commands that don't already exist
	for _, cmd := range other.Commands {
		if !g.containsExecCmd(cmd) {
			g.Exec.Commands = append(g.Exec.Commands, cmd)
		}
	}
}

func (g *GrantSet) mergeKV(other *KeyValueCapability) {
	if other == nil || len(other.Rules) == 0 {
		return
	}
	if g.KV == nil {
		g.KV = &KeyValueCapability{}
	}
	// Only append rules that don't already exist
	for _, rule := range other.Rules {
		if !g.containsKVRule(rule) {
			g.KV.Rules = append(g.KV.Rules, rule)
		}
	}
}

// Clone returns a deep copy of the GrantSet.
func (g *GrantSet) Clone() GrantSet {
	clone := GrantSet{}
	if g.Network != nil {
		clone.Network = &NetworkCapability{
			Rules: make([]NetworkRule, len(g.Network.Rules)),
		}
		for i, rule := range g.Network.Rules {
			clone.Network.Rules[i] = NetworkRule{
				Hosts: append([]string(nil), rule.Hosts...),
				Ports: append([]string(nil), rule.Ports...),
			}
		}
	}
	if g.FS != nil {
		clone.FS = &FileSystemCapability{
			Rules: make([]FileSystemRule, len(g.FS.Rules)),
		}
		for i, rule := range g.FS.Rules {
			clone.FS.Rules[i] = FileSystemRule{
				Read:  append([]string(nil), rule.Read...),
				Write: append([]string(nil), rule.Write...),
			}
		}
	}
	if g.Env != nil {
		clone.Env = &EnvironmentCapability{
			Variables: append([]string(nil), g.Env.Variables...),
		}
	}
	if g.Exec != nil {
		clone.Exec = &ExecCapability{
			Commands: append([]string(nil), g.Exec.Commands...),
		}
	}
	if g.KV != nil {
		clone.KV = &KeyValueCapability{
			Rules: make([]KeyValueRule, len(g.KV.Rules)),
		}
		for i, rule := range g.KV.Rules {
			clone.KV.Rules[i] = KeyValueRule{
				Operation: rule.Operation,
				Keys:      append([]string(nil), rule.Keys...),
			}
		}
	}
	return clone
}

// Deduplicate removes duplicate entries within this GrantSet's arrays.
func (g *GrantSet) Deduplicate() {
	// Deduplicate network rules
	if g.Network != nil && len(g.Network.Rules) > 0 {
		seen := make(map[string]bool)
		unique := make([]NetworkRule, 0, len(g.Network.Rules))
		for _, rule := range g.Network.Rules {
			key := ""
			for _, h := range rule.Hosts {
				key += h + ","
			}
			key += ":"
			for _, p := range rule.Ports {
				key += p + ","
			}
			if !seen[key] {
				seen[key] = true
				unique = append(unique, rule)
			}
		}
		g.Network.Rules = unique
	}

	// Deduplicate filesystem rules
	if g.FS != nil && len(g.FS.Rules) > 0 {
		seen := make(map[string]bool)
		unique := make([]FileSystemRule, 0, len(g.FS.Rules))
		for _, rule := range g.FS.Rules {
			key := "r:"
			for _, p := range rule.Read {
				key += p + ","
			}
			key += "|w:"
			for _, p := range rule.Write {
				key += p + ","
			}
			if !seen[key] {
				seen[key] = true
				unique = append(unique, rule)
			}
		}
		g.FS.Rules = unique
	}

	// Deduplicate environment variables
	if g.Env != nil && len(g.Env.Variables) > 0 {
		seen := make(map[string]bool)
		unique := make([]string, 0, len(g.Env.Variables))
		for _, v := range g.Env.Variables {
			if !seen[v] {
				seen[v] = true
				unique = append(unique, v)
			}
		}
		g.Env.Variables = unique
	}

	// Deduplicate exec commands
	if g.Exec != nil && len(g.Exec.Commands) > 0 {
		seen := make(map[string]bool)
		unique := make([]string, 0, len(g.Exec.Commands))
		for _, cmd := range g.Exec.Commands {
			if !seen[cmd] {
				seen[cmd] = true
				unique = append(unique, cmd)
			}
		}
		g.Exec.Commands = unique
	}

	// Deduplicate KV rules
	if g.KV != nil && len(g.KV.Rules) > 0 {
		seen := make(map[string]bool)
		unique := make([]KeyValueRule, 0, len(g.KV.Rules))
		for _, rule := range g.KV.Rules {
			key := rule.Operation + ":"
			for _, k := range rule.Keys {
				key += k + ","
			}
			if !seen[key] {
				seen[key] = true
				unique = append(unique, rule)
			}
		}
		g.KV.Rules = unique
	}
}

// Difference returns capabilities in g that are not covered by other.
func (g *GrantSet) Difference(other *GrantSet) GrantSet {
	if other == nil {
		return g.Clone()
	}

	result := GrantSet{}

	result.Network = g.diffNetwork(other)
	result.FS = g.diffFS(other)
	result.Env = g.diffEnv(other)
	result.Exec = g.diffExec(other)
	result.KV = g.diffKV(other)

	return result
}

func (g *GrantSet) diffNetwork(other *GrantSet) *NetworkCapability {
	if g.Network == nil {
		return nil
	}
	var rules []NetworkRule
	for _, rule := range g.Network.Rules {
		if !other.containsNetworkRule(rule) {
			rules = append(rules, rule)
		}
	}
	if len(rules) == 0 {
		return nil
	}
	return &NetworkCapability{Rules: rules}
}

func (g *GrantSet) diffFS(other *GrantSet) *FileSystemCapability {
	if g.FS == nil {
		return nil
	}
	var rules []FileSystemRule
	for _, rule := range g.FS.Rules {
		if !other.containsFSRule(rule) {
			rules = append(rules, rule)
		}
	}
	if len(rules) == 0 {
		return nil
	}
	return &FileSystemCapability{Rules: rules}
}

func (g *GrantSet) diffEnv(other *GrantSet) *EnvironmentCapability {
	if g.Env == nil {
		return nil
	}
	var vars []string
	for _, v := range g.Env.Variables {
		if !other.containsEnvVar(v) {
			vars = append(vars, v)
		}
	}
	if len(vars) == 0 {
		return nil
	}
	return &EnvironmentCapability{Variables: vars}
}

func (g *GrantSet) diffExec(other *GrantSet) *ExecCapability {
	if g.Exec == nil {
		return nil
	}
	var cmds []string
	for _, cmd := range g.Exec.Commands {
		if !other.containsExecCmd(cmd) {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return &ExecCapability{Commands: cmds}
}

func (g *GrantSet) diffKV(other *GrantSet) *KeyValueCapability {
	if g.KV == nil {
		return nil
	}
	var rules []KeyValueRule
	for _, rule := range g.KV.Rules {
		if !other.containsKVRule(rule) {
			rules = append(rules, rule)
		}
	}
	if len(rules) == 0 {
		return nil
	}
	return &KeyValueCapability{Rules: rules}
}

// Helpers
func (g *GrantSet) containsNetworkRule(rule NetworkRule) bool {
	if g.Network == nil {
		return false
	}
	for _, r := range g.Network.Rules {
		if networkRulesEqual(r, rule) {
			return true
		}
	}
	return false
}

func (g *GrantSet) containsFSRule(rule FileSystemRule) bool {
	if g.FS == nil {
		return false
	}
	for _, r := range g.FS.Rules {
		if fsRulesEqual(r, rule) {
			return true
		}
	}
	return false
}

func (g *GrantSet) containsEnvVar(v string) bool {
	if g.Env == nil {
		return false
	}
	for _, ev := range g.Env.Variables {
		if ev == v {
			return true
		}
	}
	return false
}

func (g *GrantSet) containsExecCmd(cmd string) bool {
	if g.Exec == nil {
		return false
	}
	for _, c := range g.Exec.Commands {
		if c == cmd {
			return true
		}
	}
	return false
}

func (g *GrantSet) containsKVRule(rule KeyValueRule) bool {
	if g.KV == nil {
		return false
	}
	for _, r := range g.KV.Rules {
		if kvRulesEqual(r, rule) {
			return true
		}
	}
	return false
}

func networkRulesEqual(a, b NetworkRule) bool {
	if len(a.Hosts) != len(b.Hosts) || len(a.Ports) != len(b.Ports) {
		return false
	}
	for i := range a.Hosts {
		if a.Hosts[i] != b.Hosts[i] {
			return false
		}
	}
	for i := range a.Ports {
		if a.Ports[i] != b.Ports[i] {
			return false
		}
	}
	return true
}

func fsRulesEqual(a, b FileSystemRule) bool {
	if len(a.Read) != len(b.Read) || len(a.Write) != len(b.Write) {
		return false
	}
	for i := range a.Read {
		if a.Read[i] != b.Read[i] {
			return false
		}
	}
	for i := range a.Write {
		if a.Write[i] != b.Write[i] {
			return false
		}
	}
	return true
}

func kvRulesEqual(a, b KeyValueRule) bool {
	if a.Operation != b.Operation || len(a.Keys) != len(b.Keys) {
		return false
	}
	for i := range a.Keys {
		if a.Keys[i] != b.Keys[i] {
			return false
		}
	}
	return true
}
