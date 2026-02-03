package external

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// FileProvider is a status provider that checks the status of a file.
type FileProvider struct {
	TimeOut time.Duration

	lastInfo struct {
		files map[string]os.FileInfo
		once  sync.Once
		lock  sync.Mutex
	}
}

// Status returns the status of the file
// url looks like this: file://blah/foo.txt (relative path) or file:///blah/foo.txt (absolute path)
func (f *FileProvider) Status(req Request) (*Response, error) {
	f.lastInfo.once.Do(func() {
		f.lastInfo.files = make(map[string]os.FileInfo)
	})

	st := time.Now()

	fname := strings.TrimPrefix(req.URL, "file://")
	fi, err := os.Stat(fname)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("file stat failed: %s %s: %w", req.Name, fname, err)
	}

	defer func() { // set file info
		f.lastInfo.lock.Lock()
		defer f.lastInfo.lock.Unlock()
		f.lastInfo.files[fname] = fi
	}()

	if err != nil {
		result := Response{
			Name:         req.Name,
			StatusCode:   200,
			Body:         map[string]any{"status": "not found"},
			ResponseTime: time.Since(st).Milliseconds(),
		}
		return &result, nil
	}

	body := map[string]any{}
	body["status"] = "found"
	body["size"] = fi.Size()
	body["modif_time"] = fi.ModTime().Format(time.RFC3339Nano)
	body["since_modif"] = time.Since(fi.ModTime()).Milliseconds()
	body["size_change"] = fi.Size() // default to file size, if this was the first time we checked
	body["modif_change"] = int64(0) // default to 0, if this was the first time we checked

	f.lastInfo.lock.Lock()
	if last, ok := f.lastInfo.files[fname]; ok {
		body["size_change"] = fi.Size() - last.Size()
		body["modif_change"] = fi.ModTime().Sub(last.ModTime()).Milliseconds()
	}
	f.lastInfo.lock.Unlock()

	fh, err := os.Open(fname) //nolint:gosec // open file for reading, this is trusted file from the provider config
	if err != nil {
		return nil, fmt.Errorf("file open failed: %s %s: %w", req.Name, fname, err)
	}
	defer fh.Close() //nolint:gosec // ro file

	data := make([]byte, 100)
	n, err := fh.Read(data)
	if err != nil {
		return nil, fmt.Errorf("file read failed: %s %s: %w", req.Name, fname, err)
	}
	body["content"] = string(data[:n])

	result := Response{
		Name:         req.Name,
		StatusCode:   200,
		Body:         body,
		ResponseTime: time.Since(st).Milliseconds(),
	}
	return &result, nil

}
