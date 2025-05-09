// Package fileutils provides useful, high-level file operations
package fileutils

import (
	//nolint:gosec // Needed for compatibility
	"crypto/md5"
	"crypto/rand"
	//nolint:gosec // Needed for compatibility
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-pkgz/fileutils/enum"
)

//go:generate enum -type=hashAlg -path=enum
//nolint:unused // This type is used by the enum generator
type hashAlg int

// These constants are used by the enum generator
//
//nolint:unused // These constants are used by the enum generator
const (
	hashAlgMD5 hashAlg = iota + 1
	hashAlgSHA1
	hashAlgSHA256
	hashAlgSHA224
	hashAlgSHA384
	hashAlgSHA512
	hashAlgSHA512_224
	hashAlgSHA512_256
)

// IsFile returns true if filename exists
func IsFile(filename string) bool {
	return exists(filename, false)
}

// IsDir returns true if directory exists
func IsDir(dirname string) bool {
	return exists(dirname, true)
}

func exists(name string, dir bool) bool {
	info, err := os.Stat(name)
	if os.IsNotExist(err) {
		return false
	}
	if dir {
		return info.IsDir()
	}
	return !info.IsDir()
}

// CopyFile copies a file from source to dest, preserving mode.
// Any existing file will be overwritten.
func CopyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("can't stat %s: %w", src, err)
	}

	if !srcInfo.Mode().IsRegular() {
		return fmt.Errorf("can't copy non-regular source file %s (%s)", src, srcInfo.Mode().String())
	}

	srcFh, err := os.Open(src) //nolint:gosec // file path is provided by the caller
	if err != nil {
		return fmt.Errorf("can't open source file %s: %w", src, err)
	}
	defer func() { _ = srcFh.Close() }()

	err = os.MkdirAll(filepath.Dir(dst), 0o750)
	if err != nil {
		return fmt.Errorf("can't make destination directory %s: %w", filepath.Dir(dst), err)
	}

	dstFh, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode()) //nolint:gosec // file path is provided by the caller
	if err != nil {
		return fmt.Errorf("can't create destination file %s: %w", dst, err)
	}
	defer func() { _ = dstFh.Close() }()

	size, err := io.Copy(dstFh, srcFh)
	if err != nil {
		return fmt.Errorf("can't copy data: %w", err)
	}
	if size != srcInfo.Size() {
		return fmt.Errorf("incomplete copy, %d of %d", size, srcInfo.Size())
	}

	return dstFh.Sync()
}

// CopyDir copies all files from src to dst, recursively
func CopyDir(src, dst string) error {
	list, err := ListFiles(src)
	if err != nil {
		return fmt.Errorf("can't list source files in %s: %w", src, err)
	}
	for _, srcFile := range list {
		stripSrcDir := strings.TrimPrefix(srcFile, src)
		dstFile := filepath.Join(dst, stripSrcDir)
		if err = CopyFile(srcFile, dstFile); err != nil {
			return fmt.Errorf("can't copy %s to %s: %w", srcFile, dstFile, err)
		}
	}
	return nil
}

// ListFiles gets recursive list of all files in a directory
func ListFiles(directory string) (list []string, err error) {
	err = filepath.Walk(directory, func(path string, info os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		if info.IsDir() {
			return nil
		}
		list = append(list, path)
		return nil
	})
	sort.Slice(list, func(i, j int) bool {
		return list[i] < list[j]
	})
	return list, err
}

// TempFileName returns a new temporary file name in the directory dir.
// The filename is generated by taking pattern and adding a random
// string to the end. If pattern includes a "*", the random string
// replaces the last "*".
// If dir is the empty string, TempFileName uses the default directory
// for temporary files (see os.TempDir).
// Multiple programs calling TempFileName simultaneously
// will not choose the same file name.
func TempFileName(dir, pattern string) (string, error) {
	if dir == "" {
		dir = os.TempDir()
	}

	// prefixAndSuffix splits pattern by the last wildcard "*", if applicable
	prefix, suffix := pattern, ""
	if pos := strings.LastIndex(pattern, "*"); pos != -1 {
		prefix, suffix = pattern[:pos], pattern[pos+1:]
	}

	// try to generate unique name
	const maxTries = 10000
	const randomBytes = 16 // 32 hex chars

	for i := 0; i < maxTries; i++ {
		// generate random bytes
		b := make([]byte, randomBytes)
		if _, err := rand.Read(b); err != nil {
			return "", fmt.Errorf("failed to generate random name: %w", err)
		}

		// create file name and check if it exists
		name := filepath.Join(dir, prefix+hex.EncodeToString(b)+suffix)
		if _, err := os.Stat(name); os.IsNotExist(err) {
			return name, nil
		}
	}

	return "", errors.New("failed to create temporary file name after multiple attempts")
}

