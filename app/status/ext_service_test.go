package status

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtServices_Status(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Millisecond * 10)
		w.WriteHeader(http.StatusOK)
		_, e := w.Write([]byte(`{"status": "ok", "foo": "bar"}`))
		require.NoError(t, e)

	}))

	svc := NewExtServices(time.Second, 4, "e1:"+ts.URL+"/status1", "e2:"+ts.URL+"/status2")
	res := svc.Status()
	assert.Equal(t, 2, len(res))

	assert.Equal(t, "e1", res[0].Name)
	assert.Equal(t, 200, res[0].StatusCode)
	assert.True(t, res[0].ResponseTime > 0)
	assert.Equal(t, map[string]interface{}{"foo": "bar", "status": "ok"}, res[0].Body)

	assert.Equal(t, "e2", res[1].Name)
	assert.Equal(t, 200, res[1].StatusCode)
	assert.True(t, res[1].ResponseTime > 0)
	assert.Equal(t, map[string]interface{}{"foo": "bar", "status": "ok"}, res[1].Body)
}

func TestExtServices_StatusMany(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, e := w.Write([]byte(`{"status": "ok"}`))
		require.NoError(t, e)
	}))

	var endpoints []string
	for i := 0; i < 100; i++ {
		endpoints = append(endpoints, "e"+strconv.Itoa(i)+":"+ts.URL+"/status"+strconv.Itoa(i))
	}

	svc := NewExtServices(time.Second, 16, endpoints...)
	res := svc.Status()
	assert.Equal(t, 100, len(res))
}
