// Package capabilities provides utilities for managing plugin capabilities.
package capabilities

import (
	"github.com/reglet-dev/reglet-abi/hostfunc"
	"github.com/reglet-dev/reglet/internal/domain/capability"
)

// FromABI converts an ABI GrantSet to an internal GrantSet.
func FromABI(abiGS *hostfunc.GrantSet) capability.GrantSet {
	if abiGS == nil {
		return capability.GrantSet{}
	}

	gs := capability.GrantSet{}

	if abiGS.Network != nil {
		rules := make([]capability.NetworkRule, len(abiGS.Network.Rules))
		for i, r := range abiGS.Network.Rules {
			rules[i] = capability.NetworkRule{
				Hosts: append([]string(nil), r.Hosts...),
				Ports: append([]string(nil), r.Ports...),
			}
		}
		gs.Network = &capability.NetworkCapability{Rules: rules}
	}

	if abiGS.FS != nil {
		rules := make([]capability.FileSystemRule, len(abiGS.FS.Rules))
		for i, r := range abiGS.FS.Rules {
			rules[i] = capability.FileSystemRule{
				Read:  append([]string(nil), r.Read...),
				Write: append([]string(nil), r.Write...),
			}
		}
		gs.FS = &capability.FileSystemCapability{Rules: rules}
	}

	if abiGS.Env != nil {
		gs.Env = &capability.EnvironmentCapability{
			Variables: append([]string(nil), abiGS.Env.Variables...),
		}
	}

	if abiGS.Exec != nil {
		gs.Exec = &capability.ExecCapability{
			Commands: append([]string(nil), abiGS.Exec.Commands...),
		}
	}

	if abiGS.KV != nil {
		rules := make([]capability.KeyValueRule, len(abiGS.KV.Rules))
		for i, r := range abiGS.KV.Rules {
			rules[i] = capability.KeyValueRule{
				Operation: r.Operation,
				Keys:      append([]string(nil), r.Keys...),
			}
		}
		gs.KV = &capability.KeyValueCapability{Rules: rules}
	}

	return gs
}

// ToABI converts an internal GrantSet to an ABI GrantSet.
func ToABI(gs capability.GrantSet) *hostfunc.GrantSet {
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
