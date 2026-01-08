package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	regletsdk "github.com/whiskeyjimbo/reglet/sdk"
)

// processPlugin implements the sdk.Plugin interface for process checks.
// Linux-only: reads from /proc filesystem.
type processPlugin struct{}

// Describe provides the process plugin's metadata and capabilities.
func (p *processPlugin) Describe(ctx context.Context) (regletsdk.Metadata, error) {
	return regletsdk.Metadata{
		Name:        "process",
		Version:     "1.0.0",
		Description: "Check running processes via /proc (Linux-only)",
		Capabilities: []regletsdk.Capability{
			{
				Kind:    "fs",
				Pattern: "read:/proc/**",
			},
		},
	}, nil
}

// ProcessConfig defines configuration for process checks.
type ProcessConfig struct {
	// Name matches the process name (from /proc/<pid>/comm)
	Name string `json:"name,omitempty" description:"Process name to check (e.g., nginx, sshd)"`

	// PID checks a specific process ID
	PID int `json:"pid,omitempty" description:"Process ID to check"`

	// Pattern matches against the full command line (regex)
	Pattern string `json:"pattern,omitempty" description:"Regex pattern to match against cmdline"`
}

// Schema generates the JSON schema for the plugin's configuration.
func (p *processPlugin) Schema(ctx context.Context) ([]byte, error) {
	return regletsdk.GenerateSchema(ProcessConfig{})
}

// Check executes process validation based on the provided configuration.
func (p *processPlugin) Check(ctx context.Context, config regletsdk.Config) (regletsdk.Evidence, error) {
	var cfg ProcessConfig
	if err := regletsdk.ValidateConfig(config, &cfg); err != nil {
		return regletsdk.Evidence{
			Status: false,
			Error:  regletsdk.ToErrorDetail(&regletsdk.ConfigError{Err: err}),
		}, nil
	}

	// Require at least one search criterion
	if cfg.Name == "" && cfg.PID == 0 && cfg.Pattern == "" {
		return regletsdk.Evidence{
			Status: false,
			Error:  regletsdk.ToErrorDetail(&regletsdk.ConfigError{Field: "name/pid/pattern", Err: fmt.Errorf("at least one of name, pid, or pattern is required")}),
		}, nil
	}

	return checkProcesses(cfg)
}

// ProcessInfo holds information about a running process.
type ProcessInfo struct {
	PID         int    `json:"pid"`
	Name        string `json:"name"`
	Cmdline     string `json:"cmdline"`
	State       string `json:"state"`
	UID         int    `json:"uid"`
	PPID        int    `json:"ppid"`
	Threads     int    `json:"threads"`
	MemoryRSSKB int64  `json:"memory_rss_kb"`
}

// checkProcesses performs the actual process check logic.
// Returns found (bool) and count (int). Use --format json for full details.
func checkProcesses(cfg ProcessConfig) (regletsdk.Evidence, error) {
	result := map[string]interface{}{
		"found": false,
		"count": 0,
	}

	// Compile pattern if provided
	var pattern *regexp.Regexp
	if cfg.Pattern != "" {
		var err error
		pattern, err = regexp.Compile(cfg.Pattern)
		if err != nil {
			return regletsdk.Failure("config", fmt.Sprintf("invalid regex pattern: %v", err)), nil
		}
	}

	// If checking specific PID, just check that one
	if cfg.PID > 0 {
		info, err := readProcessInfo(cfg.PID)
		if err != nil {
			// Process doesn't exist or not readable
			return regletsdk.Success(result), nil
		}

		// Check if it matches name/pattern filters
		if cfg.Name != "" && info.Name != cfg.Name {
			return regletsdk.Success(result), nil
		}
		if pattern != nil && !pattern.MatchString(info.Cmdline) {
			return regletsdk.Success(result), nil
		}

		result["found"] = true
		result["count"] = 1
		return regletsdk.Success(result), nil
	}

	// Enumerate all processes
	count, err := countProcesses(cfg.Name, pattern)
	if err != nil {
		return regletsdk.Failure("fs", fmt.Sprintf("failed to enumerate processes: %v", err)), nil
	}

	result["found"] = count > 0
	result["count"] = count

	return regletsdk.Success(result), nil
}

