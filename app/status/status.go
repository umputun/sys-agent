package status

import (
	"fmt"
	"log"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"

	"github.com/umputun/sys-agent/app/status/external"
)

//go:generate moq -out ext_mock.go -skip-ensure -fmt goimports . ExtServices

// Service provides disk and cpu utilization
type Service struct {
	Volumes     []Volume
	ExtServices ExtServices
}

// ExtServices declares interface to get status of all external services
type ExtServices interface {
	Status() []external.Response
}

// Info contains disk and cpu utilization results
type Info struct {
	HostName   string            `json:"hostname"`
	Procs      int               `json:"procs"`
	HostID     string            `json:"host_id"`
	CPUPercent int               `json:"cpu_percent"`
	MemPercent int               `json:"mem_percent"`
	Uptime     uint64            `json:"uptime"`
	Volumes    map[string]Volume `json:"volumes,omitempty"`
	Loads      struct {
		One     float64 `json:"one"`
		Five    float64 `json:"five"`
		Fifteen float64 `json:"fifteen"`
	} `json:"load_average"`
	ExtServices map[string]external.Response `json:"services,omitempty"`
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

	memp, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory percent: %w", err)
	}

	hostStat, err := host.Info()
	if err != nil {
		return nil, fmt.Errorf("failed to get host info: %w", err)
	}

	loads, err := load.Avg()
	if err != nil {
		return nil, fmt.Errorf("failed to get load average: %w", err)
	}

	res := Info{
		HostName:   hostStat.Hostname,
		Procs:      int(hostStat.Procs),
		HostID:     hostStat.HostID,
		CPUPercent: int(cpup[0]),
		MemPercent: int(memp.UsedPercent),
		Volumes:    map[string]Volume{},
		Uptime:     hostStat.Uptime,
	}
	res.Loads.One, res.Loads.Five, res.Loads.Fifteen = loads.Load1, loads.Load5, loads.Load15

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

	if s.ExtServices != nil {
		res.ExtServices = map[string]external.Response{}
		for _, v := range s.ExtServices.Status() {
			res.ExtServices[v.Name] = v
		}
	}

	log.Printf("[DEBUG] status: %+v", res)
	return &res, nil
}
