package net

import (
	"context"
	"time"

	"github.com/whiskeyjimbo/reglet/wireformat"
)

// Re-export wire format types from shared wireformat package
type (
	ContextWireFormat = wireformat.ContextWireFormat
	DNSRequestWire    = wireformat.DNSRequestWire
	DNSResponseWire   = wireformat.DNSResponseWire
	TCPRequestWire    = wireformat.TCPRequestWire
	TCPResponseWire   = wireformat.TCPResponseWire
)


// createContextWireFormat extracts relevant info from a Go context into the wire format.
func createContextWireFormat(ctx context.Context) ContextWireFormat {
	wire := ContextWireFormat{}

	// Handle deadline
	if deadline, ok := ctx.Deadline(); ok {
		wire.Deadline = &deadline
		wire.TimeoutMs = time.Until(deadline).Milliseconds()
	}

	// Handle cancellation
	select {
	case <-ctx.Done():
		wire.Cancelled = true
	default:
		// Not cancelled yet
	}

	// TODO: Extract request_id if available in context values
	// Example: if reqID, ok := ctx.Value("request_id").(string); ok { wire.RequestID = reqID }

	return wire
}
