//go:build wasip1

package net

import (
	"context"

	sdkcontext "github.com/reglet-dev/reglet/sdk/internal/context"
)

// createContextWireFormat extracts relevant info from a Go context into the wire format.
// This is now a wrapper around sdkcontext.ContextToWire for backwards compatibility.
func createContextWireFormat(ctx context.Context) ContextWireFormat {
	return sdkcontext.ContextToWire(ctx)
}
