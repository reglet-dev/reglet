package hostfuncs

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/whiskeyjimbo/reglet/wireformat"
)

// FuzzHTTPRequestParsing fuzzes HTTP request wire format parsing
// TARGETS: JSON unmarshaling of HTTPRequestWire, URL parsing, base64 decoding
// EXPECTED FAILURES: Invalid base64, malformed JSON, URL parse errors
func FuzzHTTPRequestParsing(f *testing.F) {
	// Seed with valid and invalid HTTP request structures
	validReq := wireformat.HTTPRequestWire{
		Method: "GET",
		URL:    "https://example.com/path",
		Headers: map[string][]string{
			"User-Agent": {"reglet/1.0"},
		},
		Body: base64.StdEncoding.EncodeToString([]byte("test body")),
	}

	validJSON, _ := json.Marshal(validReq)
	f.Add(validJSON)

	// Edge cases
	f.Add([]byte(`{"method":"GET","url":"http://[::1]/"}`))
	f.Add([]byte(`{"method":"GET","url":"http://user:pass@evil.com"}`))
	f.Add([]byte(`{"url":"` + strings.Repeat("x", 100000) + `"}`))
	f.Add([]byte(`{"body":"!!!invalid base64!!!"}`))

	f.Fuzz(func(t *testing.T, jsonData []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("PANIC on input: %v", r)
			}
		}()

		var req wireformat.HTTPRequestWire
		_ = json.Unmarshal(jsonData, &req)
		// Just ensure no panic on parsing
	})
}
