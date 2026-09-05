package external

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/umputun/sys-agent/app/config"
)

func TestParseTarget(t *testing.T) {
	targets := []struct {
		name       string
		rawURL     string
		wantScheme string
		wantTarget string
	}{
		{name: "program name", rawURL: "program://ps", wantScheme: "program", wantTarget: "ps"},
		{name: "program path", rawURL: "program:///abs/path.sh", wantScheme: "program", wantTarget: "/abs/path.sh"},
		{name: "relative file", rawURL: "file://rel/f.txt", wantScheme: "file", wantTarget: "rel/f.txt"},
		{name: "absolute file", rawURL: "file:///abs/f.txt", wantScheme: "file", wantTarget: "/abs/f.txt"},
		{name: "certificate host", rawURL: "cert://example.com", wantScheme: "cert", wantTarget: "example.com"},
	}

	for _, tt := range targets {
		t.Run(tt.name, func(t *testing.T) {
			target, query, err := parseTarget(tt.rawURL, tt.wantScheme)
			require.NoError(t, err)
			assert.Equal(t, tt.wantTarget, target)
			assert.Empty(t, query)
		})

		t.Run(tt.name+" with cron", func(t *testing.T) {
			target, query, err := parseTarget(tt.rawURL+"?cron=0_6_*_*_*", tt.wantScheme)
			require.NoError(t, err)
			assert.Equal(t, tt.wantTarget, target)
			assert.Equal(t, url.Values{"cron": {"0_6_*_*_*"}}, query)
		})
	}
}

func TestParseTargetEncoding(t *testing.T) {
	tests := []struct {
		name       string
		rawURL     string
		wantTarget string
		wantQuery  url.Values
	}{
		{name: "question mark", rawURL: "file:///tmp/log_%3F%3F.txt", wantTarget: "/tmp/log_??.txt"},
		{name: "hash", rawURL: "file:///tmp/a%23b", wantTarget: "/tmp/a#b"},
		{name: "space", rawURL: "file:///tmp/x%20y.txt", wantTarget: "/tmp/x y.txt"},
		{name: "raw query", rawURL: "file:///tmp/a?b", wantTarget: "/tmp/a", wantQuery: url.Values{"b": {""}}},
		{name: "relative at sign", rawURL: "file://./backup@daily/report.txt", wantTarget: "./backup@daily/report.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, query, err := parseTarget(tt.rawURL, "file")
			require.NoError(t, err)
			assert.Equal(t, tt.wantTarget, target)
			if tt.wantQuery == nil {
				assert.Empty(t, query)
				return
			}
			assert.Equal(t, tt.wantQuery, query)
		})
	}
}

func TestParseTargetErrors(t *testing.T) {
	tests := []struct {
		name       string
		rawURL     string
		wantScheme string
		wantError  string
	}{
		{name: "bare percent", rawURL: "file:///backups/50%_done.tar", wantScheme: "file", wantError: "invalid URL escape"},
		{name: "scheme mismatch", rawURL: "file:///tmp/a", wantScheme: "program", wantError: "unexpected scheme"},
		{name: "empty target", rawURL: "file://", wantScheme: "file", wantError: "empty target"},
		{name: "raw fragment", rawURL: "file:///tmp/a#b", wantScheme: "file", wantError: "fragment"},
		{name: "trailing raw fragment", rawURL: "file:///tmp/report#", wantScheme: "file", wantError: "fragment"},
		{name: "userinfo", rawURL: "file://backup@daily/report.txt", wantScheme: "file", wantError: "userinfo"},
		{name: "malformed query", rawURL: "file:///tmp/a?cron=%zz", wantScheme: "file", wantError: "invalid URL escape"},
		{name: "invalid port", rawURL: "program://ps:abc", wantScheme: "program", wantError: "invalid port"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseTarget(tt.rawURL, tt.wantScheme)
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.wantError)
		})
	}
}

