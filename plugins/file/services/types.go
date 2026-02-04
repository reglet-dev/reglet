package services

// CheckInput defines the input for file checks.
type CheckInput struct {
	Path      string `json:"path" jsonschema:"required,description=File or directory path"`
	Operation string `json:"operation" jsonschema:"required,enum=exists,enum=permissions,enum=checksum,enum=content,description=Check operation to perform"`
	// For permissions check
	Permissions string `json:"permissions,omitempty" jsonschema:"description=Expected permissions (e.g., 0644)"`
	// For checksum check
	Algorithm string `json:"algorithm,omitempty" jsonschema:"enum=md5,enum=sha256,enum=sha512,default=sha256,description=Checksum algorithm"`
	// For content check
	Contains string `json:"contains,omitempty" jsonschema:"description=String to search for in file"`
}

// CheckOutput contains file check results.
type CheckOutput struct {
	Path        string `json:"path" jsonschema:"description=Checked path"`
	Exists      bool   `json:"exists" jsonschema:"description=Whether path exists"`
	IsDir       bool   `json:"is_dir" jsonschema:"description=Whether path is a directory"`
	Size        int64  `json:"size,omitempty" jsonschema:"description=File size in bytes"`
	Permissions string `json:"permissions,omitempty" jsonschema:"description=File permissions (octal)"`
	ModTime     string `json:"mod_time,omitempty" jsonschema:"description=Last modification time"`
	Checksum    string `json:"checksum,omitempty" jsonschema:"description=File checksum"`
	Contains    bool   `json:"contains,omitempty" jsonschema:"description=Whether file contains search string"`
	UID         int    `json:"uid" jsonschema:"description=File owner UID"`
	GID         int    `json:"gid" jsonschema:"description=File group GID"`
	Readable    bool   `json:"readable" jsonschema:"description=Whether file is readable"`
	IsSymlink   bool   `json:"is_symlink" jsonschema:"description=Whether file is a symlink"`
	Mode        string `json:"mode,omitempty" jsonschema:"description=File mode (octal, alias for permissions)"`
}
