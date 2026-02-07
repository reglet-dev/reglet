package hostfuncs

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/reglet-dev/reglet-abi/hostfunc"
)

// FuzzDNSRequestParsing fuzzes DNS request wire format parsing
// TARGETS: JSON unmarshaling of hostfunc.DNSRequest
// EXPECTED FAILURES: Malformed JSON, invalid hostnames
func FuzzDNSRequestParsing(f *testing.F) {
	// Seed with valid request
	validReq := hostfunc.DNSRequest{
		Hostname: "example.com",
		Type:     "A",
	}
	validJSON, _ := json.Marshal(validReq)
	f.Add(validJSON)

	// Edge cases
	f.Add([]byte(`{"hostname":"` + strings.Repeat("a", 256) + `","type":"A"}`)) // Too long hostname
	f.Add([]byte(`{"hostname":"example.com","type":"INVALID"}`))
	f.Add([]byte(`{"hostname":"","type":"A"}`))
	f.Add([]byte(`{"hostname":"example.com","type":"A","nameserver":"1.1.1.1:53"}`))
	f.Add([]byte(`{"hostname":"example.com","type":"A","nameserver":"[::1]:53"}`))

	f.Fuzz(func(t *testing.T, jsonData []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("PANIC on input: %v", r)
			}
		}()

		var req hostfunc.DNSRequest
		_ = json.Unmarshal(jsonData, &req)
		// Just ensure no panic on parsing
	})
}
