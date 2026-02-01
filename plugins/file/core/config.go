package core

// FileConfig defines the configuration for file checks.
type FileConfig struct {
	Path        string `json:"path" jsonschema:"required,description=File or directory path"`
	Operation   string `json:"operation" jsonschema:"required,enum=exists,enum=permissions,enum=checksum,enum=content,description=Check operation to perform"`
	Permissions string `json:"permissions,omitempty" jsonschema:"description=Expected permissions in octal format like 0644"`
	Checksum    string `json:"checksum,omitempty" jsonschema:"description=Expected SHA256 checksum"`
	Contains    string `json:"contains,omitempty" jsonschema:"description=String that file should contain"`
}
