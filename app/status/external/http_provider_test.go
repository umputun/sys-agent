package external

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHttpProvider_Status(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Millisecond * 10)
		w.WriteHeader(http.StatusOK)
		_, e := w.Write([]byte(`{"status": "ok", "foo": "bar"}`))
		require.NoError(t, e)

	}))

	p := HTTPProvider{Client: http.Client{Timeout: time.Second}}
	resp, err := p.Status(Request{Name: "r1", URL: ts.URL})
	require.NoError(t, err)

	assert.Equal(t, "r1", resp.Name)
	assert.Equal(t, 200, resp.StatusCode)
	assert.True(t, resp.ResponseTime > 0)
	assert.Equal(t, map[string]interface{}{"foo": "bar", "status": "ok"}, resp.Body)
}

func TestHttpProvider_StatusHttpNoJson(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Millisecond * 10)
		w.WriteHeader(http.StatusOK)
		_, e := w.Write([]byte(`pong`))
		require.NoError(t, e)

	}))

	p := HTTPProvider{Client: http.Client{Timeout: time.Second}}
	resp, err := p.Status(Request{Name: "r1", URL: ts.URL})
	require.NoError(t, err)

	assert.Equal(t, "r1", resp.Name)
	assert.Equal(t, 200, resp.StatusCode)
	assert.True(t, resp.ResponseTime > 0)
	assert.Equal(t, map[string]interface{}{"text": "pong"}, resp.Body)
}
