// Package watcher provides abstractions for file system watching.
// It defines the Watcher port interface and provides an fsnotify-based
// implementation for cross-platform file modification detection.
package watcher

// Watcher defines the port interface for file system watching.
// This interface follows the Hexagonal Architecture pattern, allowing
// the application to remain decoupled from the concrete file watching
// implementation (fsnotify).
//
// Implementations must be safe for concurrent use from multiple goroutines.
type Watcher interface {
	// Add starts watching the specified file or directory for changes.
	// Returns an error if the path doesn't exist or cannot be watched.
	Add(path string) error

	// Remove stops watching the specified file or directory.
	// Returns an error if the path is not currently being watched.
	Remove(path string) error

	// Events returns a read-only channel that receives file system events.
	// The channel is closed when Close() is called.
	Events() <-chan Event

	// Errors returns a read-only channel that receives watcher errors.
	// The channel is closed when Close() is called.
	Errors() <-chan error

	// Close stops all watching and releases resources.
	// After Close returns, the Events and Errors channels will be closed.
	Close() error
}

// Event represents a file system change event.
type Event struct {
	// Path is the absolute path to the file or directory that changed.
	Path string

	// Op is the type of file system operation that occurred.
	Op Operation
}

// Operation represents the type of file system operation.
type Operation int

const (
	// Create indicates a new file or directory was created.
	Create Operation = 1 << iota

	// Write indicates a file was modified.
	Write

	// Remove indicates a file or directory was removed.
	Remove

	// Rename indicates a file or directory was renamed.
	Rename

	// Chmod indicates file permissions were changed.
	Chmod
)

// String returns a human-readable representation of the operation.
func (op Operation) String() string {
	switch op {
	case Create:
		return "CREATE"
	case Write:
		return "WRITE"
	case Remove:
		return "REMOVE"
	case Rename:
		return "RENAME"
	case Chmod:
		return "CHMOD"
	default:
		return "UNKNOWN"
	}
}
