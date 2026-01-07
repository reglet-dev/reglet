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

	return checkFile(cfg)
}

// checkFile performs the actual file check logic.
func checkFile(cfg FileConfig) (regletsdk.Evidence, error) {
	result := map[string]interface{}{
		"path": cfg.Path,
	}

	// 1. Open file and get metadata
	f, info, err := openAndStat(cfg.Path)
	if err != nil {
		return handleOpenError(err, cfg.Path, result)
	}
	if f != nil {
		defer f.Close()
		result["readable"] = true
	} else {
		result["readable"] = false
	}

	// 2. Populate metadata
	populateMetadata(result, info)

	// 3. Check for symlink
	checkSymlink(result, cfg.Path)

	// 4. Read content if requested
	if cfg.ReadContent && !info.IsDir() {
		if err := readContent(f, result); err != nil {
			return err.(regletsdk.Evidence), nil
		}
	}

	// 5. Calculate hash if requested
	if cfg.Hash && !info.IsDir() {
		if err := calculateHash(f, result); err != nil {
			return err.(regletsdk.Evidence), nil
		}
	}

	return regletsdk.Success(result), nil
}

// openAndStat attempts to open the file and get its metadata.
// Returns (file, info, error). file may be nil if unreadable but exists.
func openAndStat(path string) (*os.File, os.FileInfo, error) {
	f, openErr := os.Open(path)
	if openErr == nil {
		// Successfully opened - get stats from file handle (atomic)
		info, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, nil, fmt.Errorf("stat on open file failed: %w", err)
		}
		return f, info, nil
	}

	// Open failed - try stat to check if file exists but is unreadable
	info, statErr := os.Stat(path)
	if statErr != nil {
		if os.IsNotExist(statErr) || os.IsNotExist(openErr) {
			return nil, nil, os.ErrNotExist
		}
		return nil, nil, fmt.Errorf("stat failed: %w", statErr)
	}

	// File exists but is unreadable
	return nil, info, nil
}

// handleOpenError handles errors from openAndStat.
func handleOpenError(err error, path string, result map[string]interface{}) (regletsdk.Evidence, error) {
	if os.IsNotExist(err) {
		result["exists"] = false
		result["readable"] = false
		return regletsdk.Success(result), nil
	}
	return regletsdk.Failure("fs", err.Error()), nil
}

// populateMetadata fills in file metadata fields.
func populateMetadata(result map[string]interface{}, info os.FileInfo) {
	result["exists"] = true
	result["is_dir"] = info.IsDir()
	result["size"] = info.Size()
	result["mode"] = fmt.Sprintf("%04o", info.Mode().Perm())
	result["permissions"] = info.Mode().String()
	result["mod_time"] = info.ModTime().Format(time.RFC3339)

	// Attempt to get ownership (Unix-specific)
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		result["uid"] = stat.Uid
		result["gid"] = stat.Gid
	}
}

// checkSymlink checks if the path is a symlink and populates result.
func checkSymlink(result map[string]interface{}, path string) {
	linfo, err := os.Lstat(path)
	if err != nil {
		result["is_symlink"] = false
		return
	}

	if linfo.Mode()&os.ModeSymlink != 0 {
		result["is_symlink"] = true
		if target, err := os.Readlink(path); err == nil {
			result["symlink_target"] = target
		}
	} else {
		result["is_symlink"] = false
	}
}

// readContent reads file content into result. Returns Evidence on error.
func readContent(f *os.File, result map[string]interface{}) interface{} {
	if f == nil {
		return regletsdk.Failure("fs", "read failed: file not readable")
	}

	if _, err := f.Seek(0, 0); err != nil {
		return regletsdk.Failure("fs", fmt.Sprintf("seek failed: %v", err))
	}

	content, err := io.ReadAll(f)
	if err != nil {
		return regletsdk.Failure("fs", fmt.Sprintf("read failed: %v", err))
	}

	result["content_b64"] = base64.StdEncoding.EncodeToString(content)
	result["encoding"] = "base64"
	return nil
}

// calculateHash calculates SHA256 hash of file content. Returns Evidence on error.
func calculateHash(f *os.File, result map[string]interface{}) interface{} {
	if f == nil {
		return regletsdk.Failure("fs", "hash calculation failed: file not readable")
	}

	if _, err := f.Seek(0, 0); err != nil {
		return regletsdk.Failure("fs", fmt.Sprintf("seek for hash failed: %v", err))
	}

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return regletsdk.Failure("fs", fmt.Sprintf("hash calculation failed: %v", err))
	}

	result["sha256"] = hex.EncodeToString(hasher.Sum(nil))
	return nil
}
