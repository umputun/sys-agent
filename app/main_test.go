package main

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/umputun/sys-agent/app/status"
)

func Test_parseVolumes(t *testing.T) {

	tbl := []struct {
		inp  []string
		vols []status.Volume
		err  error
	}{
		{[]string{"data volume:/data"}, []status.Volume{{Name: "data volume", Path: "/data"}}, nil},
		{[]string{"data volume:/data", "blah:/"},
			[]status.Volume{{Name: "data volume", Path: "/data"}, {Name: "blah", Path: "/"}}, nil},
		{[]string{"/data"}, []status.Volume{}, errors.New("invalid volume format, should be <name>:<path>")},
	}

	for i, tt := range tbl {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			vols, err := parseVolumes(tt.inp)
			if tt.err != nil {
				require.EqualError(t, err, tt.err.Error())
				return
			}
			assert.Equal(t, tt.vols, vols)
		})
	}
}

func Test_main(t *testing.T) {
	port := 40000 + int(rand.Int31n(1000))
	os.Args = []string{"app", "--listen=127.0.0.1:" + strconv.Itoa(port), "-v root:/", "-s echo:https://echo.umputun.com", "--dbg"}

	done := make(chan struct{})
	go func() {
		<-done
		e := syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
		require.NoError(t, e)
	}()

	finished := make(chan struct{})
	go func() {
		main()
		close(finished)
	}()

	// defer cleanup because require check below can fail
	defer func() {
		close(done)
		<-finished
	}()

	waitForHTTPServerStart(port)
	time.Sleep(time.Second)

	{
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/ping", port))
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, "pong", string(body))
	}

	{
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/status", port))
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, 200, resp.StatusCode)
	}
}

func waitForHTTPServerStart(port int) {
	// wait for up to 10 seconds for server to start before returning it
	client := http.Client{Timeout: time.Second}
	for i := 0; i < 100; i++ {
		time.Sleep(time.Millisecond * 100)
		if resp, err := client.Get(fmt.Sprintf("http://localhost:%d/ping", port)); err == nil {
			_ = resp.Body.Close()
			return
		}
	}
}
