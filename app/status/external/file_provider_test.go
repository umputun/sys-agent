package external

import (
	"os"
	"path/filepath"
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
}
