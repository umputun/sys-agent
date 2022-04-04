package external

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNginxProvider_Status(t *testing.T) {
	ts := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/nginx_status", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				data, err := os.ReadFile("testdata/nginx.txt")
				require.NoError(t, err)
				_, e := w.Write(data)
				require.NoError(t, e)
			},
		),
	)

	provider := NginxProvider{TimeOut: time.Second}
	res, err := provider.Status(Request{Name: "nginx-test", URL: ts.URL + "/nginx_status"})
	require.NoError(t, err)
	exp := map[string]interface{}{"accepts": 1377590, "active_connections": 125, "change_handled": 1377590, "handled": 1377590,
		"reading": 2, "requests": 1873302, "waiting": 10, "writing": 115}
	assert.EqualValues(t, exp, res.Body)
}

func TestNginxProvider_StatusFailedTooShort(t *testing.T) {
	ts := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/nginx_status", r.URL.Path)
				w.WriteHeader(http.StatusOK)
				_, e := w.Write([]byte("blah"))
				require.NoError(t, e)
			},
		),
	)

	provider := NginxProvider{TimeOut: time.Second}
	_, err := provider.Status(Request{Name: "nginx-test", URL: ts.URL + "/nginx_status"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "response is too short")
}
