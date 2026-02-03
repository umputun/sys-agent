package external

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const dockerClientVersion = "1.24"

// DockerProvider is a status provider that uses docker
type DockerProvider struct {
	TimeOut time.Duration
}

// Status the url looks like: docker:///var/run/docker.sock or docker://1.2.3.4:2375
// optionally the url can contain a query param "required" with a comma separated list of required container names
// i.e. docker:///var/run/docker.sock?containers=foo,bar
func (d *DockerProvider) Status(req Request) (*Response, error) {

	st := time.Now()
	u := strings.Replace(req.URL, "docker://", "tcp://", 1)
	if strings.HasPrefix(req.URL, "docker:///") { // i.e. docker:///var/run/docker.sock
		u = strings.Replace(req.URL, "docker://", "unix://", 1)
	}
	uu, err := url.Parse(u)
	if err != nil {
		return nil, fmt.Errorf("docker url parse failed: %s %s: %w", req.Name, req.URL, err)
	}

	if uu.Scheme == "unix" { // for unix socket use path as host
		uu.Host = uu.Path
	}

	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial(uu.Scheme, uu.Host)
			},
		},
		Timeout: d.TimeOut,
	}

	dkURL := fmt.Sprintf("http://localhost/v%s/containers/json", dockerClientVersion)
	resp, err := client.Get(dkURL)
	if err != nil {
		return nil, fmt.Errorf("docker request failed: %s %s: %w", req.Name, req.URL, err)
	}
	defer func() {
		if e := resp.Body.Close(); e != nil {
			log.Printf("[WARN] docker response close failed: %s %s: %s", req.Name, req.URL, e)
		}
	}()

	var required []string
	if uu.Query().Get("containers") != "" {
		for r := range strings.SplitSeq(uu.Query().Get("containers"), ":") {
			r = strings.TrimSpace(r)
			if r == "" {
				continue
			}
			required = append(required, r)
		}
	}
	dkinfo, err := d.parseDockerResponse(resp.Body, required)
	if err != nil {
		return nil, fmt.Errorf("docker parsing failed: %s %s: %w", req.Name, req.URL, err)
	}

	result := Response{
		Name:         req.Name,
		StatusCode:   resp.StatusCode,
		Body:         dkinfo,
		ResponseTime: time.Since(st).Milliseconds(),
	}
	return &result, nil
}

func (d *DockerProvider) parseDockerResponse(r io.Reader, required []string) (map[string]any, error) {
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
	running, healthy, unhealthy := 0, 0, 0
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
		if strings.HasSuffix(r.Status, "(unhealthy)") {
			unhealthy++
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

	res := map[string]any{
		"containers": containers,
		"total":      len(containers),
		"healthy":    healthy,
		"unhealthy":  unhealthy,
		"running":    running,
		"failed":     len(containers) - running,
		"required":   "ok",
	}

	if len(requiredNotFound) > 0 {
		res["required"] = "failed: " + strings.Join(requiredNotFound, ",")
	}
	log.Printf("[DEBUG] required containers %+v, failed: %+v", required, requiredNotFound)
	return res, nil
}
