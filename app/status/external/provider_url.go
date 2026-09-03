package external

import (
	"fmt"
	"net/url"
)

// parseTarget joins host and path to preserve relative and absolute provider targets.
func parseTarget(rawURL, wantScheme string) (string, url.Values, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", nil, fmt.Errorf("parse provider URL: %w", err)
	}
	if parsed.Scheme != wantScheme {
		return "", nil, fmt.Errorf("unexpected scheme %q, want %q", parsed.Scheme, wantScheme)
	}
	if parsed.Fragment != "" {
		return "", nil, fmt.Errorf("URL fragment is not allowed, encode # as %%23")
	}

	target := parsed.Host + parsed.Path
	if target == "" {
		return "", nil, fmt.Errorf("empty target for %s URL", wantScheme)
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return "", nil, fmt.Errorf("parse provider URL query: %w", err)
	}
	return target, query, nil
}
