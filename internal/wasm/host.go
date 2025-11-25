package wasm

// host.go contains host functions that plugins can call
// These enforce capability restrictions and provide sandboxed access to:
// - Filesystem operations (fs:read, fs:write)
// - Network operations (network:outbound)
// - Environment variables (env:*)
// - Command execution (exec:*)

// TODO: Implement host functions for capability enforcement
// This will be expanded in Phase 1a to include:
//
// Host function interface:
// - fs_read(path: string) -> result<bytes, error>
// - fs_write(path: string, data: bytes) -> result<void, error>
// - net_connect(host: string, port: u16) -> result<connection, error>
// - env_get(name: string) -> result<string, error>
// - exec_run(command: string, args: []string) -> result<output, error>
//
// Each function checks granted capabilities before proceeding:
// 1. Look up plugin's granted capabilities
// 2. Check if requested operation matches any granted pattern
// 3. If no match, return capability violation error
// 4. If match, execute operation in sandboxed manner
//
// Capability patterns use glob matching:
// - fs:read:/etc/** matches /etc/ssh/sshd_config
// - fs:read:/var/log/*.log matches /var/log/app.log but not /var/log/app/debug.log
// - network:outbound:80,443 matches ports 80 and 443 only
// - env:AWS_* matches AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, etc.

// CapabilityManager handles capability checking and enforcement
type CapabilityManager struct {
	// TODO: Implement capability management
	// - Track granted capabilities per plugin
	// - Provide methods to check if operation is allowed
	// - Log capability violations for audit
}

// NewCapabilityManager creates a new capability manager
func NewCapabilityManager() *CapabilityManager {
	return &CapabilityManager{}
}

// CheckCapability checks if a plugin has the required capability
func (cm *CapabilityManager) CheckCapability(_ string, _ Capability) error {
	// TODO: Implement capability checking logic
	// For now, always allow (unsafe - for development only)
	return nil
}
