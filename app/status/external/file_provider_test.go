package external

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/go-pkgz/fileutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileProvider_Status(t *testing.T) {
	p := FileProvider{TimeOut: time.Second}
	tmpDir, err := os.MkdirTemp(os.TempDir(), "file_provider_test")
	require.NoError(t, err)
	fname := filepath.Join(tmpDir, "ping.txt")
	err = fileutils.CopyFile("testdata/ping.txt", fname)
	require.NoError(t, err)
	defer os.Remove(fname)

	time.Sleep(time.Millisecond * 101) // wait for file to be modified in 100ms
	{
		resp, e := p.Status(Request{Name: "r1", URL: "file://" + fname})
		require.NoError(t, e)
		t.Logf("%+v", resp)
		assert.Equal(t, "r1", resp.Name)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "pong", resp.Body["content"])
		assert.Equal(t, "found", resp.Body["status"])
		assert.Equal(t, int64(4), resp.Body["size"])
		assert.Greater(t, resp.Body["since_modif"].(int64), int64(100))
		assert.Equal(t, int64(4), resp.Body["size_change"])
		assert.Equal(t, int64(0), resp.Body["modif_change"])
	}

	{ // check size change, not changed
		resp, e := p.Status(Request{Name: "r1", URL: "file://" + fname})
		require.NoError(t, e)
		t.Logf("%+v", resp)
		assert.Equal(t, "r1", resp.Name)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "pong", resp.Body["content"])
		assert.Equal(t, "found", resp.Body["status"])
		assert.Equal(t, int64(4), resp.Body["size"])
		assert.Greater(t, resp.Body["since_modif"].(int64), int64(100))
		assert.Equal(t, int64(0), resp.Body["size_change"])
		assert.Equal(t, int64(0), resp.Body["modif_change"])
	}

	{ // check size change,  changed
		err = os.WriteFile(fname, []byte("pong 1234567890"), 0o600)
		require.NoError(t, err)
		resp, err := p.Status(Request{Name: "r1", URL: "file://" + fname})
		require.NoError(t, err)
		t.Logf("%+v", resp)
		assert.Equal(t, "r1", resp.Name)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "pong 1234567890", resp.Body["content"])
		assert.Equal(t, "found", resp.Body["status"])
		assert.Equal(t, int64(15), resp.Body["size"])
		assert.Less(t, resp.Body["since_modif"].(int64), int64(100))
		assert.Equal(t, int64(11), resp.Body["size_change"])
		assert.Greater(t, resp.Body["modif_change"].(int64), int64(100))
	}
	{
		resp, err := p.Status(Request{Name: "r1", URL: "file://testdata/bad.txt"})
		require.NoError(t, err)
		t.Logf("%+v", resp)
		assert.Equal(t, "not found", resp.Body["status"])
	}

	t.Run("cron query is not part of path", func(t *testing.T) {
		resp, err := p.Status(Request{Name: "r1", URL: "file://" + fname + "?cron=0_6_*_*_*"})
		require.NoError(t, err)
		assert.Equal(t, "found", resp.Body["status"])
		assert.Equal(t, "pong 1234567890", resp.Body["content"])
	})

	t.Run("relative path", func(t *testing.T) {
		resp, err := p.Status(Request{Name: "r1", URL: "file://testdata/ping.txt"})
		require.NoError(t, err)
		assert.Equal(t, "found", resp.Body["status"])
		assert.Equal(t, "pong", resp.Body["content"])
	})

	t.Run("invalid percent", func(t *testing.T) {
		_, err := p.Status(Request{Name: "bad", URL: "file:///tmp/50%_done.txt"})
		require.ErrorContains(t, err, "invalid URL escape")

		service := NewService(Providers{File: &p}, 1, "bad:file:///tmp/50%_done.txt")
		responses := service.Status()
		require.Len(t, responses, 1)
		assert.Equal(t, 500, responses[0].StatusCode)
		assert.Contains(t, responses[0].Body["error"], "invalid URL escape")
	})
}

func TestFileProvider_MissingThenPresent(t *testing.T) {
	p := FileProvider{TimeOut: time.Second}
	fname := filepath.Join(t.TempDir(), "appears.txt")

	resp, err := p.Status(Request{Name: "file", URL: "file://" + fname})
	require.NoError(t, err)
	assert.Equal(t, "not found", resp.Body["status"])

	require.NoError(t, os.WriteFile(fname, []byte("ready"), 0o600))
	resp, err = p.Status(Request{Name: "file", URL: "file://" + fname})
	require.NoError(t, err)
	assert.Equal(t, "found", resp.Body["status"])
	assert.Equal(t, "ready", resp.Body["content"])
	assert.Equal(t, int64(5), resp.Body["size_change"])
}

