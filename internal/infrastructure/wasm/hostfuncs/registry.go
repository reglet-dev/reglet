package hostfuncs

import (
	"context"

	"github.com/reglet-dev/reglet-abi/hostfunc"
	"github.com/reglet-dev/reglet/internal/domain/capability"
	"github.com/reglet-dev/reglet/internal/infrastructure/build"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// RegisterHostFunctions registers all host functions with the wazero runtime.
//
// The handlers perform:
// 1. Memory operations (read request from guest, write response to guest)
// 2. Capability checking using the CapabilityChecker
// 3. Delegation to SDK's PerformXXX functions for the actual work
func RegisterHostFunctions(ctx context.Context, runtime wazero.Runtime, version build.Info, caps map[string]capability.GrantSet) error {
	// Convert internal capabilities to valid ABI hostfunc.GrantSet for the checker
	abiCaps := make(map[string]*hostfunc.GrantSet)
	for k, v := range caps {
		abiCaps[k] = toABIGrantSet(v)
	}

	checker := NewCapabilityChecker(abiCaps)

	// Create host module "reglet_host"
	builder := runtime.NewHostModuleBuilder("reglet_host")

	// DNS lookup - delegates to SDK's PerformDNSLookup
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
			DNSLookup(ctx, mod, stack, checker)
		}), []api.ValueType{api.ValueTypeI64}, []api.ValueType{api.ValueTypeI64}).
		Export("dns_lookup")

	// HTTP request - uses DNS pinning for SSRF protection
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
			HTTPRequest(ctx, mod, stack, checker, version)
		}), []api.ValueType{api.ValueTypeI64}, []api.ValueType{api.ValueTypeI64}).
		Export("http_request")

	// TCP connect - delegates to SDK's PerformTCPConnect
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
			TCPConnect(ctx, mod, stack, checker)
		}), []api.ValueType{api.ValueTypeI64}, []api.ValueType{api.ValueTypeI64}).
		Export("tcp_connect")

	// SMTP connect - delegates to SDK's PerformSMTPConnect
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
			SMTPConnect(ctx, mod, stack, checker)
		}), []api.ValueType{api.ValueTypeI64}, []api.ValueType{api.ValueTypeI64}).
		Export("smtp_connect")

	// Exec command - delegates to SDK's PerformSecureExecCommand
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
			ExecCommand(ctx, mod, stack, checker)
		}), []api.ValueType{api.ValueTypeI64}, []api.ValueType{api.ValueTypeI64}).
		Export("exec_command")

	// Logging function (Reglet-specific, no SDK equivalent)
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
			LogMessage(ctx, mod, stack)
		}), []api.ValueType{api.ValueTypeI64}, []api.ValueType{}).
		Export("log_message")

	// Instantiate the host module
	_, err := builder.Instantiate(ctx)
	return err
}

func toABIGrantSet(gs capability.GrantSet) *hostfunc.GrantSet {
	out := &hostfunc.GrantSet{}

	if gs.Network != nil {
		out.Network = &hostfunc.NetworkCapability{}
		for _, r := range gs.Network.Rules {
			out.Network.Rules = append(out.Network.Rules, hostfunc.NetworkRule{
				Hosts: r.Hosts,
				Ports: r.Ports,
			})
		}
	}

	if gs.FS != nil {
		out.FS = &hostfunc.FileSystemCapability{}
		for _, r := range gs.FS.Rules {
			out.FS.Rules = append(out.FS.Rules, hostfunc.FileSystemRule{
				Read:  r.Read,
				Write: r.Write,
			})
		}
	}

	if gs.Env != nil {
		out.Env = &hostfunc.EnvironmentCapability{
			Variables: gs.Env.Variables,
		}
	}

	if gs.Exec != nil {
		out.Exec = &hostfunc.ExecCapability{
			Commands: gs.Exec.Commands,
		}
	}

	if gs.KV != nil {
		out.KV = &hostfunc.KeyValueCapability{}
		for _, r := range gs.KV.Rules {
			out.KV.Rules = append(out.KV.Rules, hostfunc.KeyValueRule{
				Operation: r.Operation,
				Keys:      r.Keys,
			})
		}
	}
	return out
}
