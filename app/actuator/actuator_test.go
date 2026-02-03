package actuator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/umputun/sys-agent/app/status"
	"github.com/umputun/sys-agent/app/status/external"
)

func TestFromStatusInfo(t *testing.T) {
	tests := []struct {
		name       string
		info       *status.Info
		wantStatus string
		wantNil    bool
	}{
		{
			name:    "nil input returns nil",
			info:    nil,
			wantNil: true,
		},
		{
			name: "all components healthy - status UP",
			info: &status.Info{
				CPUPercent: 25,
				MemPercent: 50,
				Volumes: map[string]status.Volume{
					"root": {Name: "root", Path: "/", UsagePercent: 45},
				},
				ExtServices: map[string]external.Response{
					"mongo": {Name: "mongo", StatusCode: 200, ResponseTime: 10},
				},
				Loads: struct {
					One     float64 `json:"one"`
					Five    float64 `json:"five"`
					Fifteen float64 `json:"fifteen"`
				}{One: 1.5, Five: 1.2, Fifteen: 1.0},
			},
			wantStatus: StatusUp,
		},
		{
			name: "cpu at threshold - status UP",
			info: &status.Info{
				CPUPercent: 89,
				MemPercent: 50,
			},
			wantStatus: StatusUp,
		},
		{
			name: "cpu exceeds threshold - status DOWN",
			info: &status.Info{
				CPUPercent: 90,
				MemPercent: 50,
			},
			wantStatus: StatusDown,
		},
		{
			name: "memory exceeds threshold - status DOWN",
			info: &status.Info{
				CPUPercent: 25,
				MemPercent: 95,
			},
			wantStatus: StatusDown,
		},
		{
			name: "disk exceeds threshold - status DOWN",
			info: &status.Info{
				CPUPercent: 25,
				MemPercent: 50,
				Volumes: map[string]status.Volume{
					"data": {Name: "data", Path: "/data", UsagePercent: 92},
				},
			},
			wantStatus: StatusDown,
		},
		{
			name: "service error - status DOWN",
			info: &status.Info{
				CPUPercent: 25,
				MemPercent: 50,
				ExtServices: map[string]external.Response{
					"api": {Name: "api", StatusCode: 500, ResponseTime: 100},
				},
			},
			wantStatus: StatusDown,
		},
		{
			name: "service 2xx codes - status UP",
			info: &status.Info{
				CPUPercent: 25,
				MemPercent: 50,
				ExtServices: map[string]external.Response{
					"api1": {Name: "api1", StatusCode: 200, ResponseTime: 10},
					"api2": {Name: "api2", StatusCode: 201, ResponseTime: 20},
					"api3": {Name: "api3", StatusCode: 299, ResponseTime: 30},
				},
			},
			wantStatus: StatusUp,
		},
		{
			name: "empty volumes and services - status UP",
			info: &status.Info{
				CPUPercent: 25,
				MemPercent: 50,
			},
			wantStatus: StatusUp,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FromStatusInfo(tc.info)

			if tc.wantNil {
				assert.Nil(t, result)
				return
			}

			require.NotNil(t, result)
			assert.Equal(t, tc.wantStatus, result.Status)
		})
	}
}

func TestFromStatusInfo_ComponentDetails(t *testing.T) {
	info := &status.Info{
		CPUPercent: 25,
		MemPercent: 50,
		Volumes: map[string]status.Volume{
			"root": {Name: "root", Path: "/", UsagePercent: 45},
			"data": {Name: "data", Path: "/data", UsagePercent: 60},
		},
		ExtServices: map[string]external.Response{
			"mongo": {Name: "mongo", StatusCode: 200, ResponseTime: 10, Body: map[string]any{"version": "4.4"}},
		},
		Loads: struct {
			One     float64 `json:"one"`
			Five    float64 `json:"five"`
			Fifteen float64 `json:"fifteen"`
		}{One: 1.5, Five: 1.2, Fifteen: 1.0},
	}

	result := FromStatusInfo(info)
	require.NotNil(t, result)
	require.NotNil(t, result.Components)

	// check cpu component
	cpu, ok := result.Components["cpu"]
	require.True(t, ok, "cpu component should exist")
	assert.Equal(t, StatusUp, cpu.Status)
	assert.Equal(t, 25, cpu.Details["percent"])

	// check memory component
	mem, ok := result.Components["memory"]
	require.True(t, ok, "memory component should exist")
	assert.Equal(t, StatusUp, mem.Status)
	assert.Equal(t, 50, mem.Details["percent"])

	// check disk components
	diskRoot, ok := result.Components["diskSpace:root"]
	require.True(t, ok, "diskSpace:root component should exist")
	assert.Equal(t, StatusUp, diskRoot.Status)
	assert.Equal(t, "/", diskRoot.Details["path"])
	assert.Equal(t, 45, diskRoot.Details["percent"])

	diskData, ok := result.Components["diskSpace:data"]
	require.True(t, ok, "diskSpace:data component should exist")
	assert.Equal(t, StatusUp, diskData.Status)
	assert.Equal(t, "/data", diskData.Details["path"])
	assert.Equal(t, 60, diskData.Details["percent"])

	// check service component (prefixed with "service:")
	mongo, ok := result.Components["service:mongo"]
	require.True(t, ok, "service:mongo component should exist")
	assert.Equal(t, StatusUp, mongo.Status)
	assert.Equal(t, 200, mongo.Details["status_code"])
	assert.Equal(t, int64(10), mongo.Details["response_time"])
	assert.Equal(t, map[string]any{"version": "4.4"}, mongo.Details["body"])

	// check load average component
	loadAvg, ok := result.Components["loadAverage"]
	require.True(t, ok, "loadAverage component should exist")
	assert.Equal(t, StatusUp, loadAvg.Status)
	assert.InDelta(t, 1.5, loadAvg.Details["one"], 0.001)
	assert.InDelta(t, 1.2, loadAvg.Details["five"], 0.001)
	assert.InDelta(t, 1.0, loadAvg.Details["fifteen"], 0.001)
}

func TestFromStatusInfo_ComponentStatusDown(t *testing.T) {
	info := &status.Info{
		CPUPercent: 95,
		MemPercent: 91,
		Volumes: map[string]status.Volume{
			"full": {Name: "full", Path: "/full", UsagePercent: 98},
		},
		ExtServices: map[string]external.Response{
			"bad": {Name: "bad", StatusCode: 503, ResponseTime: 5000},
		},
		Loads: struct {
			One     float64 `json:"one"`
			Five    float64 `json:"five"`
			Fifteen float64 `json:"fifteen"`
		}{One: 10.0, Five: 8.0, Fifteen: 6.0},
	}

	result := FromStatusInfo(info)
	require.NotNil(t, result)
	assert.Equal(t, StatusDown, result.Status)

	// individual component statuses
	assert.Equal(t, StatusDown, result.Components["cpu"].Status)
	assert.Equal(t, StatusDown, result.Components["memory"].Status)
	assert.Equal(t, StatusDown, result.Components["diskSpace:full"].Status)
	assert.Equal(t, StatusDown, result.Components["service:bad"].Status)

	// load average is always UP (informational only)
	assert.Equal(t, StatusUp, result.Components["loadAverage"].Status)
}
