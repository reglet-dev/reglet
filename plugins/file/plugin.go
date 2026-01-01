package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"

	regletsdk "github.com/whiskeyjimbo/reglet/sdk"
)

// filePlugin implements the sdk.Plugin interface for file system operations.
type filePlugin struct{}

// Describe provides the file plugin's metadata and capabilities.
func (p *filePlugin) Describe(ctx context.Context) (regletsdk.Metadata, error) {
	return regletsdk.Metadata{
		Name:        "file",
		Version:     "1.1.0",
		Description: "File existence, content, and hash checks",
		Capabilities: []regletsdk.Capability{
			{
				Kind:    "fs",
				Pattern: "read:**",
			},
		},
	}, nil
}

type FileConfig struct {
	Path        string `json:"path" validate:"required" description:"Path to file to check"`
	ReadContent bool   `json:"read_content,omitempty" description:"Read and return file content"`
	Hash        bool   `json:"hash,omitempty" description:"Calculate SHA256 hash of file"`
}

// Schema generates the JSON schema for the plugin's configuration.
func (p *filePlugin) Schema(ctx context.Context) ([]byte, error) {
	return regletsdk.GenerateSchema(FileConfig{})
}

// Check executes file system validation based on the provided configuration.
func (p *filePlugin) Check(ctx context.Context, config regletsdk.Config) (regletsdk.Evidence, error) {
	var cfg FileConfig
	if err := regletsdk.ValidateConfig(config, &cfg); err != nil {
		return regletsdk.Evidence{
			Status: false,
			Error:  regletsdk.ToErrorDetail(&regletsdk.ConfigError{Err: err}),
		}, nil
	}

	result := map[string]interface{}{
		"path": cfg.Path,
	}

	// 1. Check Existence & Metadata (Open First to avoid TOCTOU)
	var info os.FileInfo
	var f *os.File
	var openErr error

	// Attempt to open the file first
	f, openErr = os.Open(cfg.Path)
	if openErr == nil {
		defer f.Close()
		result["readable"] = true

		// Use the open file handle to get stats (atomic check)
		info, openErr = f.Stat()
		if openErr != nil {
			return regletsdk.Failure("fs", fmt.Sprintf("stat on open file failed: %v", openErr)), nil
		}
	} else {
		result["readable"] = false

		// Open failed. Fallback to os.Stat to check existence/metadata.
		// It might exist but be unreadable (permission denied, directory, etc.)
		var statErr error
		info, statErr = os.Stat(cfg.Path)
		if statErr != nil {
			if os.IsNotExist(statErr) || os.IsNotExist(openErr) {
				result["exists"] = false
				return regletsdk.Success(result), nil
			}
			return regletsdk.Failure("fs", fmt.Sprintf("stat failed: %v", statErr)), nil
		}
	}

	result["exists"] = true
	result["is_dir"] = info.IsDir()
	result["size"] = info.Size()
	result["mode"] = fmt.Sprintf("%04o", info.Mode().Perm())
	result["permissions"] = info.Mode().String()
	result["mod_time"] = info.ModTime().Format(time.RFC3339)

	// Update readable status if it's a directory (directories might trigger openErr if utilizing file flags,
	// but os.Open(".") usually works. If os.Open failed but Stat passed, we keep readable=false unless logic dictates otherwise)
	if info.IsDir() {
		// If we couldn't open the dir, it's not readable (listing would fail)
		// So result["readable"] from openErr is correct.
	}

	// Attempt to get ownership
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		result["uid"] = stat.Uid
		result["gid"] = stat.Gid
	}

	// Check for symlink (Must use Lstat on path, inherently racy but necessary for detection)
	// We can't use f.Stat() for this because it follows links.
	linfo, err := os.Lstat(cfg.Path)
	if err == nil && linfo.Mode()&os.ModeSymlink != 0 {
		result["is_symlink"] = true
		target, err := os.Readlink(cfg.Path)
		if err == nil {
			result["symlink_target"] = target
		}
	} else {
		result["is_symlink"] = false
	}

	// 2. Read Content (if requested and not a directory)
	if cfg.ReadContent && !info.IsDir() {
		if f != nil {
			// Use the existing handle
			if _, err := f.Seek(0, 0); err != nil {
				return regletsdk.Failure("fs", fmt.Sprintf("seek failed: %v", err)), nil
			}
			content, err := io.ReadAll(f)
			if err != nil {
				return regletsdk.Failure("fs", fmt.Sprintf("read failed: %v", err)), nil
			}
			result["content_b64"] = base64.StdEncoding.EncodeToString(content)
			result["encoding"] = "base64"
		} else {
			// File exists but we couldn't open it (shouldn't happen if we passed the checks above,
			// unless we implement fine-grained permission logic, but here readable was false)
			return regletsdk.Failure("fs", fmt.Sprintf("read failed: file not readable")), nil
		}
	}

	// 3. Calculate Hash (if requested and not a directory)
	if cfg.Hash && !info.IsDir() {
		if f != nil {
			// Use the existing handle
			if _, err := f.Seek(0, 0); err != nil {
				return regletsdk.Failure("fs", fmt.Sprintf("seek for hash failed: %v", err)), nil
			}
			hasher := sha256.New()
			if _, err := io.Copy(hasher, f); err != nil {
				return regletsdk.Failure("fs", fmt.Sprintf("hash calculation failed: %v", err)), nil
			}
			result["sha256"] = hex.EncodeToString(hasher.Sum(nil))
		} else {
			return regletsdk.Failure("fs", fmt.Sprintf("hash calculation failed: file not readable")), nil
		}
	}

	return regletsdk.Success(result), nil
}
