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

	// 1. Check Existence & Metadata
	info, err := os.Stat(cfg.Path)
	if err != nil {
		if os.IsNotExist(err) {
			result["exists"] = false
			return regletsdk.Success(result), nil
		}
		return regletsdk.Failure("fs", fmt.Sprintf("stat failed: %v", err)), nil
	}

	result["exists"] = true
	result["is_dir"] = info.IsDir()
	result["size"] = info.Size()
	result["mode"] = fmt.Sprintf("%04o", info.Mode().Perm())
	result["permissions"] = info.Mode().String()
	result["mod_time"] = info.ModTime().Format(time.RFC3339)

	// Check if file is readable (not a directory)
	if !info.IsDir() {
		f, err := os.Open(cfg.Path)
		if err == nil {
			result["readable"] = true
			f.Close()
		} else {
			result["readable"] = false
		}
	} else {
		// Directories are considered "readable" if we could stat them
		result["readable"] = true
	}

	// Attempt to get ownership
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		result["uid"] = stat.Uid
		result["gid"] = stat.Gid
	}

	// Check for symlink
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
		content, err := os.ReadFile(cfg.Path)
		if err != nil {
			return regletsdk.Failure("fs", fmt.Sprintf("read failed: %v", err)), nil
		}
		result["content_b64"] = base64.StdEncoding.EncodeToString(content)
		result["encoding"] = "base64"
		// We could add plain "content" string if utf8, but b64 is safer for generic use
	}

	// 3. Calculate Hash (if requested and not a directory)
	if cfg.Hash && !info.IsDir() {
		f, err := os.Open(cfg.Path)
		if err != nil {
			return regletsdk.Failure("fs", fmt.Sprintf("open for hash failed: %v", err)), nil
		}
		defer f.Close()

		hasher := sha256.New()
		if _, err := io.Copy(hasher, f); err != nil {
			return regletsdk.Failure("fs", fmt.Sprintf("hash calculation failed: %v", err)), nil
		}
		result["sha256"] = hex.EncodeToString(hasher.Sum(nil))
	}

	return regletsdk.Success(result), nil
}
