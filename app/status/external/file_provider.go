package external

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"text/template"
	"time"
)

// FileProvider is a status provider that checks the status of a file.
type FileProvider struct {
	TimeOut time.Duration

	now      func() time.Time
	lastInfo struct {
		files map[string]fileMatch
		once  sync.Once
		lock  sync.Mutex
	}
}

type fileDateFields struct {
	YYYY     string
	YY       string
	MM       string
	DD       string
	YYYYMMDD string
	YYYYMM   string
	YYMMDD   string
}

type fileMatch struct {
	path       string
	info       os.FileInfo
	matchCount int
}

// Status returns the status of the file
// url looks like this: file://blah/foo.txt (relative path) or file:///blah/foo.txt (absolute path)
func (f *FileProvider) Status(req Request) (*Response, error) {
	f.lastInfo.once.Do(func() {
		f.lastInfo.files = make(map[string]fileMatch)
	})

	st := time.Now()
	target, _, err := parseTarget(req.URL, "file")
	if err != nil {
		return nil, fmt.Errorf("file URL parse failed: %s %s: %w", req.Name, req.URL, err)
	}
	match, err := f.resolveTarget(target)
	if err != nil {
		return nil, fmt.Errorf("file target resolution failed: %s %s: %w", req.Name, target, err)
	}
	if match.info == nil {
		return &Response{
			Name:         req.Name,
			StatusCode:   200,
			Body:         map[string]any{"status": "not found", "match_count": match.matchCount},
			ResponseTime: time.Since(st).Milliseconds(),
		}, nil
	}

	fi := match.info
	body := map[string]any{
		"status":       "found",
		"path":         match.path,
		"match_count":  match.matchCount,
		"size":         fi.Size(),
		"modif_time":   fi.ModTime().Format(time.RFC3339Nano),
		"since_modif":  time.Since(fi.ModTime()).Milliseconds(),
		"size_change":  fi.Size(),
		"modif_change": int64(0),
		"content":      "",
	}

	f.lastInfo.lock.Lock()
	last := f.lastInfo.files[target]
	f.lastInfo.lock.Unlock()
	if last.info != nil {
		body["size_change"] = fi.Size() - last.info.Size()
		body["modif_change"] = fi.ModTime().Sub(last.info.ModTime()).Milliseconds()
	}

	if !fi.IsDir() {
		fh, err := os.Open(match.path) //nolint:gosec // open file for reading, this is trusted file from the provider config
		if err != nil {
			return nil, fmt.Errorf("file open failed: %s %s: %w", req.Name, match.path, err)
		}
		defer fh.Close() //nolint:gosec // ro file

		data := make([]byte, 100)
		n, readErr := fh.Read(data)
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return nil, fmt.Errorf("file read failed: %s %s: %w", req.Name, match.path, readErr)
		}
		body["content"] = string(data[:n])
	}

	f.lastInfo.lock.Lock()
	f.lastInfo.files[target] = fileMatch{path: match.path, info: fi}
	f.lastInfo.lock.Unlock()

	return &Response{
		Name:         req.Name,
		StatusCode:   200,
		Body:         body,
		ResponseTime: time.Since(st).Milliseconds(),
	}, nil
}

func (f *FileProvider) clock() time.Time {
	if f.now != nil {
		return f.now()
	}
	return time.Now()
}

func newFileDateFields(ts time.Time) fileDateFields {
	year, month, day := ts.Date()
	midnight := time.Date(year, month, day, 0, 0, 0, 0, ts.Location())
	return fileDateFields{
		YYYY:     midnight.Format("2006"),
		YY:       midnight.Format("06"),
		MM:       midnight.Format("01"),
		DD:       midnight.Format("02"),
		YYYYMMDD: midnight.Format("20060102"),
		YYYYMM:   midnight.Format("200601"),
		YYMMDD:   midnight.Format("060102"),
	}
}

func (f *FileProvider) expandTarget(target string) (string, error) {
	tmpl, err := template.New("file-target").Delims("[[", "]]").Parse(target)
	if err != nil {
		return "", fmt.Errorf("parse file target template: %w", err)
	}
	var expanded bytes.Buffer
	if err = tmpl.Execute(&expanded, newFileDateFields(f.clock())); err != nil {
		return "", fmt.Errorf("execute file target template: %w", err)
	}
	return expanded.String(), nil
}

func (f *FileProvider) resolveTarget(target string) (fileMatch, error) {
	expanded, err := f.expandTarget(target)
	if err != nil {
		return fileMatch{}, err
	}
	matches, err := filepath.Glob(expanded)
	if err != nil {
		return fileMatch{}, fmt.Errorf("glob file target: %w", err)
	}

	result := fileMatch{}
	for _, path := range matches {
		fi, statErr := os.Stat(path)
		if errors.Is(statErr, os.ErrNotExist) {
			continue
		}
		if statErr != nil {
			return fileMatch{}, fmt.Errorf("file stat failed: %s: %w", path, statErr)
		}
		result.matchCount++
		if result.info == nil || fi.ModTime().After(result.info.ModTime()) ||
			(fi.ModTime().Equal(result.info.ModTime()) && path < result.path) {
			result.path = path
			result.info = fi
		}
	}
	return result, nil
}
