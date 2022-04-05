package external

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// NginxProvider is a status provider that uses nginx status response
type NginxProvider struct {
	TimeOut     time.Duration
	lastHandled int
}

// Status url looks like: nginx://example.com/nginx_status. It will try https first and if it fails http
// nginx response looks like this:
//  Active connections: 124
//  server accepts handled requests
//   783855 783855 1676992
//  Reading: 0 Writing: 300 Waiting: 27
func (n *NginxProvider) Status(req Request) (*Response, error) {

	st := time.Now()
	result := &Response{Name: req.Name}
	client := http.Client{Timeout: n.TimeOut}

	u := strings.Replace(req.URL, "nginx://", "https://", 1)

	resp, err := client.Get(u)
	if err != nil {
		u = strings.Replace(req.URL, "nginx://", "http://", 1)
		resp, err = client.Get(u)
		if err != nil {
			return nil, errors.Wrapf(err, "both https and http failed for %s", req.URL)
		}
	}
	defer resp.Body.Close() // nolint
	result.StatusCode = resp.StatusCode
	result.ResponseTime = time.Since(st).Milliseconds()

	if resp.StatusCode != 200 {
		return result, nil
	}

	ngStats, err := n.parseResponse(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse nginx response for %s", req.URL)
	}
	result.Body = ngStats
	return result, nil
}

func (n *NginxProvider) parseResponse(r io.Reader) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response")
	}
	lines := strings.Split(string(body), "\n")
	if len(lines) < 4 {
		return nil, errors.New("response is too short")
	}

	if !strings.HasPrefix(strings.TrimSpace(lines[0]), "Active connections:") {
		return nil, errors.New("response does not start with \"active connections\"")
	}
	active, err := strconv.Atoi(strings.TrimSpace(strings.Split(strings.TrimSpace(lines[0]), ":")[1]))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse active connections %s", lines[0])
	}
	result["active_connections"] = active

	if !strings.HasPrefix(strings.TrimSpace(lines[1]), "server accepts handled requests") {
		return nil, errors.New("response does not include \"server accepts handled requests\"")
	}

	elems := strings.Fields(strings.TrimSpace(lines[2]))
	if len(elems) != 3 {
		return nil, fmt.Errorf("failed to parse server accepts handled requests %s", lines[2])
	}

	accepts, err := strconv.Atoi(elems[0])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse accepts %s", lines[2])
	}
	result["accepts"] = accepts

	handled, err := strconv.Atoi(elems[1])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse handled %s", lines[2])
	}
	result["handled"] = handled

	requests, err := strconv.Atoi(elems[2])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse requests %s", lines[2])
	}
	result["requests"] = requests

	result["change_handled"] = handled - n.lastHandled
	n.lastHandled = handled

	l := strings.Replace(strings.TrimSpace(lines[3]), "Reading: ", "", 1)
	l = strings.Replace(l, "Writing: ", "", 1)
	l = strings.Replace(l, "Waiting: ", "", 1)
	elems = strings.Fields(l)
	if len(elems) != 3 {
		return nil, fmt.Errorf("failed to parse \"reading writing waiting\" %s", lines[3])
	}

	reading, err := strconv.Atoi(elems[0])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse reading %s", lines[2])
	}
	result["reading"] = reading

	writing, err := strconv.Atoi(elems[1])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse writing %s", lines[2])
	}
	result["writing"] = writing

	waiting, err := strconv.Atoi(elems[2])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse waiting %s", lines[2])
	}
	result["waiting"] = waiting

	return result, nil
}