// countProcesses enumerates /proc and counts processes matching criteria.
func countProcesses(name string, pattern *regexp.Regexp) (int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, fmt.Errorf("cannot read /proc: %w", err)
	}

	count := 0

	for _, entry := range entries {
		// Skip non-directories and non-numeric entries
		if !entry.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			// Not a PID directory
			continue
		}

		info, err := readProcessInfo(pid)
		if err != nil {
			// Process may have exited or permission denied - skip
			continue
		}

		// Apply filters
		if name != "" && info.Name != name {
			continue
		}
		if pattern != nil && !pattern.MatchString(info.Cmdline) {
			continue
		}

		count++
	}

	return count, nil
}

// readProcessInfo reads process information from /proc/<pid>/.
func readProcessInfo(pid int) (*ProcessInfo, error) {
	procDir := filepath.Join("/proc", strconv.Itoa(pid))

	// Check if process exists
	if _, err := os.Stat(procDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("process %d does not exist", pid)
	}

	info := &ProcessInfo{PID: pid}

	// Read process name from /proc/<pid>/comm
	commPath := filepath.Join(procDir, "comm")
	commData, err := os.ReadFile(commPath)
	if err == nil {
		info.Name = strings.TrimSpace(string(commData))
	}

	// Read command line from /proc/<pid>/cmdline
	cmdlinePath := filepath.Join(procDir, "cmdline")
	cmdlineData, err := os.ReadFile(cmdlinePath)
	if err == nil {
		// cmdline is NUL-separated, replace with spaces
		info.Cmdline = strings.ReplaceAll(string(cmdlineData), "\x00", " ")
		info.Cmdline = strings.TrimSpace(info.Cmdline)
	}

	// Read status info from /proc/<pid>/status
	statusPath := filepath.Join(procDir, "status")
	_ = parseStatusFile(statusPath, info) // Non-fatal, continue with partial info

	// Read RSS from /proc/<pid>/stat (field 24 is rss in pages)
	statPath := filepath.Join(procDir, "stat")
	if rss, err := parseRSSFromStat(statPath); err == nil {
		// Convert pages to KB (page size is typically 4KB)
		info.MemoryRSSKB = rss * 4
	}

	return info, nil
}

// parseStatusFile parses /proc/<pid>/status for state, uid, ppid, threads.
func parseStatusFile(path string, info *ProcessInfo) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "State":
			// Format: "S (sleeping)" - take first char
			if len(value) > 0 {
				info.State = string(value[0])
			}
		case "Uid":
			// Format: "1000\t1000\t1000\t1000" - take first
			fields := strings.Fields(value)
			if len(fields) > 0 {
				if uid, err := strconv.Atoi(fields[0]); err == nil {
					info.UID = uid
				}
			}
		case "PPid":
			if ppid, err := strconv.Atoi(value); err == nil {
				info.PPID = ppid
			}
		case "Threads":
			if threads, err := strconv.Atoi(value); err == nil {
				info.Threads = threads
			}
		}
	}

	return scanner.Err()
}

// parseRSSFromStat parses /proc/<pid>/stat for RSS (field 24, 0-indexed 23).
func parseRSSFromStat(path string) (int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	// The stat file has the format: pid (comm) state ppid ...
	// The comm field can contain spaces and parentheses, so we need to find
	// the closing paren first
	content := string(data)
	closeParenIdx := strings.LastIndex(content, ")")
	if closeParenIdx == -1 || closeParenIdx >= len(content)-2 {
		return 0, fmt.Errorf("invalid stat format")
	}

	// Fields after the closing paren
	fields := strings.Fields(content[closeParenIdx+2:])
	// RSS is field 22 in the remaining fields (0-indexed)
	// Original fields: 1=pid, 2=comm, 3=state, ..., 24=rss
	// After stripping pid and comm: index 21 is rss
	if len(fields) < 22 {
		return 0, fmt.Errorf("not enough fields in stat")
	}

	rss, err := strconv.ParseInt(fields[21], 10, 64)
	if err != nil {
		return 0, err
	}

	return rss, nil
}
