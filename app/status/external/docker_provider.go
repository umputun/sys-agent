package external

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DockerProvider is a status provider that uses docker
type DockerProvider struct {
	TimeOut time.Duration
}

// Status the url looks like: docker:///var/run/docker.sock or docker://1.2.3.4:2375
// optionally the url can contain a query param "required" with a comma separated list of required container names
// i.e. docker:///var/run/docker.sock?containers=foo,bar
func (d *DockerProvider) Status(req Request) (*Response, error) {

	u := strings.Replace(req.URL, "docker://", "tcp://", 1)
	if strings.HasPrefix(req.URL, " docker:///") { // i.e. docker:///var/run/docker.sock
		u = strings.Replace(req.URL, "docker://", "unix://", 1)
	}
	uu, err := url.Parse(u)
	if err != nil {
		return nil, fmt.Errorf("docker url parse failed: %s %s: %w", req.Name, req.URL, err)
	}

	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial(uu.Scheme, uu.Host)
			},
		},
		Timeout: d.TimeOut,
	}

	resp, err := client.Get("http://localhost/v1.22/containers/json")
	if err != nil {
		return nil, fmt.Errorf("docker request failed: %s %s: %w", req.Name, req.URL, err)
	}
	defer resp.Body.Close()

	var required []string
	if uu.Query().Get("containers") != "" {
		required = strings.Split(uu.Query().Get("containers"), ",")
	}
	dkinfo, err := d.parseDockerResponse(resp.Body, required)
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

func (d *DockerProvider) parseDockerResponse(r io.Reader, required []string) (map[string]interface{}, error) {
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

	var requiredNotFound []string
	for _, rq := range required {
		if _, ok := containers[rq]; !ok {
			requiredNotFound = append(requiredNotFound, rq)
			continue
		}
		if containers[rq].State != "running" {
			requiredNotFound = append(requiredNotFound, rq)
		}
	}

	res := map[string]interface{}{
		"containers": containers,
		"total":      len(containers),
		"healthy":    healthy,
		"running":    running,
		"failed":     len(containers) - running,
		"required":   "ok",
	}

	if len(requiredNotFound) > 0 {
		res["required"] = "failed: " + strings.Join(requiredNotFound, ",")
	}

	return res, nil
}
