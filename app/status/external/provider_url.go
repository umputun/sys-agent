package external

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// parseTarget joins host and path to preserve relative and absolute provider targets.
func parseTarget(rawURL, wantScheme string) (string, url.Values, error) {
	if strings.Contains(rawURL, "#") {
		return "", nil, fmt.Errorf("URL fragment is not allowed, encode # as %%23")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", nil, fmt.Errorf("parse provider URL: %w", err)
	}
	if parsed.Scheme != wantScheme {
		return "", nil, fmt.Errorf("unexpected scheme %q, want %q", parsed.Scheme, wantScheme)
	}
	if parsed.User != nil {
		return "", nil, fmt.Errorf("URL userinfo is not allowed, prefix a relative target containing @ with ./")
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

// urlToken matches a URL-like substring inside free-form error text.
var urlToken = regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.\-]*://\S+`)

// credentialBreaks are the delimiters url.Parse splits a malformed authority on. It quotes the piece
// it choked on, so a password containing one reaches the error in fragments.
const credentialBreaks = "/?#"

// firstAtSchemes name consumers that take everything before the FIRST @ as userinfo, whatever
// net/url makes of the authority. The mongo driver does this at connstring.go:918, before it splits
// host, path or query, so mongodb://user:123/secret@host/ carries a password net/url reports as a
// host and port.
var firstAtSchemes = map[string]bool{"mongodb": true, "mongodb+srv": true}

// sanitizeErr renders err for the unauthenticated /status body with the credentials configured in
// rawURL removed. Scrubbing the configured values comes first and URL masking second: an error is
// free-form text, and a provider may repeat a secret outside any URL.
func sanitizeErr(err error, rawURL string) string {
	if err == nil {
		return ""
	}
	out := err.Error()
	for _, secret := range configuredSecrets(rawURL) {
		out = strings.ReplaceAll(out, secret, "xxxxx")
	}
	return urlToken.ReplaceAllStringFunc(out, maskURL)
}

// configuredSecrets returns the credential material in rawURL - userinfo, password and every query
// value - each in raw, percent-decoded and Go-quoted form, since the mongo driver re-emits a bad
// option value with %q. Where the userinfo ends depends on who parses the target: net/url answers
// for its own consumers, the driver's own rule answers for the schemes in firstAtSchemes, and when
// net/url rejects the target nothing about the boundary is known, so everything ahead of the last @
// is credential material. All three answers are collected rather than chosen between.
func configuredSecrets(rawURL string) []string {
	secrets := []string{}
	add := func(v string) {
		if v == "" {
			return
		}
		forms := []string{v}
		if decoded, err := url.QueryUnescape(v); err == nil && decoded != v {
			forms = append(forms, decoded)
		}
		// a provider may rewrite the target before using it, and its error then repeats the
		// transformed value rather than the configured one
		for _, f := range slices.Clip(forms) {
			if rewritten := rmqAPIPath(f); rewritten != f {
				forms = append(forms, rewritten)
			}
		}
		for _, f := range forms {
			secrets = append(secrets, f)
			if quoted := strings.Trim(strconv.Quote(f), `"`); quoted != f {
				secrets = append(secrets, quoted)
			}
		}
	}

	addCredential := func(credential string) {
		add(credential)
		for _, segment := range strings.FieldsFunc(credential, func(r rune) bool {
			return strings.ContainsRune(credentialBreaks, r)
		}) {
			add(segment)
		}
	}

	scheme, rest, _ := strings.Cut(rawURL, "://")
	if firstAtSchemes[scheme] {
		if userinfo, _, found := strings.Cut(rest, "@"); found {
			addCredential(userinfo)
			if _, password, ok := strings.Cut(userinfo, ":"); ok {
				addCredential(password)
			}
		}
	}

	if parsed, err := url.Parse(rawURL); err == nil {
		if parsed.User != nil {
			addCredential(parsed.User.String())
			addCredential(parsed.User.Username())
			if password, ok := parsed.User.Password(); ok {
				addCredential(password)
			}
			if raw, _, found := strings.Cut(rest, "@"); found {
				addCredential(raw)
				if _, rawPassword, ok := strings.Cut(raw, ":"); ok {
					addCredential(rawPassword)
				}
			}
		}
	} else if userinfo, _, found := cutLast(rest, "@"); found {
		addCredential(userinfo)
		if _, password, ok := strings.Cut(userinfo, ":"); ok {
			addCredential(password)
		}
	}

	if _, rawQuery, found := strings.Cut(rawURL, "?"); found {
		for _, pair := range strings.FieldsFunc(rawQuery, func(r rune) bool { return r == '&' || r == ';' }) {
			if _, value, ok := strings.Cut(pair, "="); ok {
				add(value)
			}
		}
	}

	// longest first, so a value containing another is replaced whole rather than left in fragments
	sort.Slice(secrets, func(i, j int) bool { return len(secrets[i]) > len(secrets[j]) })
	return secrets
}

// cutLast is strings.Cut around the last occurrence of sep.
func cutLast(s, sep string) (before, after string, found bool) {
	i := strings.LastIndex(s, sep)
	if i < 0 {
		return s, "", false
	}
	return s[:i], s[i+len(sep):], true
}

// maskURL masks userinfo and query of a single URL token, keeping trailing punctuation the
// surrounding error text put there. An unparsable query is dropped whole, since url.Parse keeps
// RawQuery verbatim and Query silently discards pairs it cannot read.
func maskURL(token string) string {
	trailing := ""
	for token != "" && strings.ContainsRune(`:,;.)]}"\'`, rune(token[len(token)-1])) {
		trailing = token[len(token)-1:] + trailing
		token = token[:len(token)-1]
	}

	parsed, err := url.Parse(token)
	if err != nil {
		return maskUnparsed(token) + trailing
	}
	if parsed.User != nil {
		parsed.User = url.User("xxxxx")
	}
	if parsed.RawQuery != "" {
		query, qErr := url.ParseQuery(parsed.RawQuery)
		if qErr != nil {
			parsed.RawQuery = "xxxxx"
		} else {
			for k := range query {
				query[k] = []string{"xxxxx"}
			}
			parsed.RawQuery = query.Encode()
		}
	}
	return parsed.String() + trailing
}

// maskUnparsed masks a token url.Parse rejected. Everything ahead of the last @ goes, since the
// authority boundary is unknown here; a token with no @ keeps its path, where the diagnostic lives.
func maskUnparsed(token string) string {
	scheme, rest, hasScheme := strings.Cut(token, "://")
	if _, after, found := cutLast(rest, "@"); found {
		rest = "xxxxx@" + after
	}
	if hasScheme {
		return scheme + "://" + rest
	}
	return rest
}