var reInvalidPathChars = regexp.MustCompile(`[<>:"|?*]+`) // invalid path characters
const maxPathLength = 1024                                // maximum length for path

// SanitizePath returns a sanitized version of the given path.
func SanitizePath(s string) string {
	s = strings.TrimSpace(s)
	s = reInvalidPathChars.ReplaceAllString(filepath.Clean(s), "_")

	// normalize path separators to '/'
	s = strings.ReplaceAll(s, `\`, "/")

	if len(s) > maxPathLength {
		s = s[:maxPathLength]
	}

	return s
}

// MoveFile moves a file from src to dst.
// If rename fails (e.g., cross-device move), it will fall back to copy+delete.
// It will create destination directories if they don't exist.
func MoveFile(src, dst string) error {
	if src == "" {
		return errors.New("empty source path")
	}
	if dst == "" {
		return errors.New("empty destination path")
	}

	// check if source exists
	srcInfo, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("source file not found: %s", src)
		}
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	// ensure source is a regular file
	if !srcInfo.Mode().IsRegular() {
		return fmt.Errorf("source is not a regular file: %s", src)
	}

	// try atomic rename first
	if err = os.Rename(src, dst); err == nil {
		return nil
	}

	// create destination directory if needed
	if err = os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// try rename again after creating directory
	if err = os.Rename(src, dst); err == nil {
		return nil
	}

	// fallback to copy+delete if rename fails
	if err = CopyFile(src, dst); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// verify the copy succeeded and sizes match
	dstInfo, err := os.Stat(dst)
	if err != nil {
		return fmt.Errorf("failed to stat destination file: %w", err)
	}
	if srcInfo.Size() != dstInfo.Size() {
		return fmt.Errorf("size mismatch after copy: source %d, destination %d", srcInfo.Size(), dstInfo.Size())
	}

	// remove the source file
	if err := os.Remove(src); err != nil {
		return fmt.Errorf("failed to remove source file: %w", err)
	}

	return nil
}

// TouchFile creates an empty file if it doesn't exist,
// or updates access and modification times if it does.
func TouchFile(path string) error {
	if path == "" {
		return errors.New("empty path")
	}

	// try to get file info
	_, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to stat file: %w", err)
		}
		// create empty file with default mode
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644) //nolint:gosec // intentionally permissive
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("failed to close file: %w", err)
		}
		return nil
	}

	// file exists, update timestamps
	now := time.Now()
	return os.Chtimes(path, now, now)
}

// Checksum calculates the checksum of a file using the specified hash algorithm.
// Supported algorithms are MD5, SHA1, SHA224, SHA256, SHA384, SHA512, SHA512_224, and SHA512_256.
func Checksum(path string, algo enum.HashAlg) (string, error) {
	if path == "" {
		return "", errors.New("empty path")
	}

	var h hash.Hash
	switch algo {
	case enum.HashAlgMD5:
		h = md5.New() //nolint:gosec // needed for compatibility
	case enum.HashAlgSHA1:
		h = sha1.New() //nolint:gosec // needed for compatibility
	case enum.HashAlgSHA256:
		h = sha256.New()
	case enum.HashAlgSHA224:
		h = sha256.New224()
	case enum.HashAlgSHA384:
		h = sha512.New384()
	case enum.HashAlgSHA512:
		h = sha512.New()
	case enum.HashAlgSHA512_224:
		h = sha512.New512_224()
	case enum.HashAlgSHA512_256:
		h = sha512.New512_256()
	default:
		return "", fmt.Errorf("unsupported hash algorithm: %v", algo)
	}

	f, err := os.Open(path) //nolint:gosec // path is provided by the caller
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", path)
		}
		return "", fmt.Errorf("failed to open file %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to read file %s for hashing: %w", path, err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
