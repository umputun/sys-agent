package status

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/go-pkgz/mongo/v2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtServices_StatusHttp(t *testing.T) {
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

func TestExtServices_StatusHttpNoJosn(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Millisecond * 10)
		w.WriteHeader(http.StatusOK)
		_, e := w.Write([]byte(`pong`))
		require.NoError(t, e)

	}))

	svc := NewExtServices(time.Second, 4, "e1:"+ts.URL+"/status1", "e2:"+ts.URL+"/status2")
	res := svc.Status()
	assert.Equal(t, 2, len(res))

	assert.Equal(t, "e1", res[0].Name)
	assert.Equal(t, 200, res[0].StatusCode)
	assert.True(t, res[0].ResponseTime > 0)
	assert.Equal(t, map[string]interface{}{"text": "pong"}, res[0].Body)

	assert.Equal(t, "e2", res[1].Name)
	assert.Equal(t, 200, res[1].StatusCode)
	assert.True(t, res[1].ResponseTime > 0)
	assert.Equal(t, map[string]interface{}{"text": "pong"}, res[1].Body)
}

func TestExtServices_StatusMongo(t *testing.T) {
	_, _, teardown := mongo.MakeTestConnection(t)
	defer teardown()

	{
		svc := NewExtServices(time.Second, 4, "m1:mongodb://127.0.0.1:27017/test")
		res := svc.Status()
		assert.Equal(t, 1, len(res))
		assert.Equal(t, "m1", res[0].Name)
		assert.Equal(t, 200, res[0].StatusCode)
		assert.True(t, res[0].ResponseTime > 0)
		assert.Equal(t, map[string]interface{}{"status": "ok"}, res[0].Body)
	}
	{
		svc := NewExtServices(time.Second, 4, "m1:mongodb://127.0.0.1:27000/test")
		res := svc.Status()
		assert.Equal(t, 1, len(res))
		assert.Equal(t, "m1", res[0].Name)
		assert.Equal(t, 500, res[0].StatusCode)
		assert.True(t, res[0].ResponseTime >= 1000)
		t.Logf("%+v", res[0])
	}
}
