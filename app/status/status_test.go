package status

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/umputun/sys-agent/app/status/external"
)

func TestService_Get(t *testing.T) {

	ex := &ExtServicesMock{StatusFunc: func() []external.Response {
		return []external.Response{
			{
				Name:         "test1",
				StatusCode:   200,
				ResponseTime: 1000,
				Body:         map[string]any{"status": "ok"},
			},
			{
				Name:         "test2",
				StatusCode:   200,
				ResponseTime: 1000,
				Body:         map[string]any{"status": "ok"},
			},
		}
	}}

	svc := Service{
		Volumes:     []Volume{{Name: "root", Path: "/"}},
		ExtServices: ex,
	}

	res, err := svc.Get()
	require.NoError(t, err)
	t.Logf("%+v", res)
	assert.Len(t, res.Volumes, 1)
	assert.Equal(t, "root", res.Volumes["root"].Name)
	assert.Equal(t, "/", res.Volumes["root"].Path)
	assert.Positive(t, res.Volumes["root"].UsagePercent)
	assert.Positive(t, res.MemPercent)
	assert.Positive(t, res.Loads.One)
	assert.Positive(t, res.Uptime)

	assert.Len(t, res.ExtServices, 2)
}

func TestService_GetNoExt(t *testing.T) {

	svc := Service{
		Volumes: []Volume{{Name: "root", Path: "/"}},
	}

	res, err := svc.Get()
	require.NoError(t, err)
	t.Logf("%+v", res)
	assert.Len(t, res.Volumes, 1)
	assert.Equal(t, "root", res.Volumes["root"].Name)
	assert.Equal(t, "/", res.Volumes["root"].Path)
	assert.Positive(t, res.Volumes["root"].UsagePercent)
	assert.Positive(t, res.MemPercent)
	assert.Positive(t, res.Loads.One)
	assert.Positive(t, res.Uptime)

	assert.Empty(t, res.ExtServices)
}