func TestFileProvider_KeepsLastSuccessfulInfo(t *testing.T) {
	p := FileProvider{TimeOut: time.Second}
	fname := filepath.Join(t.TempDir(), "replaced.txt")
	require.NoError(t, os.WriteFile(fname, []byte("old"), 0o600))

	resp, err := p.Status(Request{Name: "file", URL: "file://" + fname})
	require.NoError(t, err)
	assert.Equal(t, int64(3), resp.Body["size_change"])

	require.NoError(t, os.Remove(fname))
	resp, err = p.Status(Request{Name: "file", URL: "file://" + fname})
	require.NoError(t, err)
	assert.Equal(t, "not found", resp.Body["status"])

	require.NoError(t, os.WriteFile(fname, []byte("new-data"), 0o600))
	resp, err = p.Status(Request{Name: "file", URL: "file://" + fname})
	require.NoError(t, err)
	assert.Equal(t, "found", resp.Body["status"])
	assert.Equal(t, int64(5), resp.Body["size_change"])
}

func TestFileProvider_EmptyFile(t *testing.T) {
	p := FileProvider{TimeOut: time.Second}
	fname := filepath.Join(t.TempDir(), "empty.txt")
	require.NoError(t, os.WriteFile(fname, nil, 0o600))

	resp, err := p.Status(Request{Name: "file", URL: "file://" + fname})
	require.NoError(t, err)
	assert.Equal(t, "found", resp.Body["status"])
	assert.Equal(t, int64(0), resp.Body["size"])
	content, ok := resp.Body["content"]
	require.True(t, ok)
	assert.Empty(t, content)
}

func TestFileProvider_Directory(t *testing.T) {
	p := FileProvider{TimeOut: time.Second}
	dir := t.TempDir()

	resp, err := p.Status(Request{Name: "dir", URL: "file://" + dir})
	require.NoError(t, err)
	assert.Equal(t, "found", resp.Body["status"])
	content, ok := resp.Body["content"]
	require.True(t, ok)
	assert.Empty(t, content)
}

func TestNewFileDateFields(t *testing.T) {
	loc := time.FixedZone("test", -7*60*60)
	ts := time.Date(2026, 9, 3, 23, 30, 0, 0, loc)
	assert.Equal(t, fileDateFields{
		YYYY:     "2026",
		YY:       "26",
		MM:       "09",
		DD:       "03",
		YYYYMMDD: "20260903",
		YYYYMM:   "202609",
		YYMMDD:   "260903",
	}, newFileDateFields(ts))
}

func TestFileProvider_ExpandTarget(t *testing.T) {
	loc := time.FixedZone("test", -7*60*60)
	p := FileProvider{now: func() time.Time {
		return time.Date(2026, 9, 3, 23, 30, 0, 0, loc)
	}}

	target := "backup_[[.YYYY]]_[[.YY]]_[[.MM]]_[[.DD]]_[[.YYYYMMDD]]_[[.YYYYMM]]_[[.YYMMDD]].tar"
	expanded, err := p.expandTarget(target)
	require.NoError(t, err)
	assert.Equal(t, "backup_2026_26_09_03_20260903_202609_260903.tar", expanded)

	expanded, err = p.expandTarget("backup-plain.tar")
	require.NoError(t, err)
	assert.Equal(t, "backup-plain.tar", expanded)

	_, err = p.expandTarget("backup_[[.Nope]].tar")
	require.ErrorContains(t, err, "execute file target template")
	_, err = p.expandTarget("backup_[[.YYYY.tar")
	require.ErrorContains(t, err, "parse file target template")
}

func TestFileProvider_GlobNewest(t *testing.T) {
	dir := t.TempDir()
	base := time.Now().Add(-3 * time.Hour)
	a := filepath.Join(dir, "a_gitlab_backup.tar")
	b := filepath.Join(dir, "b_gitlab_backup.tar")
	c := filepath.Join(dir, "c_gitlab_backup.tar")
	writeFile(t, a, "a")
	writeFile(t, b, "bb")
	writeFile(t, c, "ccc")
	setFileTime(t, a, base)
	setFileTime(t, b, base.Add(time.Hour))
	setFileTime(t, c, base.Add(2*time.Hour))

	p := FileProvider{TimeOut: time.Second}
	resp, err := p.Status(Request{Name: "backup", URL: "file://" + filepath.Join(dir, "*_gitlab_backup.tar")})
	require.NoError(t, err)
	assert.Equal(t, c, resp.Body["path"])
	assert.Equal(t, 3, resp.Body["match_count"])
	assert.Equal(t, "ccc", resp.Body["content"])

	setFileTime(t, b, base.Add(2*time.Hour))
	p = FileProvider{TimeOut: time.Second}
	resp, err = p.Status(Request{Name: "backup", URL: "file://" + filepath.Join(dir, "*_gitlab_backup.tar")})
	require.NoError(t, err)
	assert.Equal(t, b, resp.Body["path"])
}

