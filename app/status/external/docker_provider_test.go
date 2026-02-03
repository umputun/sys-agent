package external

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDockerProvider_Status(t *testing.T) {
	ts := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/v1.24/containers/json", r.URL.Path)
				time.Sleep(time.Millisecond * 10)
				w.WriteHeader(http.StatusOK)
				data, err := os.ReadFile("testdata/containers.json")
				assert.NoError(t, err)
				_, e := w.Write(data)
				assert.NoError(t, e)

			},
		),
	)

	p := DockerProvider{TimeOut: time.Second}
	resp, err := p.Status(Request{Name: "d1", URL: strings.Replace(ts.URL, "http://", "tcp://", 1)})
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Greater(t, resp.ResponseTime, int64(1))
}

func TestDockerProvider_StatusWithRequired(t *testing.T) {
	ts := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/v1.24/containers/json", r.URL.Path)
				time.Sleep(time.Millisecond * 10)
				w.WriteHeader(http.StatusOK)
				data, err := os.ReadFile("testdata/containers.json")
				assert.NoError(t, err)
				_, e := w.Write(data)
				assert.NoError(t, e)

			},
		),
	)

	p := DockerProvider{TimeOut: time.Second}

	{
		resp, err := p.Status(
			Request{Name: "d1", URL: strings.Replace(ts.URL, "http://", "tcp://", 1) + "?containers=c1:c2"})
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "failed: c1,c2", resp.Body["required"])
	}

	{
		resp, err := p.Status(
			Request{Name: "d1", URL: strings.Replace(ts.URL, "http://", "tcp://", 1) + "?containers=nginx : weather"})
		require.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "ok", resp.Body["required"])
	}
}

func TestDockerProvider_parseDockerResponse(t *testing.T) {
	fh, err := os.Open("testdata/containers.json")
	require.NoError(t, err)

	p := DockerProvider{}
	res, err := p.parseDockerResponse(fh, nil)
	require.NoError(t, err)
	t.Logf("%+v", res)
	assert.Len(t, res, 7)
	assert.Equal(t, "map[blah:{blah running Up 21 hours (unhealthy)} nginx:{nginx running Up 2 seconds} "+
		"weather:{weather running Up 2 hours (healthy)}]", fmt.Sprintf("%v", res["containers"]),
	)
	assert.Equal(t, 3, res["total"])
	assert.Equal(t, 3, res["running"])
	assert.Equal(t, 1, res["healthy"])
	assert.Equal(t, 1, res["unhealthy"])
	assert.Equal(t, 0, res["failed"])
	assert.Equal(t, "ok", res["required"])
}
