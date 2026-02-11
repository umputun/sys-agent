package actuator

import (
	"github.com/umputun/sys-agent/app/status"
)

// status constants matching Spring Boot Actuator format
const (
	StatusUp   = "UP"
	StatusDown = "DOWN"
)

// HealthResponse represents Spring Boot Actuator compatible health response
type HealthResponse struct {
	Status     string               `json:"status"`
	Components map[string]Component `json:"components,omitempty"`
}

// Component represents a single health component with status and details
type Component struct {
	Status  string         `json:"status"`
	Details map[string]any `json:"details,omitempty"`
}

// thresholds for determining component health
const threshold = 90

// FromStatusInfo converts status.Info to actuator HealthResponse
func FromStatusInfo(info *status.Info) *HealthResponse {
	if info == nil {
		return nil
	}

	resp := &HealthResponse{
		Status:     StatusUp,
		Components: make(map[string]Component),
	}

	// cpu component
	cpuStatus := StatusUp
	if info.CPUPercent >= threshold {
		cpuStatus = StatusDown
	}
	resp.Components["cpu"] = Component{
		Status:  cpuStatus,
		Details: map[string]any{"percent": info.CPUPercent},
	}

	// memory component
	memStatus := StatusUp
	if info.MemPercent >= threshold {
		memStatus = StatusDown
	}
	resp.Components["memory"] = Component{
		Status:  memStatus,
		Details: map[string]any{"percent": info.MemPercent},
	}

	// disk components
	for name, vol := range info.Volumes {
		diskStatus := StatusUp
		if vol.UsagePercent >= threshold {
			diskStatus = StatusDown
		}
		resp.Components["diskSpace:"+name] = Component{
			Status: diskStatus,
			Details: map[string]any{
				"path":    vol.Path,
				"percent": vol.UsagePercent,
			},
		}
	}

	// external service components - prefixed with "service:" to avoid collision with reserved keys
	for name, svc := range info.ExtServices {
		svcStatus := StatusUp
		if svc.StatusCode < 200 || svc.StatusCode >= 300 {
			svcStatus = StatusDown
		}
		details := map[string]any{
			"status_code":   svc.StatusCode,
			"response_time": svc.ResponseTime,
		}
		if svc.Body != nil {
			details["body"] = svc.Body
		}
		resp.Components["service:"+name] = Component{
			Status:  svcStatus,
			Details: details,
		}
	}

	// load average component (informational, always UP)
	resp.Components["loadAverage"] = Component{
		Status: StatusUp,
		Details: map[string]any{
			"one":     info.Loads.One,
			"five":    info.Loads.Five,
			"fifteen": info.Loads.Fifteen,
		},
	}

	// determine overall status - DOWN if any component is DOWN
	for _, comp := range resp.Components {
		if comp.Status == StatusDown {
			resp.Status = StatusDown
			break
		}
	}

	return resp
}

// DiscoveryResponse represents the actuator discovery endpoint response with links to available endpoints
type DiscoveryResponse struct {
	Links map[string]Link `json:"_links"`
}

// Link represents a single HAL-style link
type Link struct {
	Href string `json:"href"`
}

// Discovery returns the actuator discovery response listing available endpoints
func Discovery() *DiscoveryResponse {
	return &DiscoveryResponse{
		Links: map[string]Link{
			"self":   {Href: "/actuator"},
			"health": {Href: "/actuator/health"},
		},
	}
}
