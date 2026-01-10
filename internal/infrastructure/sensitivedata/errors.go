package sensitivedata

import (
	"fmt"
	"strings"

	"github.com/reglet-dev/reglet/internal/application/ports"
)

// SafeError wraps an error, redacting any sensitive values in the message.
func SafeError(err error, provider ports.SensitiveValueProvider) error {
	if err == nil {
		return nil
	}
	if provider == nil {
		return err
	}

	msg := err.Error()
	for _, secret := range provider.AllValues() {
		if secret != "" && strings.Contains(msg, secret) {
			msg = strings.ReplaceAll(msg, secret, "[REDACTED]")
		}
	}

	if msg == err.Error() {
		return err // No redaction needed, return original error to preserve type
	}

	return fmt.Errorf("%s", msg)
}
