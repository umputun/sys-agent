package config

import (
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	{
		_, err := New("testdata/invalid.yml")
		require.EqualError(t, err, "can't read config testdata/invalid.yml: open testdata/invalid.yml: no such file or directory")
	}

	{
		p, err := New("testdata/config.yml")
		require.NoError(t, err)
		assert.Equal(t, []Volume{{Name: "root", Path: "/hostroot"}, {Name: "data", Path: "/data"}}, p.Volumes)
		assert.Equal(t, []Certificate{{Name: "prim_cert", URL: "https://example1.com"},
			{Name: "second_cert", URL: "https://example2.com"}}, p.Services.Certificate)
		assert.Equal(t, []Docker{
			{Name: "docker1", URL: "unix:///var/run/docker.sock", Containers: []string{"reproxy", "mattermost", "postgres"}},
			{Name: "docker2", URL: "tcp://192.168.1.1:4080", Containers: []string(nil)}}, p.Services.Docker)
		assert.Equal(t, []File{{Name: "first", Path: "/tmp/example1.txt"}, {Name: "second", Path: "/tmp/example2.txt"}},
			p.Services.File)
		assert.Equal(t, []HTTP{{Name: "first", URL: "https://example1.com"}, {Name: "second", URL: "https://example2.com"}},
			p.Services.HTTP)
		assert.Equal(t, []Mongo{{Name: "dev", URL: "mongodb://example.com:27017", OplogMaxDelta: 30 * time.Minute}},
			p.Services.Mongo)
		assert.Equal(t, []Nginx{{Name: "nginx", StatusURL: "http://example.com:80"}}, p.Services.Nginx)
		assert.Equal(t, []RMQ{{Name: "rmqtest", URL: "http://example.com:15672", User: "guest", Pass: "passwd",
			Vhost: "v1", Queue: "q1"}}, p.Services.RMQ)
	}
}

func TestParameters_MarshalVolumes(t *testing.T) {
	p, err := New("testdata/config.yml")
	require.NoError(t, err)
	assert.Equal(t, []string{"root:/hostroot", "data:/data"}, p.MarshalVolumes())
}

func TestParameters_String(t *testing.T) {
	p, err := New("testdata/config.yml")
	require.NoError(t, err)
	exp := "config file: \"testdata/config.yml\", {Volumes:[{Name:root Path:/hostroot} {Name:data Path:/data}] " +
		"Services:{HTTP:[{Name:first URL:https://example1.com} " +
		"{Name:second URL:https://example2.com}] " +
		"Certificate:[{Name:prim_cert URL:https://example1.com} " +
		"{Name:second_cert URL:https://example2.com}] " +
		"File:[{Name:first Path:/tmp/example1.txt} " +
		"{Name:second Path:/tmp/example2.txt}] " +
		"Mongo:[{Name:dev URL:mongodb://example.com:27017 OplogMaxDelta:30m0s Collection: DB: CountQuery:}] " +
		"Nginx:[{Name:nginx StatusURL:http://example.com:80}] " +
		"Program:[{Name:first Path:/usr/bin/example1 Args:[arg1 arg2]} " +
		"{Name:second Path:/usr/bin/example2 Args:[]}] " +
		"Docker:[{Name:docker1 URL:unix:///var/run/docker.sock Containers:[reproxy mattermost postgres]} " +
		"{Name:docker2 URL:tcp://192.168.1.1:4080 Containers:[]}] " +
		"RMQ:[{Name:rmqtest URL:http://example.com:15672 User:guest Pass:passwd Vhost:v1 Queue:q1}]} " +
		"fileName:testdata/config.yml}"
	assert.Equal(t, exp, p.String())
}

