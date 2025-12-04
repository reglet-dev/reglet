package hostfuncs

import (
	"context"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// RegisterHostFunctions registers all host functions with the wazero runtime
func RegisterHostFunctions(ctx context.Context, runtime wazero.Runtime, caps map[string][]Capability) error {
	checker := NewCapabilityChecker(caps)

	// Create host module "reglet_host"
	builder := runtime.NewHostModuleBuilder("reglet_host")

	// Register DNS lookup function
	// Parameters: requestPacked (i64) - packed ptr+len of DNSRequestWire JSON
	// Returns: responsePacked (i64) - packed ptr+len of DNSResponseWire JSON
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
			DNSLookup(ctx, mod, stack, checker)
		}), []api.ValueType{api.ValueTypeI64}, []api.ValueType{api.ValueTypeI64}).
		Export("dns_lookup")

	// Register HTTP request function
	// Parameters: http_requestPacked (i64) - packed ptr+len of HTTPRequestWire JSON
	// Returns: http_responsePacked (i64) - packed ptr+len of HTTPResponseWire JSON
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
			HTTPRequest(ctx, mod, stack, checker) // Now uncommented
		}), []api.ValueType{api.ValueTypeI64}, []api.ValueType{api.ValueTypeI64}).
		Export("http_request")

	// Register TCP connect function
	// Parameters: tcp_requestPacked (i64) - packed ptr+len of TCPRequestWire JSON
	// Returns: tcp_responsePacked (i64) - packed ptr+len of TCPResponseWire JSON
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
			TCPConnect(ctx, mod, stack, checker)
		}), []api.ValueType{api.ValueTypeI64}, []api.ValueType{api.ValueTypeI64}).
		Export("tcp_connect")
	
	// Register logging function
	builder.NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, mod api.Module, stack []uint64) {
			LogMessage(ctx, mod, stack)
		}), []api.ValueType{api.ValueTypeI64}, []api.ValueType{}). // No return value
		Export("log_message")


	// Instantiate the host module
	_, err := builder.Instantiate(ctx)
	return err
}
