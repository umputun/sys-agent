package status

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_Get(t *testing.T) {
	svc := Service{Volumes: []Volume{{Name: "root", Path: "/"}}}
	res, err := svc.Get()
	require.NoError(t, err)
	assert.Equal(t, 1, len(res.Volumes))
	assert.Equal(t, "root", res.Volumes["root"].Name)
	assert.Equal(t, "/", res.Volumes["root"].Path)
	assert.True(t, res.Volumes["root"].UsagePercent > 0)
}
