// Package fileutils provides useful, high-level file operations
package fileutils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// CopyFile copies a file from source to dest. Any existing file will be overwritten
// and attributes will not be copied
func CopyFile(src string, dst string) error {

	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("can't stat %s: %w", src, err)
	}

	if !srcInfo.Mode().IsRegular() {
		return fmt.Errorf("can't copy non-regular source file %s (%s)", src, srcInfo.Mode().String())
	}

	srcFh, err := os.Open(src) //nolint
	if err != nil {
		return fmt.Errorf("can't open source file %s: %w", src, err)
	}
	defer srcFh.Close()

	err = os.MkdirAll(filepath.Dir(dst), 0750)
	if err != nil {
		return fmt.Errorf("can't make destination directory %s: %w", filepath.Dir(dst), err)
	}

	dstFh, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("can't create destination file %s: %w", dst, err)
	}
	defer dstFh.Close()

	if _, err = io.Copy(dstFh, srcFh); err != nil {
		return fmt.Errorf("can't copy data: %w", err)
	}
	return dstFh.Sync()
}

// CopyDir copies all files from src to dst, recursively
func CopyDir(src string, dst string) error {
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
