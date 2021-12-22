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
				Body:         map[string]interface{}{"status": "ok"},
			},
			{
				Name:         "test2",
				StatusCode:   200,
				ResponseTime: 1000,
				Body:         map[string]interface{}{"status": "ok"},
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
	assert.Equal(t, 1, len(res.Volumes))
	assert.Equal(t, "root", res.Volumes["root"].Name)
	assert.Equal(t, "/", res.Volumes["root"].Path)
	assert.True(t, res.Volumes["root"].UsagePercent > 0)
	assert.True(t, res.MemPercent > 0)
	assert.True(t, res.Loads.One > 0)
	assert.True(t, res.Uptime > 0)

	assert.Equal(t, 2, len(res.ExtServices))
}

func TestService_GetNoExt(t *testing.T) {

	svc := Service{
		Volumes: []Volume{{Name: "root", Path: "/"}},
	}

	res, err := svc.Get()
	require.NoError(t, err)
	t.Logf("%+v", res)
	assert.Equal(t, 1, len(res.Volumes))
	assert.Equal(t, "root", res.Volumes["root"].Name)
	assert.Equal(t, "/", res.Volumes["root"].Path)
	assert.True(t, res.Volumes["root"].UsagePercent > 0)
	assert.True(t, res.MemPercent > 0)
	assert.True(t, res.Loads.One > 0)
	assert.True(t, res.Uptime > 0)

	assert.Equal(t, 0, len(res.ExtServices))
}
