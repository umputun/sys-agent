package status

import (
	"fmt"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
)

// Service provides disk and cpu utilization
type Service struct {
	Volumes []Volume
}

// Info contains disk and cpu utilization results
type Info struct {
	CPUPercent int `json:"cpu_percent"`
	Volumes    []struct {
		Name         string `json:"name"`
		Path         string `json:"path"`
		UsagePercent int    `json:"usage_percent"`
	} `json:"volumes,omitempty"`
}

// Volume contains input information for a volume
type Volume struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// Get returns the disk and cpu utilization
func (s Service) Get() (*Info, error) {
	cpup, err := cpu.Percent(0, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get cpu percent: %w", err)
	}
	res := Info{CPUPercent: int(cpup[0])}

	for _, v := range s.Volumes {
		usage, err := disk.Usage(v.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to get disk usage for %s: %w", v.Path, err)
		}
		res.Volumes = append(res.Volumes, struct {
			Name         string `json:"name"`
			Path         string `json:"path"`
			UsagePercent int    `json:"usage_percent"`
		}{
			Name:         v.Name,
			Path:         v.Path,
			UsagePercent: int(usage.UsedPercent),
		})
	}
	return &res, nil
}
