package external

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileProvider_Status(t *testing.T) {
	p := FileProvider{TimeOut: time.Second}
	{
		resp, err := p.Status(Request{Name: "r1", URL: "file://testdata/ping.txt"})
		require.NoError(t, err)
		t.Logf("%+v", resp)
		assert.Equal(t, "r1", resp.Name)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "pong", resp.Body["content"])
		assert.Equal(t, "found", resp.Body["status"])
		assert.Equal(t, int64(4), resp.Body["size"])
		// assert.Equal(t, "2022-07-11T16:12:03.674378878-05:00", resp.Body["modif_time"])
		assert.True(t, resp.Body["since_modif"].(int64) > 100)
	}

	{
		resp, err := p.Status(Request{Name: "r1", URL: "file://testdata/bad.txt"})
		require.NoError(t, err)
		t.Logf("%+v", resp)
		assert.Equal(t, "not found", resp.Body["status"])
	}
}
