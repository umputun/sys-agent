package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	{
		_, err := New("testdata/invalid.yml")
		require.Error(t, err)
		assert.EqualErrorf(t, err, "can't read config testdata/invalid.yml: open testdata/invalid.yml: no such file or directory", "expected error")
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
		assert.Equal(t, []Http{{Name: "first", URL: "https://example1.com"}, {Name: "second", URL: "https://example2.com"}},
			p.Services.Http)
		assert.Equal(t, []Mongo{{Name: "dev", URL: "mongodb://example.com:27017", OplogMaxDelta: time.Duration(30 * time.Minute)}},
			p.Services.Mongo)
		assert.Equal(t, []Nginx{{Name: "nginx", StatusURL: "http://example.com:80"}}, p.Services.Nginx)
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
	exp := `config file: "testdata/config.yml", {Volumes:[{Name:root Path:/hostroot} {Name:data Path:/data}] Services:{Http:[{Name:first URL:https://example1.com} {Name:second URL:https://example2.com}] Certificate:[{Name:prim_cert URL:https://example1.com} {Name:second_cert URL:https://example2.com}] File:[{Name:first Path:/tmp/example1.txt} {Name:second Path:/tmp/example2.txt}] Mongo:[{Name:dev URL:mongodb://example.com:27017 OplogMaxDelta:30m0s}] Nginx:[{Name:nginx StatusURL:http://example.com:80}] Program:[{Name:first Path:/usr/bin/example1 Args:[arg1 arg2]} {Name:second Path:/usr/bin/example2 Args:[]}] Docker:[{Name:docker1 URL:unix:///var/run/docker.sock Containers:[reproxy mattermost postgres]} {Name:docker2 URL:tcp://192.168.1.1:4080 Containers:[]}]} fileName:testdata/config.yml}`
	assert.Equal(t, exp, p.String())
}

func TestParameters_MarshalServices(t *testing.T) {
	{
		p, err := New("testdata/config.yml")
		require.NoError(t, err)
		exp := []string{
			"first:https://example1.com", "second:https://example2.com",
			"prim_cert:cert://example1.com", "second_cert:cert://example2.com",
			"docker1:docker:///var/run/docker.sock?containers=reproxy,mattermost,postgres", "docker2:docker://192.168.1.1:4080",
			"first:file:///tmp/example1.txt", "second:file:///tmp/example2.txt",
			"dev:mongodb://example.com:27017?oplogMaxDelta=30m0s",
			"nginx:nginx:http://example.com:80",
			"first:program:///usr/bin/example1?args=\"arg1 arg2\"", "second:program:///usr/bin/example2",
		}
		assert.Equal(t, exp, p.MarshalServices())
	}

	{ // test mongo with query params
		p, err := New("testdata/config.yml")
		require.NoError(t, err)
		p.Services.Mongo[0].URL = "mongodb://example.com:27017/admin?foo=bar&blah=blah"
		exp := "dev:mongodb://example.com:27017/admin?foo=bar&blah=blah&oplogMaxDelta=30m0s"
		res := p.MarshalServices()
		found := false
		for _, r := range res {
			if r == exp {
				found = true
				break
			}
		}
		assert.True(t, found, "expected %s in %v", exp, res)
	}
}
