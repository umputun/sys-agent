package external

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRMQ_Status(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/queues/feeds/notification.queue", r.URL.Path)
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		body, err := os.ReadFile("testdata/rmq.json")
		require.NoError(t, err)
		_, err = w.Write(body)
		require.NoError(t, err)
	}))
	defer ts.Close()

	rmq := RMQProvider{TimeOut: time.Second}
	u := ts.URL + "/queues/feeds/notification.queue"
	u = strings.Replace(u, "http://", "rmq://", 1)

	{
		resp, err := rmq.Status(Request{Name: "rmq-test", URL: u})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		t.Logf("resp: %+v", resp)

		assert.Equal(t, "rmq-test", resp.Name)
		assert.Equal(t, 200, resp.StatusCode)
		assert.True(t, resp.ResponseTime > 0)
		assert.Equal(t, "feeds", resp.Body["vhost"])
		assert.Equal(t, "notification.queue", resp.Body["name"])
		assert.Equal(t, 56178, resp.Body["messages"])
		assert.Equal(t, 56178, resp.Body["messages_ready"])
		assert.Equal(t, 0, resp.Body["messages_unacknowledged"])
		assert.Equal(t, 4, resp.Body["consumers"])
		assert.Equal(t, 15.5, resp.Body["avg_egress_rate"])
		assert.Equal(t, 19.9, resp.Body["avg_ingress_rate"])
		assert.Equal(t, 3771, resp.Body["messages_ready_ram"])
		assert.Equal(t, 13847734, resp.Body["publish"])
		assert.Equal(t, 56178, resp.Body["messages_delta"])
	}

	{
		resp, err := rmq.Status(Request{Name: "rmq-test", URL: u})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		t.Logf("resp: %+v", resp)
		assert.Equal(t, 0, resp.Body["messages_delta"])
	}
}

func TestRMQ_StatusFailed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/queues/feeds/notification.queue", r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	rmq := RMQProvider{TimeOut: time.Second}
	u := ts.URL + "/queues/feeds/notification.queue"
	u = strings.Replace(u, "http://", "rmq://", 1)
	_, err := rmq.Status(Request{Name: "rmq-test", URL: u})
	require.ErrorContains(t, err, "failed to get RabbitMQ response")
	require.ErrorContains(t, err, "500 Internal Server Error")
}
