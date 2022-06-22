package external

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCertificateProvider_Status(t *testing.T) {
	cp := CertificateProvider{TimeOut: time.Minute}
	resp, err := cp.Status(Request{Name: "test", URL: "cert://umputun.com"})
	require.NoError(t, err)
	t.Logf("%+v", resp)
	assert.Equal(t, "test", resp.Name)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "ok", resp.Body["status"])
	assert.Equal(t, "https://umputun.com", resp.Body["host"])

	exp, err := time.Parse(time.RFC3339, resp.Body[`expire`].(string))
	require.NoError(t, err)
	assert.True(t, exp.After(time.Now().Add(5*24*time.Hour)))
	t.Logf("expire: %+v", exp)
}

func TestCertificateProvider_StatusFailed(t *testing.T) {
	cp := CertificateProvider{TimeOut: time.Minute}
	_, err := cp.Status(Request{Name: "test", URL: "cert://127.0.0.1"})
	require.Error(t, err)
}
