package external

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// DockerProvider is a status provider that uses docker
type DockerProvider struct {
	TimeOut time.Duration
}

// Status the url looks like: docker:///var/run/docker.sock or docker://1.2.3.4:2375
// optionally the url can contain a query param "containers" with a comma separated list of container names to be presented
func (d *DockerProvider) Status(req Request) (*Response, error) {

	uu, err := url.Parse(req.URL)
	if err != nil {
		return nil, fmt.Errorf("docker url parse failed: %s %s: %w", req.Name, req.URL, err)
	}

	var schemaRegex = regexp.MustCompile("^(?:([a-z0-9]+)://)?(.*)$")
	parts := schemaRegex.FindStringSubmatch(uu.Path)
	proto, addr := parts[1], parts[2]
	if proto == "" {
		proto = "unix"
	}
	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial(proto, addr)
			},
		},
		Timeout: d.TimeOut,
	}

	resp, err := client.Get("http://localhost/v1.22/containers/json")
	if err != nil {
		return nil, fmt.Errorf("docker request failed: %s %s: %w", req.Name, req.URL, err)
	}
	defer resp.Body.Close()

	dkinfo, err := d.parseDockerResponse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("docker parsing failed: %s %s: %w", req.Name, req.URL, err)
	}

	result := Response{
		Name:       req.Name,
		StatusCode: resp.StatusCode,
		Body:       dkinfo,
	}
	return &result, nil
}

func (d *DockerProvider) parseDockerResponse(r io.Reader) (map[string]interface{}, error) {
	var dkResp []struct {
		ID      string `json:"Id"`
		State   string
		Status  string
		Created int64
		Names   []string
	}

	if err := json.NewDecoder(r).Decode(&dkResp); err != nil {
		return nil, fmt.Errorf("docker ummarshal failed: %w", err)
	}

	type container struct {
		Name   string `json:"name"`
		State  string `json:"state"`
		Status string `json:"status"`
	}

	containers := map[string]container{}
	running, healthy := 0, 0
	for _, r := range dkResp {
		if len(r.Names) == 0 || r.Names[0] == "/" {
			continue
		}
		name := strings.TrimPrefix(r.Names[0], "/")
		containers[name] = container{
			Name:   name,
			State:  r.State,
			Status: r.Status,
		}

		if r.State == "running" {
			running++
		}
		if strings.HasSuffix(r.Status, "(healthy)") {
			healthy++
		}
	}

	res := map[string]interface{}{
		"containers": containers,
		"total":      len(containers),
		"healthy":    healthy,
		"running":    running,
		"failed":     len(containers) - running,
	}
	return res, nil
}
