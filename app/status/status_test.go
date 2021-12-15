package status

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_Get(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, e := w.Write([]byte(`{"status": "ok"}`))
		require.NoError(t, e)

	}))

	svc := Service{
		Volumes:     []Volume{{Name: "root", Path: "/"}},
		ExtServices: NewExtServices(time.Second, 4, "e1:"+ts.URL+"/status1", "e2:"+ts.URL+"/status2"),
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
