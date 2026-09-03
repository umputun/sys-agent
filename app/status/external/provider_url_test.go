package external

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