func TestParameters_MarshalServices(t *testing.T) {
	t.Run("config.yml directly", func(t *testing.T) {
		p, err := New("testdata/config.yml")
		require.NoError(t, err)
		exp := []string{
			"first:https://example1.com", "second:https://example2.com",
			"prim_cert:cert://example1.com", "second_cert:cert://example2.com",
			"docker1:docker:///var/run/docker.sock?containers=reproxy:mattermost:postgres", "docker2:docker://192.168.1.1:4080",
			"first:file:///tmp/example1.txt", "second:file:///tmp/example2.txt",
			"dev:mongodb://example.com:27017?oplogMaxDelta=30m0s",
			"nginx:nginx://example.com:80",
			"first:program:///usr/bin/example1?args=arg1&args=arg2", "second:program:///usr/bin/example2",
			"rmqtest:rmq://guest:passwd@example.com:15672/v1/q1",
		}
		assert.Equal(t, exp, p.MarshalServices())
	})

	t.Run("mongo with query params", func(t *testing.T) {
		p, err := New("testdata/config.yml")
		require.NoError(t, err)
		p.Services.Mongo[0].URL = "mongodb://example.com:27017/admin?foo=bar&blah=blah"

		var rawURL string
		for _, service := range p.MarshalServices() {
			if strings.HasPrefix(service, "dev:mongodb://") {
				rawURL = strings.TrimPrefix(service, "dev:")
				break
			}
		}
		require.NotEmpty(t, rawURL)
		parsed, err := url.Parse(rawURL)
		require.NoError(t, err)
		assert.Equal(t, "bar", parsed.Query().Get("foo"))
		assert.Equal(t, "blah", parsed.Query().Get("blah"))
		assert.Equal(t, "30m0s", parsed.Query().Get("oplogMaxDelta"))
	})

	t.Run("mongo with count params", func(t *testing.T) {
		p, err := New("testdata/config.yml")
		require.NoError(t, err)
		p.Services.Mongo[0].URL = "mongodb://example.com:27017/admin"
		p.Services.Mongo[0].OplogMaxDelta = 0
		p.Services.Mongo[0].Collection = "coll"
		p.Services.Mongo[0].DB = "test"
		p.Services.Mongo[0].CountQuery = `{"status":"active"}`

		var rawURL string
		for _, service := range p.MarshalServices() {
			if strings.HasPrefix(service, "dev:mongodb://") {
				rawURL = strings.TrimPrefix(service, "dev:")
				break
			}
		}
		require.NotEmpty(t, rawURL)
		parsed, err := url.Parse(rawURL)
		require.NoError(t, err)
		assert.Equal(t, "/admin", parsed.Path)
		assert.Equal(t, "coll", parsed.Query().Get("collection"))
		assert.Equal(t, "test", parsed.Query().Get("db"))
		assert.JSONEq(t, `{"status":"active"}`, parsed.Query().Get("count"))
		assert.Empty(t, parsed.Query().Get("countQuery"))
	})

	t.Run("invalid mongo URL remains invalid", func(t *testing.T) {
		p, err := New("testdata/config.yml")
		require.NoError(t, err)
		p.Services.Mongo[0].URL = "mongodb://example.com/%zz"
		p.Services.Mongo[0].Collection = "coll"
		assert.True(t, slices.Contains(p.MarshalServices(), "dev:mongodb://example.com/%zz"))
	})

	t.Run("program args are query values", func(t *testing.T) {
		p, err := New("testdata/config.yml")
		require.NoError(t, err)
		p.Services.Program[0].Args = []string{"arg1", "arg two", "left&right"}

		var rawURL string
		for _, service := range p.MarshalServices() {
			if strings.HasPrefix(service, "first:program://") {
				rawURL = strings.TrimPrefix(service, "first:")
				break
			}
		}
		require.NotEmpty(t, rawURL)
		parsed, err := url.Parse(rawURL)
		require.NoError(t, err)
		assert.Equal(t, []string{"arg1", "arg two", "left&right"}, parsed.Query()["args"])
	})

	t.Run("program without args has no query", func(t *testing.T) {
		p, err := New("testdata/config.yml")
		require.NoError(t, err)
		p.Services.Program[0].Args = nil
		assert.True(t, slices.Contains(p.MarshalServices(), "first:program:///usr/bin/example1"))
	})

	t.Run("provider targets preserve path characters", func(t *testing.T) {
		p := &Parameters{}
		p.Services.File = []File{
			{Name: "file-percent", Path: "/srv/50%_done.tar"},
			{Name: "file-question", Path: "/data/report?.csv"},
			{Name: "file-hash", Path: "/data/a#b.log"},
			{Name: "file-pattern", Path: "/data/*_[[.YYYY]]_?.tar"},
			{Name: "file-userinfo", Path: "backup@daily/report.txt"},
		}
		p.Services.Program = []Program{
			{Name: "program-percent", Path: "/srv/50%_done"},
			{Name: "program-userinfo", Path: "backup@daily/report.sh"},
		}
		want := map[string]string{
			"file-percent":     "/srv/50%_done.tar",
			"file-question":    "/data/report?.csv",
			"file-hash":        "/data/a#b.log",
			"file-pattern":     "/data/*_[[.YYYY]]_?.tar",
			"file-userinfo":    "backup@daily/report.txt",
			"program-percent":  "/srv/50%_done",
			"program-userinfo": "backup@daily/report.sh",
		}

		for _, service := range p.MarshalServices() {
			name, rawURL, ok := strings.Cut(service, ":")
			require.True(t, ok)
			parsed, err := url.Parse(rawURL)
			require.NoError(t, err)
			assert.Nil(t, parsed.User)
			assert.Equal(t, filepath.Clean(want[name]), filepath.Clean(parsed.Host+parsed.Path))
		}
	})
}
