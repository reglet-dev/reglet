package sensitivedata

import "runtime"

// SecureString holds a sensitive value that is zeroed when no longer needed.
// Use this for high-value secrets like private keys or master passwords.
type SecureString struct {
	value []byte
}

// NewSecureString creates a secure string from the input.
// The input is copied and the original should be zeroed by the caller.
func NewSecureString(s string) *SecureString {
	ss := &SecureString{
		value: []byte(s),
	}
	// Set finalizer to zero memory when garbage collected
	runtime.SetFinalizer(ss, func(ss *SecureString) {
		ss.Zero()
	})
	return ss
}

// String returns the secret value. Avoid logging this!
func (ss *SecureString) String() string {
	return string(ss.value)
}

// Zero overwrites the memory with zeros.
// Call this explicitly when done with the secret.
func (ss *SecureString) Zero() {
	for i := range ss.value {
		ss.value[i] = 0
	}
}
