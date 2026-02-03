package config

import (
	"slices"
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
			"first:program:///usr/bin/example1?args=\"arg1 arg2\"", "second:program:///usr/bin/example2",
			"rmqtest:rmq://guest:passwd@example.com:15672/v1/q1",
		}
		assert.Equal(t, exp, p.MarshalServices())
	})

	t.Run("mongo with query params", func(t *testing.T) {
		p, err := New("testdata/config.yml")
		require.NoError(t, err)
		p.Services.Mongo[0].URL = "mongodb://example.com:27017/admin?foo=bar&blah=blah"
		exp := "dev:mongodb://example.com:27017/admin?foo=bar&blah=blah&oplogMaxDelta=30m0s"
		res := p.MarshalServices()
		assert.True(t, slices.Contains(res, exp), "expected %s in %v", exp, res)
	})

	t.Run("mongo with count params", func(t *testing.T) {
		p, err := New("testdata/config.yml")
		require.NoError(t, err)
		p.Services.Mongo[0].URL = "mongodb://example.com:27017/admin"
		p.Services.Mongo[0].Collection = "coll"
		p.Services.Mongo[0].DB = "test"
		p.Services.Mongo[0].CountQuery = `{"status":"active"}`
		exp := `dev:mongodb://example.com:27017/admin?oplogMaxDelta=30m0s&collection=coll&db=test&countQuery={"status":"active"}`
		res := p.MarshalServices()
		assert.True(t, slices.Contains(res, exp), "expected %s in %v", exp, res)
	})
}
