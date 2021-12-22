package external

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDockerProvider_parseDockerResponse(t *testing.T) {
	fh, err := os.Open("testdata/containers.json")
	require.NoError(t, err)

	p := DockerProvider{}
	res, err := p.parseDockerResponse(fh)
	require.NoError(t, err)
	t.Logf("%+v", res)
	assert.Equal(t, 5, len(res))
	assert.Equal(t, "map[nginx:{nginx running Up 2 seconds} weather:{weather running Up 2 hours (healthy)}]", fmt.Sprintf("%v", res["containers"]))
	assert.Equal(t, 2, res["total"])
	assert.Equal(t, 2, res["running"])
	assert.Equal(t, 1, res["healthy"])
	assert.Equal(t, 0, res["failed"])
}
