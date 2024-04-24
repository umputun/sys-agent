// Package config provides the configuration for the application.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Parameters represents the whole configuration parameters
type Parameters struct {
	Volumes  []Volume `yaml:"volumes"`
	Services struct {
		HTTP        []HTTP        `yaml:"http"`
		Certificate []Certificate `yaml:"certificate"`
		File        []File        `yaml:"file"`
		Mongo       []Mongo       `yaml:"mongo"`
		Nginx       []Nginx       `yaml:"nginx"`
		Program     []Program     `yaml:"program"`
		Docker      []Docker      `yaml:"docker"`
		RMQ         []RMQ         `yaml:"rmq"`
	} `yaml:"services"`

	fileName string `yaml:"-"`
}

// Volume represents a volumes to check
type Volume struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

// HTTP represents a http service to check
type HTTP struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

// Certificate represents a certificate to check
type Certificate struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

// Docker represents a docker container to check
type Docker struct {
	Name       string   `yaml:"name"`
	URL        string   `yaml:"url"`
	Containers []string `yaml:"containers"` // required containers
}

// File represents a file to check
type File struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

// Mongo represents a mongo service to check
type Mongo struct {
	Name          string        `yaml:"name"`
	URL           string        `yaml:"url"`
	OplogMaxDelta time.Duration `yaml:"oplog_max_delta"`
	Collection    string        `yaml:"collection"`
	DB            string        `yaml:"db"`
	CountQuery    string        `yaml:"count_query"`
}

// Nginx represents a nginx service to check
type Nginx struct {
	Name      string `yaml:"name"`
	StatusURL string `yaml:"status_url"`
}

// Program represents a program to check
type Program struct {
	Name string   `yaml:"name"`
	Path string   `yaml:"path"`
	Args []string `yaml:"args"`
}

// RMQ represents a rmq to check
type RMQ struct {
	Name  string `yaml:"name"`
	URL   string `yaml:"url"`
	User  string `yaml:"user"`
	Pass  string `yaml:"pass"`
	Vhost string `yaml:"vhost"`
	Queue string `yaml:"queue"`
}

// New creates a new Parameters from the given file
func New(fname string) (*Parameters, error) {
	p := &Parameters{fileName: fname}
	data, err := os.ReadFile(fname) // nolint gosec
	if err != nil {
		return nil, fmt.Errorf("can't read config %s: %w", fname, err)
	}
	if err = yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("failed to parsse config %s: %w", fname, err)
	}
	return p, nil
}

// MarshalVolumes returns the volumes as a list of strings with the format "name:path"
func (p *Parameters) MarshalVolumes() []string {
	res := make([]string, 0, len(p.Volumes))
	for _, v := range p.Volumes {
		res = append(res, fmt.Sprintf("%s:%s", v.Name, v.Path))
	}
	return res
}

// MarshalServices returns the services as a list of strings with the format used by command line
func (p *Parameters) MarshalServices() []string {
	res := []string{}

	for _, v := range p.Services.HTTP {
		res = append(res, fmt.Sprintf("%s:%s", v.Name, v.URL))
	}

	for _, v := range p.Services.Certificate {
		url := strings.TrimPrefix(v.URL, "https://")
		url = strings.TrimPrefix(url, "http://")
		res = append(res, fmt.Sprintf("%s:cert://%s", v.Name, url))
	}

	for _, v := range p.Services.Docker {
		url := strings.TrimPrefix(v.URL, "https://")
		url = strings.TrimPrefix(url, "http://")
		url = strings.TrimPrefix(url, "tcp://")
		url = strings.TrimPrefix(url, "unix://")
		if len(v.Containers) > 0 {
			url += "?containers=" + strings.Join(v.Containers, ":")
		}
		res = append(res, fmt.Sprintf("%s:docker://%s", v.Name, url))
	}

	for _, v := range p.Services.File {
		res = append(res, fmt.Sprintf("%s:file://%s", v.Name, v.Path))
	}

	for _, v := range p.Services.Mongo {
		m := fmt.Sprintf("%s:%s", v.Name, v.URL)
		if v.OplogMaxDelta > 0 {
			if strings.Contains(m, "?") {
				m += fmt.Sprintf("&oplogMaxDelta=%v", v.OplogMaxDelta)
			} else {
				m += fmt.Sprintf("?oplogMaxDelta=%v", v.OplogMaxDelta)
			}
		}
		if v.Collection != "" {
			m += fmt.Sprintf("&collection=%s", v.Collection)
		}
		if v.DB != "" {
			m += fmt.Sprintf("&db=%s", v.DB)
		}
		if v.CountQuery != "" {
			m += fmt.Sprintf("&countQuery=%s", v.CountQuery)
		}
		res = append(res, m)
	}

	for _, v := range p.Services.Nginx {
		u := v.StatusURL
		u = strings.TrimPrefix(u, "http://")
		u = strings.TrimPrefix(u, "https://")

		res = append(res, fmt.Sprintf("%s:nginx://%s", v.Name, u))
	}

	for _, v := range p.Services.Program {
		prg := fmt.Sprintf("%s:program://%s", v.Name, v.Path)
		if len(v.Args) > 0 {
			prg += "?args=\"" + strings.Join(v.Args, " ") + "\""
		}
		res = append(res, prg)
	}

	for _, v := range p.Services.RMQ {
		u := v.URL
		u = strings.TrimPrefix(u, "http://")
		u = strings.TrimPrefix(u, "https://")
		if v.User != "" && v.Pass != "" {
			u = fmt.Sprintf("%s:%s@%s", v.User, v.Pass, u)

		}
		res = append(res, fmt.Sprintf("%s:rmq://%s/%s/%s", v.Name, u, v.Vhost, v.Queue))
	}

	return res
}

func (p *Parameters) String() string {
	return fmt.Sprintf("config file: %q, %+v", p.fileName, *p)
}
