package status

import (
	"fmt"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
)

// Service provides disk and cpu utilization
type Service struct {
	Volumes []Volume
}

// Info contains disk and cpu utilization results
type Info struct {
	HostName   string            `json:"hostname"`
	Procs      int               `json:"procs"`
	HostID     string            `json:"host_id"`
	CPUPercent int               `json:"cpu_percent"`
	Volumes    map[string]Volume `json:"volumes,omitempty"`
}

// Volume contains input information for a volume and the result for utilization percentage
type Volume struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	UsagePercent int    `json:"usage_percent"`
}

// Get returns the disk and cpu utilization
func (s Service) Get() (*Info, error) {
	cpup, err := cpu.Percent(0, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get cpu percent: %w", err)
	}

	hostStat, err := host.Info()
	if err != nil {
		return nil, fmt.Errorf("failed to get host info: %w", err)
	}
	res := Info{
		HostName:   hostStat.Hostname,
		Procs:      int(hostStat.Procs),
		HostID:     hostStat.HostID,
		CPUPercent: int(cpup[0]),
		Volumes:    map[string]Volume{},
	}

	for _, v := range s.Volumes {
		usage, err := disk.Usage(v.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to get disk usage for %s: %w", v.Path, err)
		}
		res.Volumes[v.Name] = Volume{
			Name:         v.Name,
			Path:         v.Path,
			UsagePercent: int(usage.UsedPercent),
		}
	}
	return &res, nil
}
