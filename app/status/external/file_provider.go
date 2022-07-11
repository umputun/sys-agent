package external

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// FileProvider is a status provider that checks the status of a file.
type FileProvider struct {
	TimeOut     time.Duration
	lastHandled int
}

// Status returns the status of the file
// url looks like this: file://blah/foo.txt (relative path) or file:///blah/foo.txt (absolute path)
func (f *FileProvider) Status(req Request) (*Response, error) {
	st := time.Now()

	fname := strings.TrimPrefix(req.URL, "file://")
	fi, err := os.Stat(fname)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("file stat failed: %s %s: %w", req.Name, fname, err)
	}

	if err != nil {
		result := Response{
			Name:         req.Name,
			StatusCode:   200,
			Body:         map[string]interface{}{"status": "not found"},
			ResponseTime: time.Since(st).Milliseconds(),
		}
		return &result, nil
	}

	body := map[string]interface{}{}
	body["status"] = "found"
	body["size"] = fi.Size()
	body["modif_time"] = fi.ModTime().Format(time.RFC3339Nano)
	body["since_modif"] = time.Since(fi.ModTime()).Milliseconds()

	fh, err := os.Open(fname)
	if err != nil {
		return nil, fmt.Errorf("file open failed: %s %s: %w", req.Name, fname, err)
	}
	defer fh.Close()
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