func TestParseTargetConfigRelativePaths(t *testing.T) {
	params := &config.Parameters{}
	params.Services.File = []config.File{{Name: "backup", Path: "*_gitlab_backup.tar"}}
	params.Services.Program = []config.Program{{Name: "processes", Path: "ps"}}

	services := params.MarshalServices()
	require.Len(t, services, 2)
	fileTarget, _, err := parseTarget(strings.TrimPrefix(services[0], "backup:"), "file")
	require.NoError(t, err)
	assert.Equal(t, "./*_gitlab_backup.tar", fileTarget)
	programTarget, _, err := parseTarget(strings.TrimPrefix(services[1], "processes:"), "program")
	require.NoError(t, err)
	assert.Equal(t, "ps", programTarget)
}

func TestParseTargetConfigEmptyPaths(t *testing.T) {
	t.Run("file", func(t *testing.T) {
		params := &config.Parameters{}
		params.Services.File = []config.File{{Name: "backup"}}
		services := params.MarshalServices()
		require.Len(t, services, 1)
		_, _, err := parseTarget(strings.TrimPrefix(services[0], "backup:"), "file")
		require.ErrorContains(t, err, "empty target")
	})

	t.Run("program", func(t *testing.T) {
		params := &config.Parameters{}
		params.Services.Program = []config.Program{{Name: "backup"}}
		services := params.MarshalServices()
		require.Len(t, services, 1)
		_, _, err := parseTarget(strings.TrimPrefix(services[0], "backup:"), "program")
		require.ErrorContains(t, err, "empty target")
	})
}

func TestSanitizeErr(t *testing.T) {
	const pass = "pa55word"
	tbl := []struct {
		name   string
		rawURL string
		err    error
		want   string
	}{
		{name: "nil", err: nil, want: ""},
		{name: "no url", rawURL: "mongodb://h/", err: errors.New("mongo replset is empty"), want: "mongo replset is empty"},
		{
			name:   "mongo userinfo and query",
			rawURL: "mongodb://user:" + pass + "@10.0.0.5:27017/?authSource=admin",
			err:    errors.New("mongo connect failed: db mongodb://user:" + pass + "@10.0.0.5:27017/?authSource=admin: timeout"),
			want:   "mongo connect failed: db mongodb://xxxxx@10.0.0.5:27017/?authSource=xxxxx: timeout",
		},
		{
			name:   "http token query",
			rawURL: "http://example.com/health?token=secret",
			err:    errors.New("http request failed: api http://example.com/health?token=secret: connection refused"),
			want:   "http request failed: api http://example.com/health?token=xxxxx: connection refused",
		},
		{
			name:   "unparsable target keeps its path",
			rawURL: "file:///backups/50%_done.tar",
			err:    errors.New(`file URL parse failed: backup file:///backups/50%_done.tar: invalid URL escape "%_d"`),
			want:   `file URL parse failed: backup file:///backups/50%_done.tar: invalid URL escape "%_d"`,
		},
		{
			name:   "path at sign is not userinfo",
			rawURL: "file://./backup@daily/report.txt",
			err:    errors.New("file check failed: backup file://./backup@daily/report.txt: no such file"),
			want:   "file check failed: backup file://./backup@daily/report.txt: no such file",
		},
		{
			name:   "plain target without credentials",
			rawURL: "docker:///var/run/docker.sock",
			err:    errors.New("docker request failed: dk docker:///var/run/docker.sock: no such file"),
			want:   "docker request failed: dk docker:///var/run/docker.sock: no such file",
		},
		{
			name:   "malformed query escape",
			rawURL: "http://host/health?token=SUPERSECRET%zz",
			err:    errors.New("http request failed: probe http://host/health?token=SUPERSECRET%zz: dial failed"),
			want:   "http request failed: probe http://host/health?token=xxxxx: dial failed",
		},
		{
			name:   "semicolon separated query",
			rawURL: "http://host/health?token=SUPERSECRET;extra=value",
			err:    errors.New("http request failed: probe http://host/health?token=SUPERSECRET;extra=value: dial failed"),
			want:   "http request failed: probe http://host/health?xxxxx: dial failed",
		},
		{
			name:   "whitespace inside userinfo",
			rawURL: "http://user:prefix SUPERSECRET@host/health",
			err:    errors.New("http request failed: probe http://user:prefix SUPERSECRET@host/health: dial failed"),
			want:   "http request failed: probe http://xxxxx@host/health: dial failed",
		},
		{
			name:   "secret repeated outside the url",
			rawURL: "mongodb://host/?tlsCertificateKeyFilePassword=SUPERSECRET%zz",
			err: errors.New(`mongo connect failed: probe mongodb://host/?tlsCertificateKeyFilePassword=SUPERSECRET%zz: ` +
				`error parsing uri: invalid option value "SUPERSECRET%zz": invalid URL escape "%zz"`),
			want: `mongo connect failed: probe mongodb://host/?tlsCertificateKeyFilePassword=xxxxx: ` +
				`error parsing uri: invalid option value "xxxxx": invalid URL escape "%zz"`,
		},
	}

	for _, tt := range tbl {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sanitizeErr(tt.err, tt.rawURL))
		})
	}
}

