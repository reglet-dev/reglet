package net

import (
	"github.com/whiskeyjimbo/reglet/wireformat"
)

// Re-export wire format types from shared wireformat package
// This file has no build tags so tests can use these types
type (
	ContextWireFormat  = wireformat.ContextWireFormat
	DNSRequestWire     = wireformat.DNSRequestWire
	DNSResponseWire    = wireformat.DNSResponseWire
	TCPRequestWire     = wireformat.TCPRequestWire
	TCPResponseWire    = wireformat.TCPResponseWire
	SMTPRequestWire    = wireformat.SMTPRequestWire
	SMTPResponseWire   = wireformat.SMTPResponseWire
)
