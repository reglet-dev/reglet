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
	"github.com/reglet-dev/reglet/plugins/file/core"
)

// FileService provides file system checks.
type FileService struct {
	plugin.Service `name:"file" desc:"File system checks"`

	Check plugin.Op[CheckInput, CheckOutput] `desc:"Check file properties" method:"CheckHandler"`
}

func init() {
	plugin.RegisterOp[CheckInput, CheckOutput]("Check",
		plugin.Example[CheckInput, CheckOutput]{
			Name:        "check_exists",
			Description: "Check if /etc/hosts exists",
			Input:       CheckInput{Path: "/etc/hosts", Operation: "exists"},
			ExpectedOutput: &CheckOutput{
				Path:   "/etc/hosts",
				Exists: true,
			},
		},
		plugin.Example[CheckInput, CheckOutput]{
			Name:        "check_content",
			Description: "Check if file contains 'localhost'",
			Input:       CheckInput{Path: "/etc/hosts", Operation: "content", Contains: "localhost"},
			ExpectedOutput: &CheckOutput{
				Path:     "/etc/hosts",
				Exists:   true,
				Contains: true,
			},
		},
	)

	plugin.MustRegisterService(core.Plugin, &FileService{})
}

func (s *FileService) CheckHandler(ctx context.Context, in *CheckInput) (*CheckOutput, error) {
	// 1. Open/Stat
	f, info, err := openAndStat(in.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return &CheckOutput{
				Path:   in.Path,
				Exists: false,
			}, nil
		}
		return nil, fmt.Errorf("fs error: %w", err)
	}
	defer func() {
		if f != nil {
			f.Close()
		}
	}()

	// Populate basic metadata
	output := &CheckOutput{
		Path:        in.Path,
		Exists:      true,
		IsDir:       info.IsDir(),
		Size:        info.Size(),
		Permissions: fmt.Sprintf("%04o", info.Mode().Perm()),
		ModTime:     info.ModTime().Format(time.RFC3339),
	}

	// Logic varies by operation
	switch in.Operation {
	case "exists":
		// Already checked existence
		return output, nil

	case "permissions":
		if in.Permissions != "" {
			if output.Permissions != in.Permissions {
				// Mismatch logic: return failure error or just data?
				// To match typed op pattern, we return data.
				// Validation should happen in Reglet Policy usually, but legacy returned error.
				// However, new pattern prefers returning data for expression evaluation.
				// So we return the ACTUAL permissions.
				// The legacy handler returned ResultFailure.
				// With typed ops + reglet profile migration, the check "data.permissions == '0644'" handles validation.
				// So returning actual is better.
			}
		}
		return output, nil

	case "checksum":
		if info.IsDir() {
			return nil, fmt.Errorf("cannot calculate checksum of a directory")
		}
		if in.Algorithm == "" || in.Algorithm == "sha256" {
			hash, errStr := calculateHash(f)
			if errStr != "" {
				return nil, fmt.Errorf("hashing failed: %s", errStr)
			}
			output.Checksum = hash
		}
		return output, nil

	case "content":
		if info.IsDir() {
			return nil, fmt.Errorf("cannot read content of a directory")
		}
		if in.Contains != "" {
			content, errStr := readContent(f)
			if errStr != "" {
				return nil, fmt.Errorf("read failed: %s", errStr)
			}
			output.Contains = strings.Contains(content, in.Contains)
		}
		return output, nil

	default:
		return nil, fmt.Errorf("unknown operation: %s", in.Operation)
	}
}

// Helpers

func openAndStat(path string) (*os.File, os.FileInfo, error) {
	// Implementation matches original
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

// func populateMetadata(result map[string]interface{}, info os.FileInfo) {
// 	result["exists"] = true
// 	result["is_dir"] = info.IsDir()
// 	result["size"] = info.Size()
// 	result["mode"] = fmt.Sprintf("%04o", info.Mode().Perm())
// 	result["permissions"] = info.Mode().String()
// 	result["mod_time"] = info.ModTime().Format(time.RFC3339)

// 	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
// 		result["uid"] = stat.Uid
// 		result["gid"] = stat.Gid
// 	}
// }

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

func (s *FileService) getSysInfo(info os.FileInfo, output *CheckOutput) {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		// Just noting we might want uid/gid in output if schema supports it.
		// Current CheckOutput schema doesn't have Uid/Gid.
		_ = stat
	}
}
