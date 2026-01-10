package ports

// SensitiveValueProvider tracks and provides all sensitive values for protection.
// This is a PORT - the application defines what it needs, infrastructure provides it.
type SensitiveValueProvider interface {
	// Track registers a sensitive value to be protected (redacted).
	Track(value string)

	// AllValues returns all tracked sensitive values.
	AllValues() []string
}

// SecretResolver resolves named secrets for profile interpolation.
// Implementations automatically track resolved values for redaction.
type SecretResolver interface {
	// Resolve returns the secret value by name.
	// The resolved value is automatically tracked for redaction.
	Resolve(name string) (string, error)
}
