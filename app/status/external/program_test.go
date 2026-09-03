package external

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProgramProvider_Status(t *testing.T) {
	p := ProgramProvider{TimeOut: time.Second}

	t.Run("command succeeds", func(t *testing.T) {
		resp, err := p.Status(Request{Name: "test", URL: `program://ls?args=-la`})
		require.NoError(t, err)
		assert.Equal(t, "test", resp.Name)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "ok", resp.Body["status"])
		assert.Contains(t, resp.Body["stdout"], "program.go")
	})

	t.Run("script succeeds", func(t *testing.T) {
		resp, err := p.Status(Request{Name: "test", URL: `program://testdata/test.sh`})
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "ok", resp.Body["status"])
		assert.Contains(t, resp.Body["stdout"], "Hello, World!")
	})

	t.Run("command is missing", func(t *testing.T) {
		resp, err := p.Status(Request{Name: "test", URL: `program://blah?args=-la`})
		require.NoError(t, err)
		assert.Equal(t, 500, resp.StatusCode)
		assert.Contains(t, resp.Body["status"], "file not found")
	})

	t.Run("argument succeeds", func(t *testing.T) {
		resp, err := p.Status(Request{Name: "test", URL: `program://cat?args=program.go`})
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "ok", resp.Body["status"])
		assert.Contains(t, resp.Body["stdout"], "CommandContext")
	})

	t.Run("argument fails", func(t *testing.T) {
		resp, err := p.Status(Request{Name: "test", URL: `program://cat?args=blah`})
		require.NoError(t, err)
		assert.Equal(t, 500, resp.StatusCode)
		assert.Contains(t, resp.Body["status"], "exit status 1")
	})
}

func TestProgramProvider_Arguments(t *testing.T) {
	p := ProgramProvider{TimeOut: time.Second}
	argCounter, err := filepath.Abs("testdata/argcount.sh")
	require.NoError(t, err)
	tests := []struct {
		name        string
		url         string
		wantOutput  string
		wantCommand string
	}{
		{
			name:        "no argument",
			url:         "program://" + argCounter,
			wantOutput:  "0\n",
			wantCommand: argCounter,
		},
		{
			name:        "repeated arguments",
			url:         "program://testdata/argcount.sh?args=-e&args=-f",
			wantOutput:  "2\n<-e>\n<-f>\n",
			wantCommand: "testdata/argcount.sh -e -f",
		},
		{
			name:        "cron query",
			url:         "program://testdata/argcount.sh?cron=0_6_*_*_*",
			wantOutput:  "0\n",
			wantCommand: "testdata/argcount.sh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := p.Status(Request{Name: "test", URL: tt.url})
			require.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)
			assert.Equal(t, tt.wantOutput, resp.Body["stdout"])
			assert.Equal(t, tt.wantCommand, resp.Body["command"])
		})
	}

	t.Run("repeated ps arguments", func(t *testing.T) {
		resp, err := p.Status(Request{Name: "test", URL: "program://ps?args=-e&args=-f"})
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "ps -e -f", resp.Body["command"])
		assert.NotEmpty(t, resp.Body["stdout"])
	})
}

func TestProgramProvider_ExplicitShell(t *testing.T) {
	p := ProgramProvider{TimeOut: time.Second}
	resp, err := p.Status(Request{
		Name: "test",
		URL:  "program:///bin/sh?args=-c&args=printf+shell-ok",
	})
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "shell-ok", resp.Body["stdout"])
	assert.Equal(t, "/bin/sh -c printf shell-ok", resp.Body["command"])
}
