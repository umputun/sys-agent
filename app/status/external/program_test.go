package external

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProgram_StatusWithShell(t *testing.T) {
	p := ProgramProvider{WithShell: true, TimeOut: time.Second}

	{
		req := Request{Name: "test", URL: `program://ls?args=-la`}
		resp, err := p.Status(req)
		require.NoError(t, err)
		assert.Equal(t, "test", resp.Name)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "ok", resp.Body["status"])
		assert.Contains(t, resp.Body["stdout"], "program.go")
		t.Logf("%+v", resp)
	}
	{
		req := Request{Name: "test", URL: `program://testdata/test.sh`}
		resp, err := p.Status(req)
		require.NoError(t, err)
		assert.Equal(t, "test", resp.Name)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "ok", resp.Body["status"])
		assert.Contains(t, resp.Body["stdout"], "Hello, World!")
	}
	{
		req := Request{Name: "test", URL: `program://blah?args=-la`}
		resp, err := p.Status(req)
		require.NoError(t, err)
		assert.Equal(t, "test", resp.Name)
		assert.Equal(t, 500, resp.StatusCode)
		assert.Contains(t, resp.Body["status"], "file not found")
	}
}

func TestProgram_StatusWithoutShell(t *testing.T) {
	p := ProgramProvider{WithShell: true, TimeOut: time.Second}

	{
		req := Request{Name: "test", URL: `program://cat?args=program.go`}
		resp, err := p.Status(req)
		require.NoError(t, err)
		assert.Equal(t, "test", resp.Name)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "ok", resp.Body["status"])
		assert.Contains(t, resp.Body["stdout"], "CommandContext")
	}
	{
		req := Request{Name: "test", URL: `program://cat?args=blah`}
		resp, err := p.Status(req)
		require.NoError(t, err)
		assert.Equal(t, "test", resp.Name)
		assert.Equal(t, 500, resp.StatusCode)
		assert.Contains(t, resp.Body["status"], "exit status 1", resp.Body["status"])
		t.Logf("%+v", resp)
	}
}
