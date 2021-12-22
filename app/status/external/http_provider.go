package external

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPProvider is an external service that checks the status of a HTTP endpoint
type HTTPProvider struct {
	http.Client
}

// Status returns the status of the external service via HTTP GET
func (h *HTTPProvider) Status(req Request) (*Response, error) {
	st := time.Now()
	resp, err := h.Get(req.URL)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %s %s: %w", req.Name, req.URL, err)
	}
	defer resp.Body.Close()

	bodyStr, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("http read failed: %s %s: %w", req.Name, req.URL, err)
	}

	var bodyJSON map[string]interface{}
	if err := json.Unmarshal(bodyStr, &bodyJSON); err != nil {
		bodyJSON = map[string]interface{}{"text": string(bodyStr)}
	}
	result := Response{
		Name:         req.Name,
		StatusCode:   resp.StatusCode,
		Body:         bodyJSON,
		ResponseTime: time.Since(st).Milliseconds(),
	}
	return &result, nil
}
