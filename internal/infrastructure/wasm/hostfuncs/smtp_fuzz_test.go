package hostfuncs

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/reglet-dev/reglet/wireformat"
)

// FuzzSMTPRequestParsing fuzzes SMTP request wire format parsing
// TARGETS: JSON unmarshaling of SMTPRequestWire
// EXPECTED FAILURES: Malformed JSON, invalid ports
func FuzzSMTPRequestParsing(f *testing.F) {
	// Seed with valid request
	validReq := wireformat.SMTPRequestWire{
		Host:     "smtp.example.com",
		Port:     "25",
		TLS:      false,
		StartTLS: true,
	}
	validJSON, _ := json.Marshal(validReq)
	f.Add(validJSON)

	// Edge cases
	f.Add([]byte(`{"host":"smtp.gmail.com","port":"587","starttls":true}`))
	f.Add([]byte(`{"host":"[::1]","port":"25"}`))
	f.Add([]byte(`{"host":"` + strings.Repeat("x", 1000) + `","port":"25"}`))
	f.Add([]byte(`{"host":"example.com","port":"invalid"}`))

	f.Fuzz(func(t *testing.T, jsonData []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("PANIC on input: %v", r)
			}
		}()

		var req wireformat.SMTPRequestWire
		_ = json.Unmarshal(jsonData, &req)
		// Just ensure no panic on parsing
	})
}
