package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/umputun/sys-agent/app/status"
)

func TestRest_Run(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	srv := Rest{Listen: "localhost:54009", Version: "v1"}
	err := srv.Run(ctx)
	require.Error(t, err)
	assert.Equal(t, "http: Server closed", err.Error())
}

func TestStatusCtrl(t *testing.T) {
	sts := &StatusMock{
		GetFunc: func() (*status.Info, error) {
			return &status.Info{CPUPercent: 12, Volumes: map[string]status.Volume{"v1": {Name: "v1", Path: "/p1", UsagePercent: 5}}}, nil
		},
	}
	srv := Rest{Listen: "localhost:54009", Status: sts, Version: "v1"}
	ts := httptest.NewServer(srv.router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/status")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	t.Log(string(body))
	assert.Contains(t, string(body), `"volumes":`, string(body))
	assert.Contains(t, string(body), `"mem_percent":`, string(body))
	assert.Contains(t, string(body), `"cpu_percent":`, string(body))
	assert.Contains(t, string(body), `"uptime":`, string(body))
	assert.Contains(t, string(body), `"load_average":`, string(body))
	assert.Equal(t, 1, len(sts.GetCalls()))
}
