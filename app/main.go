package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	log "github.com/go-pkgz/lgr"
	"github.com/umputun/go-flags"

	"github.com/umputun/sys-agent/app/server"
	"github.com/umputun/sys-agent/app/status"
)

var revision string

var opts struct {
	Listen  string   `short:"l" long:"listen" env:"LISTEN" default:"localhost:8080" description:"listen on host:port"`
	Volumes []string `short:"v" long:"volume" env:"VOLUMES" default:"root:/" env-delim:"," description:"volumes to report"`
	Dbg     bool     `long:"dbg" env:"DEBUG" description:"show debug info"`
}

func main() {
	fmt.Printf("sys-agent %s\n", revision)

	p := flags.NewParser(&opts, flags.PassDoubleDash|flags.HelpFlag)
	if _, err := p.Parse(); err != nil {
		if err.(*flags.Error).Type != flags.ErrHelp {
			fmt.Printf("%v\n", err)
			os.Exit(1)
		}
		p.WriteHelp(os.Stderr)
		os.Exit(2)
	}
	setupLog(opts.Dbg)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if x := recover(); x != nil {
			log.Printf("[WARN] run time panic:\n%v", x)
			panic(x)
		}

		// catch signal and invoke graceful termination
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		<-stop
		log.Printf("[WARN] interrupt signal")
		cancel()
	}()

	vols, err := parseVolumes(opts.Volumes)
	if err != nil {
		log.Fatalf("[ERROR] %s", err)
	}

	srv := server.Rest{
		Listen:  opts.Listen,
		Version: revision,
		Status:  &status.Service{Volumes: vols},
	}

	if err := srv.Run(ctx); err != nil && err.Error() != "http: Server closed" {
		log.Fatalf("[ERROR] %s", err)
	}
}

// parseVolumes parses volumes from string list, each element in format "name:path"
func parseVolumes(volumes []string) ([]status.Volume, error) {
	res := make([]status.Volume, len(volumes))
	for i, v := range volumes {
		parts := strings.SplitN(v, ":", 2)
		if len(parts) != 2 {
			return nil, errors.New("invalid volume format, should be <name>:<path>")
		}
		res[i] = status.Volume{Name: parts[0], Path: parts[1]}
	}
	log.Printf("[DEBUG] volumes: %+v", res)
	return res, nil
}

func setupLog(dbg bool) {
	if dbg {
		log.Setup(log.Debug, log.CallerFile, log.CallerFunc, log.Msec, log.LevelBraces)
		return
	}
	log.Setup(log.Msec, log.LevelBraces)
}