func TestFileProvider_DateTemplateGlob(t *testing.T) {
	loc := time.FixedZone("test", -7*60*60)
	p := FileProvider{TimeOut: time.Second, now: func() time.Time {
		return time.Date(2026, 9, 3, 23, 30, 0, 0, loc)
	}}
	dir := t.TempDir()
	yesterday := filepath.Join(dir, "1788325835_2026_09_02_19.2.4_gitlab_backup.tar")
	today := filepath.Join(dir, "1788412235_2026_09_03_19.2.4_gitlab_backup.tar")
	writeFile(t, yesterday, "old")
	writeFile(t, today, "today")

	pattern := filepath.Join(dir, "*_[[.YYYY]]_[[.MM]]_[[.DD]]_*_gitlab_backup.tar")
	resp, err := p.Status(Request{Name: "backup", URL: "file://" + pattern})
	require.NoError(t, err)
	assert.Equal(t, today, resp.Body["path"])
	assert.Equal(t, 1, resp.Body["match_count"])

	compact := filepath.Join(dir, "*_[[.YYYYMMDD]]_*_gitlab_backup.tar")
	resp, err = p.Status(Request{Name: "backup", URL: "file://" + compact})
	require.NoError(t, err)
	assert.Equal(t, "not found", resp.Body["status"])
	assert.Equal(t, 0, resp.Body["match_count"])
}

func TestFileProvider_QuestionMarkGlob(t *testing.T) {
	dir := t.TempDir()
	want := filepath.Join(dir, "log_ab.txt")
	writeFile(t, want, "two")
	writeFile(t, filepath.Join(dir, "log_a.txt"), "one")
	writeFile(t, filepath.Join(dir, "log_abc.txt"), "three")

	p := FileProvider{TimeOut: time.Second}
	pattern := filepath.Join(dir, "log_%3F%3F.txt")
	resp, err := p.Status(Request{Name: "logs", URL: "file://" + pattern})
	require.NoError(t, err)
	assert.Equal(t, want, resp.Body["path"])
	assert.Equal(t, 1, resp.Body["match_count"])
}

func TestFileProvider_CacheUsesRawTarget(t *testing.T) {
	loc := time.FixedZone("test", -7*60*60)
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, loc)
	p := FileProvider{TimeOut: time.Second, now: func() time.Time { return now }}
	dir := t.TempDir()
	pattern := filepath.Join(dir, "backup_[[.YYYY]]_[[.MM]]_[[.DD]].tar")
	first := filepath.Join(dir, "backup_2026_09_02.tar")
	second := filepath.Join(dir, "backup_2026_09_03.tar")
	writeFile(t, first, "old")

	resp, err := p.Status(Request{Name: "backup", URL: "file://" + pattern})
	require.NoError(t, err)
	assert.Equal(t, first, resp.Body["path"])
	assert.Equal(t, int64(3), resp.Body["size_change"])

	now = now.AddDate(0, 0, 1)
	resp, err = p.Status(Request{Name: "backup", URL: "file://" + pattern})
	require.NoError(t, err)
	assert.Equal(t, "not found", resp.Body["status"])
	assert.Equal(t, 0, resp.Body["match_count"])

	writeFile(t, second, "new-data")
	resp, err = p.Status(Request{Name: "backup", URL: "file://" + pattern})
	require.NoError(t, err)
	assert.Equal(t, second, resp.Body["path"])
	assert.Equal(t, int64(5), resp.Body["size_change"])
}

func TestFileProvider_GlobNoMatches(t *testing.T) {
	p := FileProvider{TimeOut: time.Second}
	pattern := filepath.Join(t.TempDir(), "missing-*.txt")
	resp, err := p.Status(Request{Name: "missing", URL: "file://" + pattern})
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "not found", resp.Body["status"])
	assert.Equal(t, 0, resp.Body["match_count"])
}

func TestFileProvider_GlobStatErrors(t *testing.T) {
	t.Run("skip vanished match", func(t *testing.T) {
		dir := t.TempDir()
		good := filepath.Join(dir, "match-good")
		writeFile(t, good, "good")
		if err := os.Symlink("missing", filepath.Join(dir, "match-gone")); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}

		p := FileProvider{TimeOut: time.Second}
		resp, err := p.Status(Request{Name: "files", URL: "file://" + filepath.Join(dir, "match-*")})
		require.NoError(t, err)
		assert.Equal(t, good, resp.Body["path"])
		assert.Equal(t, 1, resp.Body["match_count"])
	})

	t.Run("propagate stat error", func(t *testing.T) {
		dir := t.TempDir()
		loop := filepath.Join(dir, "match-loop")
		if err := os.Symlink(filepath.Base(loop), loop); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}

		p := FileProvider{TimeOut: time.Second}
		_, err := p.Status(Request{Name: "files", URL: "file://" + filepath.Join(dir, "match-*")})
		require.ErrorContains(t, err, "file stat failed")
	})
}

func TestFileProvider_ExactPathStatError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permissions differ on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "blocked.txt")
	writeFile(t, path, "blocked")
	require.NoError(t, os.Chmod(dir, 0))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //nolint:gosec // cleanup needs directory traversal

	p := FileProvider{TimeOut: time.Second}
	_, err := p.Status(Request{Name: "blocked", URL: "file://" + path})
	require.ErrorContains(t, err, "file stat failed")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func setFileTime(t *testing.T, path string, ts time.Time) {
	t.Helper()
	require.NoError(t, os.Chtimes(path, ts, ts))
}
