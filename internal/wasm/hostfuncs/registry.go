package hostfuncs

import (
	"context"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// RegisterHostFunctions registers all host functions with the wazero runtime
func RegisterHostFunctions(ctx context.Context, runtime wazero.Runtime, caps []Capability) error {
	checker := NewCapabilityChecker(caps)

	// Create host module "reglet_host"
	builder := runtime.NewHostModuleBuilder("reglet_host")

	// Register DNS lookup function
	// Parameters: hostnamePtr (i32), hostnameLen (i32), recordTypePtr (i32), recordTypeLen (i32)
	// Returns: resultPtr (i32)
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
			DNSLookup(ctx, mod, stack, checker)
		}), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).
		Export("dns_lookup")

	// Register HTTP request function
	// Parameters: urlPtr (i32), urlLen (i32), methodPtr (i32), methodLen (i32),
	//             headersPtr (i32), headersLen (i32), bodyPtr (i32), bodyLen (i32)
	// Returns: resultPtr (i32)
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
			HTTPRequest(ctx, mod, stack, checker)
		}), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32,
			api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).
		Export("http_request")

	// Register TCP connect function
	// Parameters: hostPtr (i32), hostLen (i32), portPtr (i32), portLen (i32),
	//             timeoutMs (i32), useTLS (i32)
	// Returns: resultPtr (i32)
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
			TCPConnect(ctx, mod, stack, checker)
		}), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32,
			api.ValueTypeI32, api.ValueTypeI32}, []api.ValueType{api.ValueTypeI32}).
		Export("tcp_connect")

	// Instantiate the host module
	_, err := builder.Instantiate(ctx)
	return err
}
