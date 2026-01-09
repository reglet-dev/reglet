package hostfuncs

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/reglet-dev/reglet/wireformat"
)

// FuzzTCPRequestParsing fuzzes TCP request wire format parsing
// TARGETS: JSON unmarshaling of TCPRequestWire
// EXPECTED FAILURES: Malformed JSON, invalid ports
func FuzzTCPRequestParsing(f *testing.F) {
	// Seed with valid request
	validReq := wireformat.TCPRequestWire{
		Host: "example.com",
		Port: "80",
		TLS:  false,
	}
	validJSON, _ := json.Marshal(validReq)
	f.Add(validJSON)

	// Edge cases
	f.Add([]byte(`{"host":"127.0.0.1","port":"80","tls":true}`))
	f.Add([]byte(`{"host":"[::1]","port":"8080","tls":false}`))
	f.Add([]byte(`{"host":"example.com","port":"65536","tls":false}`)) // Invalid port
	f.Add([]byte(`{"host":"` + strings.Repeat("x", 1000) + `","port":"80"}`))
	f.Add([]byte(`{"host":"example.com","port":"-1"}`))

	f.Fuzz(func(t *testing.T, jsonData []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("PANIC on input: %v", r)
			}
		}()

		var req wireformat.TCPRequestWire
		_ = json.Unmarshal(jsonData, &req)
		// Just ensure no panic on parsing
	})
}
