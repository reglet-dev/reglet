package values

import (
	"bytes"
	"testing"
)

func TestNewDigest(t *testing.T) {
	tests := []struct {
		name    string
		algo    string
		val     string
		wantErr bool
	}{
		{"ValidSHA256", "sha256", "abc123456", false},
		{"ValidSHA512", "sha512", "abc123456", false},
		{"InvalidAlgo", "md5", "abc123456", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewDigest(tt.algo, tt.val)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewDigest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Algorithm() != tt.algo {
					t.Errorf("Algorithm() = %v, want %v", got.Algorithm(), tt.algo)
				}
				if got.Value() != tt.val {
					t.Errorf("Value() = %v, want %v", got.Value(), tt.val)
				}
			}
		})
	}
}

func TestParseDigest(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		valid     bool
		wantAlgo  string
		wantValue string
	}{
		{"ValidSHA256", "sha256:abcd", true, "sha256", "abcd"},
		{"ValidSHA512", "sha512:1234", true, "sha512", "1234"},
		{"MissingAlgo", ":abcd", false, "", "abcd"},
		{"NoColon", "sha256abcd", false, "", ""},
		{"MultipleColons", "sha256:abc:def", true, "sha256", "abc:def"}, // implementation splits N=2
	}

	// Wait, checkingNewDigest implementation:
	// switch algorithm { case "sha256", "sha512": ... default: error }
	// So "" algo will fail.

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDigest(tt.input)
			if tt.valid {
				if err != nil {
					// Expecting success but got error?
					// Check if NewDigest validates emptiness.
					// NewDigest switch does not have casing for "".
					// So "MissingAlgo" test case expects error effectively if it calls NewDigest("", ...).
					// But wait, the test definition said `valid: true`.
					// Let's adjust expectation based on code reading.
					// If input is ":abcd", parts=["", "abcd"]. NewDigest("", "abcd") -> error "unsupported digest algorithm: "
					t.Errorf("ParseDigest() unexpected error = %v", err)
				} else {
					if got.Algorithm() != tt.wantAlgo {
						t.Errorf("Algorithm() = %v, want %v", got.Algorithm(), tt.wantAlgo)
					}
					if got.Value() != tt.wantValue {
						t.Errorf("Value() = %v, want %v", got.Value(), tt.wantValue)
					}
				}
			} else {
				if err == nil {
					t.Errorf("ParseDigest() expected error, got nil")
				}
			}
		})
	}
}

func TestDigest_Equals(t *testing.T) {
	d1, _ := NewDigest("sha256", "abc")
	d2, _ := NewDigest("sha256", "abc")
	d3, _ := NewDigest("sha256", "def")
	d4, _ := NewDigest("sha512", "abc")

	if !d1.Equals(d2) {
		t.Error("Identical digests should be equal")
	}
	if d1.Equals(d3) {
		t.Error("Different values should not be equal")
	}
	if d1.Equals(d4) {
		t.Error("Different algorithms should not be equal")
	}
}

func TestDigest_Verify(t *testing.T) {
	data := []byte("hello world")

	// Compute real sha256
	// echo -n "hello world" | sha256sum -> b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9
	expectedHash := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"

	d, _ := NewDigest("sha256", expectedHash)

	if err := d.Verify(data); err != nil {
		t.Errorf("Verify failed for correct data: %v", err)
	}

	if err := d.Verify([]byte("wrong data")); err == nil {
		t.Error("Verify should fail for wrong data")
	}

	dBad, _ := NewDigest("sha512", "badhash")
	if err := dBad.Verify(data); err == nil {
		t.Error("Verify should fail for bad hash")
	}

	dEmpty := Digest{} // invalid algo
	if err := dEmpty.Verify(data); err == nil {
		// Expect error "unsupported algorithm: "
		t.Error("Verify should fail for empty/unsupported algo")
	}
}

func TestComputeDigestSHA256(t *testing.T) {
	data := []byte("test data")
	r := bytes.NewReader(data)

	d, err := ComputeDigestSHA256(r)
	if err != nil {
		t.Fatalf("ComputeDigestSHA256 failed: %v", err)
	}

	if d.Algorithm() != "sha256" {
		t.Errorf("Expected sha256, got %s", d.Algorithm())
	}

	if err := d.Verify(data); err != nil {
		t.Errorf("Computed digest verification failed: %v", err)
	}
}
