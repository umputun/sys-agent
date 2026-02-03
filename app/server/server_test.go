package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/umputun/sys-agent/app/actuator"
	"github.com/umputun/sys-agent/app/status"
	"github.com/umputun/sys-agent/app/status/external"
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
	assert.Len(t, sts.GetCalls(), 1)
}

func TestActuatorHealthEndpoint(t *testing.T) {
	sts := &StatusMock{
		GetFunc: func() (*status.Info, error) {
			info := &status.Info{
				CPUPercent: 25,
				MemPercent: 50,
				Volumes:    map[string]status.Volume{"root": {Name: "root", Path: "/", UsagePercent: 45}},
				ExtServices: map[string]external.Response{
					"mongo": {Name: "mongo", StatusCode: 200, ResponseTime: 10},
				},
			}
			info.Loads.One = 1.5
			info.Loads.Five = 1.2
			info.Loads.Fifteen = 1.0
			return info, nil
		},
	}
	srv := Rest{Listen: "localhost:54009", Status: sts, Version: "v1"}
	ts := httptest.NewServer(srv.router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/actuator/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	t.Log(string(body))

	var health actuator.HealthResponse
	err = json.Unmarshal(body, &health)
	require.NoError(t, err)

	assert.Equal(t, "UP", health.Status)
	assert.Len(t, health.Components, 5) // cpu, memory, diskSpace:root, service:mongo, loadAverage

	// verify cpu component
	cpu, ok := health.Components["cpu"]
	require.True(t, ok, "cpu component should exist")
	assert.Equal(t, "UP", cpu.Status)
	assert.InDelta(t, 25, cpu.Details["percent"], 0.001)

	// verify memory component
	mem, ok := health.Components["memory"]
	require.True(t, ok, "memory component should exist")
	assert.Equal(t, "UP", mem.Status)
	assert.InDelta(t, 50, mem.Details["percent"], 0.001)

	// verify disk component
	disk, ok := health.Components["diskSpace:root"]
	require.True(t, ok, "diskSpace:root component should exist")
	assert.Equal(t, "UP", disk.Status)
	assert.Equal(t, "/", disk.Details["path"])
	assert.InDelta(t, 45, disk.Details["percent"], 0.001)

	// verify external service component (prefixed with "service:")
	mongo, ok := health.Components["service:mongo"]
	require.True(t, ok, "service:mongo component should exist")
	assert.Equal(t, "UP", mongo.Status)
	assert.InDelta(t, 200, mongo.Details["status_code"], 0.001)
	assert.InDelta(t, 10, mongo.Details["response_time"], 0.001)

	// verify load average component
	load, ok := health.Components["loadAverage"]
	require.True(t, ok, "loadAverage component should exist")
	assert.Equal(t, "UP", load.Status)
	assert.InDelta(t, 1.5, load.Details["one"], 0.001)
	assert.InDelta(t, 1.2, load.Details["five"], 0.001)
	assert.InDelta(t, 1.0, load.Details["fifteen"], 0.001)

	assert.Len(t, sts.GetCalls(), 1)
}

func TestActuatorHealthEndpoint_Down(t *testing.T) {
	sts := &StatusMock{
		GetFunc: func() (*status.Info, error) {
			info := &status.Info{
				CPUPercent: 95, // above threshold - should be DOWN
				MemPercent: 50,
				Volumes:    map[string]status.Volume{},
			}
			info.Loads.One = 1.0
			info.Loads.Five = 1.0
			info.Loads.Fifteen = 1.0
			return info, nil
		},
	}
	srv := Rest{Listen: "localhost:54009", Status: sts, Version: "v1"}
	ts := httptest.NewServer(srv.router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/actuator/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode, "should return 503 when status is DOWN")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var health actuator.HealthResponse
	err = json.Unmarshal(body, &health)
	require.NoError(t, err)

	assert.Equal(t, "DOWN", health.Status)
	assert.Equal(t, "DOWN", health.Components["cpu"].Status)
}

func TestActuatorHealthEndpoint_Error(t *testing.T) {
	sts := &StatusMock{
		GetFunc: func() (*status.Info, error) {
			return nil, assert.AnError
		},
	}
	srv := Rest{Listen: "localhost:54009", Status: sts, Version: "v1"}
	ts := httptest.NewServer(srv.router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/actuator/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "failed to get status")
}
