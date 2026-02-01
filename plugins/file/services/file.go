package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/reglet-dev/reglet-sdk/go/application/plugin"
	"github.com/reglet-dev/reglet-sdk/go/domain/entities"
	"github.com/reglet-dev/reglet/plugins/file/core"
)

// FileService provides file system checks.
type FileService struct {
	plugin.Service `name:"file" desc:"File system checks"`

	Exists      plugin.Op `desc:"Check if file or directory exists" method:"ExistsHandler"`
	Permissions plugin.Op `desc:"Verify file permissions match expected" method:"PermissionsHandler"`
	Checksum    plugin.Op `desc:"Validate file checksum" method:"ChecksumHandler"`
	Content     plugin.Op `desc:"Check if file contains expected content" method:"ContentHandler"`
}

func init() {
	plugin.MustRegisterService(core.Plugin, &FileService{})
}

// Handlers

func (s *FileService) ExistsHandler(ctx context.Context, req *plugin.Request) (*entities.Result, error) {
	cfg := req.Config.(*core.FileConfig)
	return checkFile(cfg, "exists")
}

func (s *FileService) PermissionsHandler(ctx context.Context, req *plugin.Request) (*entities.Result, error) {
	cfg := req.Config.(*core.FileConfig)
	return checkFile(cfg, "permissions")
}

func (s *FileService) ChecksumHandler(ctx context.Context, req *plugin.Request) (*entities.Result, error) {
	cfg := req.Config.(*core.FileConfig)
	return checkFile(cfg, "checksum")
}

func (s *FileService) ContentHandler(ctx context.Context, req *plugin.Request) (*entities.Result, error) {
	cfg := req.Config.(*core.FileConfig)
	return checkFile(cfg, "content")
}

// checkFile performs the check based on the operation mode.
// This is adapted from the legacy plugin logic.
func checkFile(cfg *core.FileConfig, opName string) (*entities.Result, error) {
	resultData := map[string]interface{}{
		"path":      cfg.Path,
		"operation": opName,
	}

	// start := time.Now() // SDK usually handles timing

	// 1. Open/Stat
	f, info, err := openAndStat(cfg.Path)
	if err != nil {
		// If checking existence and file not found, it's a success=false result for Exists op?
		// Or if we specifically expect it to exist (default assumption in checks).
		if os.IsNotExist(err) {
			resultData["exists"] = false
			return entities.ResultFailurePtr("File does not exist", resultData), nil
		}
		return entities.ResultErrorPtr("fs", err.Error()), nil
	}
	defer func() {
		if f != nil {
			f.Close()
		}
	}()

	// Populate basic metadata
	populateMetadata(resultData, info)

	// Validate based on Op
	switch opName {
	case "exists":
		// Just existence check passed
		return entities.ResultSuccessPtr(fmt.Sprintf("File exists: %s", cfg.Path), resultData), nil

	case "permissions":
		if cfg.Permissions != "" {
			currentPerms := fmt.Sprintf("%04o", info.Mode().Perm())
			if currentPerms != cfg.Permissions {
				return entities.ResultFailurePtr(
					fmt.Sprintf("Permissions mismatch: got %s, want %s", currentPerms, cfg.Permissions),
					resultData,
				), nil
			}
		}
		return entities.ResultSuccessPtr(fmt.Sprintf("Permissions match: %s", cfg.Permissions), resultData), nil

	case "checksum":
		if info.IsDir() {
			return entities.ResultFailurePtr("Cannot calculate checksum of a directory", resultData), nil
		}
		if cfg.Checksum != "" {
			hash, errStr := calculateHash(f)
			if errStr != "" {
				return entities.ResultErrorPtr("fs", errStr), nil
			}
			resultData["sha256"] = hash
			if hash != cfg.Checksum {
				return entities.ResultFailurePtr(
					fmt.Sprintf("Checksum mismatch: got %s, want %s", hash, cfg.Checksum),
					resultData,
				), nil
			}
			return entities.ResultSuccessPtr("Checksum verified", resultData), nil
		}
		return entities.ResultSuccessPtr("Checksum check skipped (no expected value)", resultData), nil

	case "content":
		if info.IsDir() {
			return entities.ResultFailurePtr("Cannot read content of a directory", resultData), nil
		}
		if cfg.Contains != "" {
			content, errStr := readContent(f)
			if errStr != "" {
				return entities.ResultErrorPtr("fs", errStr), nil
			}
			// Don't put full content in resultData usually, maybe too big.
			// Legacy put b64 content. We can do that if needed or just verify.
			// Let's optimize: checking "Contains".
			if !strings.Contains(content, cfg.Contains) {
				return entities.ResultFailurePtr(
					fmt.Sprintf("Content does not contain expected string: %q", cfg.Contains),
					resultData,
				), nil
			}
			return entities.ResultSuccessPtr("Content verification successful", resultData), nil
		}
		return entities.ResultSuccessPtr("Content check skipped (no expectation)", resultData), nil
	}

	return entities.ResultErrorPtr("configuration", "Unknown operation"), nil
}

// Helpers reused/adapted from legacy

func openAndStat(path string) (*os.File, os.FileInfo, error) {
	f, openErr := os.Open(path)
	if openErr == nil {
		info, err := f.Stat()
		if err != nil {
			f.Close()
			return nil, nil, fmt.Errorf("stat on open file failed: %w", err)
		}
		return f, info, nil
	}

	info, statErr := os.Stat(path)
	if statErr != nil {
		if os.IsNotExist(statErr) || os.IsNotExist(openErr) {
			return nil, nil, os.ErrNotExist
		}
		return nil, nil, fmt.Errorf("stat failed: %w", statErr)
	}
	return nil, info, nil
}

func populateMetadata(result map[string]interface{}, info os.FileInfo) {
	result["exists"] = true
	result["is_dir"] = info.IsDir()
	result["size"] = info.Size()
	result["mode"] = fmt.Sprintf("%04o", info.Mode().Perm())
	result["permissions"] = info.Mode().String()
	result["mod_time"] = info.ModTime().Format(time.RFC3339)

	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		result["uid"] = stat.Uid
		result["gid"] = stat.Gid
	}
}

func readContent(f *os.File) (string, string) {
	if f == nil {
		return "", "read failed: file not readable"
	}
	if _, err := f.Seek(0, 0); err != nil {
		return "", fmt.Sprintf("seek failed: %v", err)
	}
	content, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Sprintf("read failed: %v", err)
	}
	return string(content), ""
}

func calculateHash(f *os.File) (string, string) {
	if f == nil {
		return "", "hash calculation failed: file not readable"
	}
	if _, err := f.Seek(0, 0); err != nil {
		return "", fmt.Sprintf("seek for hash failed: %v", err)
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", fmt.Sprintf("hash calculation failed: %v", err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), ""
}