func TestSanitizeErr_NeverLeaksSecret(t *testing.T) {
	const secret = "sup3rsecret"
	tbl := []struct {
		name   string
		target string
		secret string
	}{
		{name: "mongo password", target: "mongodb://user:" + secret + "@host:27017/?authSource=admin", secret: secret},
		{name: "http password and token", target: "http://admin:" + secret + "@host/health?token=" + secret, secret: secret},
		{name: "rmq password", target: "rmq://user:" + secret + "@host:15672/vh/q", secret: secret},
		{name: "encoded password", target: "mongodb://user:sup3r%25" + secret + "@host/", secret: "sup3r%25" + secret},
		{name: "malformed token escape", target: "http://host/health?token=" + secret + "%zz", secret: secret + "%zz"},
		{name: "semicolon separated", target: "http://host/health?token=" + secret + ";extra=value", secret: secret},
		{name: "whitespace in userinfo", target: "http://user:prefix " + secret + "@host/health", secret: "prefix " + secret},
		{name: "driver option", target: "mongodb://host/?tlsCertificateKeyFilePassword=" + secret + "%zz", secret: secret + "%zz"},
		{name: "quote in value", target: `mongodb://host/?tlsCertificateKeyFilePassword=` + secret + `"tail`, secret: secret + `"tail`},
		{name: "backslash in value", target: `mongodb://host/?tlsCertificateKeyFilePassword=` + secret + `\tail`, secret: secret + `\tail`},
		{name: "two character value", target: "mongodb://host/?tlsCertificateKeyFilePassword=%q", secret: "%q"},
		{name: "slash in password", target: "http://user:" + secret + "/part@host/health", secret: secret},
		{name: "question mark in password", target: "http://user:" + secret + "?part@host/health", secret: secret},
		{name: "hash in password", target: "http://user:" + secret + "#part@host/health", secret: secret},
		{name: "file userinfo", target: "file://user:" + secret + "%zz@host/path", secret: secret + "%zz"},
		{name: "mongo slash password behind numeric port", target: "mongodb://user:123/" + secret + "@host/", secret: secret},
		{name: "mongo query password behind numeric port", target: "mongodb://user:123?" + secret + "@host:bad/", secret: secret},
	}

	for _, tt := range tbl {
		t.Run(tt.name, func(t *testing.T) {
			// the mongo driver quotes the offending value again outside the URL, so both copies must go
			err := fmt.Errorf("check failed: probe %s: error parsing uri: invalid option value %q", tt.target, tt.secret)
			assert.NotContains(t, sanitizeErr(err, tt.target), tt.secret)
		})
	}
}
