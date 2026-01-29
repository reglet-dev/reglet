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

	"github.com/reglet-dev/reglet-sdk/go/application/config"
	"github.com/reglet-dev/reglet-sdk/go/application/schema"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
)

// filePlugin implements the sdk.Plugin interface for file system operations.
type filePlugin struct{}

// Describe provides the file plugin's metadata and capabilities.
func (p *filePlugin) Describe(ctx context.Context) (entities.Metadata, error) {
	return entities.Metadata{
		Name:        "file",
		Version:     "1.1.0",
		Description: "File existence, content, and hash checks",
		Capabilities: []entities.Capability{
			{
				Category: "fs",
				Resource: "read:**",
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
	return schema.GenerateSchema(FileConfig{})
}

// Check executes file system validation based on the provided configuration.
func (p *filePlugin) Check(ctx context.Context, cfgRaw config.Config) (entities.Result, error) {
	var cfg FileConfig
	if err := config.Validate(cfgRaw, &cfg); err != nil {
		return entities.ResultError(entities.NewErrorDetail("config", err.Error())), nil
	}
	if cfg.Path == "" {
		return entities.ResultError(entities.NewErrorDetail("config", "path is required")), nil
	}
	return checkFile(cfg)
}

// checkFile performs the actual file check logic.
func checkFile(cfg FileConfig) (entities.Result, error) {
	resultData := map[string]interface{}{
		"path": cfg.Path,
	}

	start := time.Now()
	// Meta helper to wrap returns
	makeResult := func(status bool, errDetail *entities.ErrorDetail) (entities.Result, error) {
		metadata := entities.NewRunMetadata(start, time.Now())
		if errDetail != nil {
			if status {
				// Should not happen: success with error? assume failure logic intended
				return entities.ResultFailure(errDetail.Message, resultData).WithMetadata(metadata), nil
			}
			// Failure or Error
			if errDetail.Type == "fs" {
				// Treat FS errors (permission, not found) as failures in context of check?
				// Actually "not found" is mostly success=false in evidence terms, i.e. ResultFailure.
				// But let's stick to ResultFailure for operational checks that return false.
				// Wait, if file doesn't exist, is it an error or just result["exists"]=false?
				// Logic below sets result["exists"]=false.
				// If we return ResultError, it means "check failed to execute".
				// If we return ResultSuccess with data["exists"]=false, that means check executed and found file missing.
				// The original code returned Success(result) for "Not Exist".
				return entities.ResultSuccess("File check completed", resultData).WithMetadata(metadata), nil
			}
			return entities.ResultError(errDetail).WithMetadata(metadata), nil
		}
		return entities.ResultSuccess("File check successful", resultData).WithMetadata(metadata), nil
	}

	_ = makeResult // suppress unused if I change logic

	// 1. Open file and get metadata
	f, info, err := openAndStat(cfg.Path)
	if err != nil {
		// handleOpenError logic
		if os.IsNotExist(err) {
			resultData["exists"] = false
			resultData["readable"] = false
			return entities.ResultSuccess("File does not exist", resultData).WithMetadata(entities.NewRunMetadata(start, time.Now())), nil
		}
		// True error
		return entities.ResultError(entities.NewErrorDetail("fs", err.Error())).WithMetadata(entities.NewRunMetadata(start, time.Now())), nil
	}

	if f != nil {
		defer f.Close()
		resultData["readable"] = true
	} else {
		resultData["readable"] = false
	}

	// 2. Populate metadata
	populateMetadata(resultData, info)

	// 3. Check for symlink
	checkSymlink(resultData, cfg.Path)

	// 4. Read content if requested
	if cfg.ReadContent && !info.IsDir() {
		if errStr := readContent(f, resultData); errStr != "" {
			return entities.ResultFailure(errStr, resultData).WithMetadata(entities.NewRunMetadata(start, time.Now())), nil
		}
	}

	// 5. Calculate hash if requested
	if cfg.Hash && !info.IsDir() {
		if errStr := calculateHash(f, resultData); errStr != "" {
			return entities.ResultFailure(errStr, resultData).WithMetadata(entities.NewRunMetadata(start, time.Now())), nil
		}
	}

	return entities.ResultSuccess("File check successful", resultData).WithMetadata(entities.NewRunMetadata(start, time.Now())), nil
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

// readContent reads file content into result. Returns error string if failed, empty otherwise.
func readContent(f *os.File, result map[string]interface{}) string {
	if f == nil {
		return "read failed: file not readable"
	}

	if _, err := f.Seek(0, 0); err != nil {
		return fmt.Sprintf("seek failed: %v", err)
	}

	content, err := io.ReadAll(f)
	if err != nil {
		return fmt.Sprintf("read failed: %v", err)
	}

	result["content_b64"] = base64.StdEncoding.EncodeToString(content)
	result["encoding"] = "base64"
	return ""
}

// calculateHash calculates SHA256 hash of file content. Returns error string if failed, empty otherwise.
func calculateHash(f *os.File, result map[string]interface{}) string {
	if f == nil {
		return "hash calculation failed: file not readable"
	}

	if _, err := f.Seek(0, 0); err != nil {
		return fmt.Sprintf("seek for hash failed: %v", err)
	}

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return fmt.Sprintf("hash calculation failed: %v", err)
	}

	result["sha256"] = hex.EncodeToString(hasher.Sum(nil))
	return ""
}
